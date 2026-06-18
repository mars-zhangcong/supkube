package drflow

import (
	"context"
	"fmt"
	"net"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var kbClusterGVR = schema.GroupVersionResource{
	Group:    "apps.kubeblocks.io",
	Version:  "v1alpha1",
	Resource: "clusters",
}

// gateRestoringDB checks:
//  1. KB Cluster CR phase == "Running"
//  2. Headless service DNS resolves (svc.<ns>.svc.cluster.local)
//  3. KB-generated secret exists
func gateRestoringDB(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, r Run) error {
	// 1. KB Cluster CR phase
	cluster, err := dynCli.Resource(kbClusterGVR).Namespace(r.DrillNS).Get(ctx, r.KBCluster, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("KB Cluster %s/%s not found: %w", r.DrillNS, r.KBCluster, err)
	}
	phase, _, _ := unstructured.NestedString(cluster.Object, "status", "phase")
	if phase != "Running" {
		return fmt.Errorf("KB Cluster phase=%s (want Running)", phase)
	}

	// 2. Service DNS
	svcName := fmt.Sprintf("%s-postgresql.%s.svc.cluster.local", r.KBCluster, r.DrillNS)
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := (&net.Resolver{}).LookupHost(dialCtx, svcName); err != nil {
		return fmt.Errorf("service DNS %s not ready: %w", svcName, err)
	}

	// 3. Secret exists
	if _, err := k8sCli.CoreV1().Secrets(r.DrillNS).Get(ctx, r.DBSecret, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("DB secret %s/%s not ready: %w", r.DrillNS, r.DBSecret, err)
	}

	return nil
}

// gateRestoringApp checks that the app pod is Running + all containers Ready.
// It uses both the caller-supplied TargetApp selector and the run-specific
// drflow-run-id label so that multiple concurrent drill runs don't interfere.
func gateRestoringApp(ctx context.Context, k8sCli kubernetes.Interface, r Run) error {
	// Always scope to this run's pods via the run ID label
	selector := labelRunID + "=" + r.ID
	if r.TargetApp != "" {
		selector = r.TargetApp + "," + selector
	}
	pods, err := k8sCli.CoreV1().Pods(r.DrillNS).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Errorf("list pods(%s): %w", r.TargetApp, err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != "Running" {
			continue
		}
		allReady := true
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			return nil
		}
	}
	return fmt.Errorf("no Running+Ready pod matching %s in %s", r.TargetApp, r.DrillNS)
}

// gateRealigning verifies that the new DB credentials from the drill-namespace
// Secret actually authenticate against the restored DB. Per ADR-052 D4 /
// ADR-053 D3, Realigning means "App can connect with re-aligned credentials"
// not just "port is open" — a TCP-only probe would allow a mismatched Secret
// to silently pass here and only blow up in Validating.
//
// Implementation: PostgreSQL wire protocol SELECT 1. We dial the svc and send
// a minimal StartupMessage + password exchange without needing a psql binary.
// Failure here pins the problem to the credential/realignment layer (connection
// refused, auth failed) vs. Validating which checks data correctness.
func gateRealigning(ctx context.Context, k8sCli kubernetes.Interface, r Run) error {
	secret, err := k8sCli.CoreV1().Secrets(r.DrillNS).Get(ctx, r.DBSecret, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("read DB secret for realign probe: %w", err)
	}
	user := string(secret.Data["username"])
	pass := string(secret.Data["password"])
	dbName := "demo"
	if v := secret.Data["database"]; len(v) > 0 {
		dbName = string(v)
	}
	svcAddr := fmt.Sprintf("%s-postgresql.%s.svc.cluster.local:5432", r.KBCluster, r.DrillNS)
	return pgSelect1(ctx, svcAddr, user, pass, dbName)
}

// pgSelect1 opens a TCP connection, performs PostgreSQL MD5/cleartext auth
// and sends SELECT 1. Returns nil on success, an error describing the
// exact failure (TCP, auth, query) otherwise. Supports cleartext and MD5
// password authentication (the two methods common in KB-managed clusters).
func pgSelect1(ctx context.Context, addr, user, pass, dbName string) error {
	dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("TCP dial to %s: %w", addr, err)
	}
	defer conn.Close()                                //nolint:errcheck
	conn.SetDeadline(time.Now().Add(8 * time.Second)) //nolint:errcheck

	// PostgreSQL startup message (protocol 3.0)
	startup := pgStartupMessage(user, dbName)
	if _, err := conn.Write(startup); err != nil {
		return fmt.Errorf("write startup: %w", err)
	}

	// Read and handle server response (auth + ReadyForQuery)
	if err := pgHandshake(conn, user, pass); err != nil {
		return fmt.Errorf("auth probe to %s: %w", addr, err)
	}
	return nil
}

// pgStartupMessage builds a minimal PG3 startup packet.
func pgStartupMessage(user, dbName string) []byte {
	params := []byte{}
	for _, kv := range [][2]string{{"user", user}, {"database", dbName}} {
		params = append(params, kv[0]...)
		params = append(params, 0)
		params = append(params, kv[1]...)
		params = append(params, 0)
	}
	params = append(params, 0) // terminator

	body := make([]byte, 4+len(params))
	// protocol version 3.0 = 0x00030000
	body[0], body[1], body[2], body[3] = 0x00, 0x03, 0x00, 0x00
	copy(body[4:], params)

	msg := make([]byte, 4+len(body))
	total := uint32(len(msg))
	msg[0], msg[1], msg[2], msg[3] = byte(total>>24), byte(total>>16), byte(total>>8), byte(total)
	copy(msg[4:], body)
	return msg
}

// pgHandshake reads PG3 messages until ReadyForQuery or an error.
// Handles AuthenticationOk (0), AuthenticationCleartextPassword (3),
// AuthenticationMD5Password (5), and ErrorResponse ('E').
func pgHandshake(conn net.Conn, user, pass string) error {
	buf := make([]byte, 4096)
	for {
		if _, err := readFull(conn, buf[:1]); err != nil {
			return fmt.Errorf("read msg type: %w", err)
		}
		msgType := buf[0]
		if _, err := readFull(conn, buf[:4]); err != nil {
			return fmt.Errorf("read msg len: %w", err)
		}
		msgLen := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
		body := make([]byte, msgLen-4)
		if len(body) > 0 {
			if _, err := readFull(conn, body); err != nil {
				return fmt.Errorf("read msg body: %w", err)
			}
		}
		switch msgType {
		case 'R': // Authentication
			if len(body) < 4 {
				return fmt.Errorf("short auth message")
			}
			authType := int(body[0])<<24 | int(body[1])<<16 | int(body[2])<<8 | int(body[3])
			switch authType {
			case 0: // AuthenticationOk — no password needed
				continue
			case 3: // CleartextPassword
				pwMsg := pgPasswordMessage(pass)
				if _, err := conn.Write(pwMsg); err != nil {
					return fmt.Errorf("send cleartext password: %w", err)
				}
			case 5: // MD5Password — salt in body[4:8]
				if len(body) < 8 {
					return fmt.Errorf("short MD5 auth message")
				}
				salt := body[4:8]
				md5pw := pgMD5Password(user, pass, salt)
				pwMsg := pgPasswordMessage(md5pw)
				if _, err := conn.Write(pwMsg); err != nil {
					return fmt.Errorf("send MD5 password: %w", err)
				}
			default:
				return fmt.Errorf("unsupported auth type %d (need scram? use KB system account)", authType)
			}
		case 'Z': // ReadyForQuery — handshake complete
			return nil
		case 'E': // ErrorResponse
			return fmt.Errorf("PG error: %s", pgErrorMessage(body))
		case 'S': // ParameterStatus — ignore
			continue
		case 'K': // BackendKeyData — ignore
			continue
		default:
			return fmt.Errorf("unexpected message type %c", msgType)
		}
	}
}

// gateValidating verifies data integrity and negative deviation isolation
// (ADR-053 D4 Q4 / PRD-017 整改②):
//
//  1. Injects a canary row into dr_seed (idempotent — skips if already present).
//  2. Verifies post-injection row count = 13 (12 baseline + 1 canary).
//  3. Verifies checksum of original rows (category != 'canary') matches M1 baseline.
//
// The canary injection is the "≥1 negative deviation" that proves the drill copy is
// truly writable and isolated from production (ADR-053 D5). All writes are confined
// to the drill-ns DB; the production/source DB is never touched.
//
// Checksum SQL mirrors the M1 verification query in m1-seed-data.sql:
//
//	md5(string_agg(id::text || category || value || num::text ORDER BY id))
const validateChecksum = "1efaf3e404186952d8d303932eb2f67d"

func gateValidating(ctx context.Context, k8sCli kubernetes.Interface, r Run) error {
	secret, err := k8sCli.CoreV1().Secrets(r.DrillNS).Get(ctx, r.DBSecret, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("read DB secret: %w", err)
	}
	user := string(secret.Data["username"])
	pass := string(secret.Data["password"])
	dbName := "demo"
	if v := secret.Data["database"]; len(v) > 0 {
		dbName = string(v)
	}
	addr := fmt.Sprintf("%s-postgresql.%s.svc.cluster.local:5432", r.KBCluster, r.DrillNS)

	// 1. Inject canary row (idempotent: skip if this run's canary already present).
	// The canary value is keyed on run ID so multiple runs don't interfere.
	canaryVal := "drill-" + r.ID[:8]
	injectSQL := fmt.Sprintf(
		`INSERT INTO dr_seed (category, value, num) SELECT 'canary', '%s', -1 WHERE NOT EXISTS (SELECT 1 FROM dr_seed WHERE value = '%s')`,
		canaryVal, canaryVal,
	)
	if err := pgExec(ctx, addr, user, pass, dbName, injectSQL); err != nil {
		return fmt.Errorf("canary injection: %w", err)
	}

	// 2. Row count: 12 baseline + 1 canary = 13
	countStr, err := pgSimpleQuery(ctx, addr, user, pass, dbName, "SELECT COUNT(*)::text FROM dr_seed")
	if err != nil {
		return fmt.Errorf("row count query: %w", err)
	}
	if countStr != "13" {
		return fmt.Errorf("post-injection row count=%s (want 13 = 12 baseline + canary)", countStr)
	}

	// 3. Checksum of original rows only (exclude canary) must match M1 baseline.
	// Using the exact SQL from m1-seed-data.sql verification query.
	checksumSQL := `SELECT md5(string_agg(id::text || category || value || num::text ORDER BY id)) FROM dr_seed WHERE category != 'canary'`
	chk, err := pgSimpleQuery(ctx, addr, user, pass, dbName, checksumSQL)
	if err != nil {
		return fmt.Errorf("checksum query: %w", err)
	}
	if chk != validateChecksum {
		return fmt.Errorf("checksum=%s (want %s) — baseline data divergence detected", chk, validateChecksum)
	}

	return nil
}
