// Package v1: plugins.go — Helm plugin status surface for Settings UI (v0.8.13).
//
// The "Plugins" tab in Settings shows which optional SupKube components
// are currently installed (Velero, Dex, MinIO Local Store) plus a
// helm-upgrade command the admin can paste to enable each.
//
// We deliberately DO NOT execute helm from inside the backend pod:
//   - The backend container ships without the helm binary
//   - Running helm requires a kubeconfig with cluster-admin (the backend
//     pod's ServiceAccount is intentionally scoped, not cluster-admin)
//   - "Click button → mutate cluster" without a CI/CD record is a bad
//     audit story for enterprise customers
//
// So the contract is: the UI shows status + the command; the admin runs
// the command from their workstation. Future v0.9 can add an opt-in
// "Apply via in-cluster helm controller" mode for customers who want it.
//
// RBAC: viewer-or-above can READ the status (Settings is a low-trust page;
// hiding plugin status from editors creates support friction).
package v1

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/velerons"
)

// PluginStatus is one row in the Settings → Plugins table.
type PluginStatus struct {
	// ID is the value clients use to look up i18n keys and decide
	// which install instructions to show.
	ID string `json:"id"`

	// DisplayName is a hardcoded fallback; the SPA prefers i18n
	// (`plugins.<id>.name`) but uses this when a locale is missing.
	DisplayName string `json:"displayName"`

	// Description: same fallback pattern.
	Description string `json:"description"`

	// Required: when true the SPA hides the Disable button. Velero is
	// required (without it nothing works); Dex / MinIO are optional.
	Required bool `json:"required"`

	// Installed: did we find the canonical Deployment for this plugin?
	// Velero → deployment/velero in `velero` ns
	// Dex    → deployment/<release>-dex in supkube ns
	// MinIO  → deployment/supkube-local-store in supkube ns
	Installed bool `json:"installed"`

	// HelmValue: the dotted-path values.yaml key that toggles this
	// plugin. The SPA renders an "Enable" command using it.
	HelmValue string `json:"helmValue"`

	// EnableCmd / DisableCmd: complete `helm upgrade` invocations the
	// admin can copy-paste. Includes the release name and namespace
	// derived from runtime context (see resolveRelease below).
	EnableCmd  string `json:"enableCmd,omitempty"`
	DisableCmd string `json:"disableCmd,omitempty"`
}

// GetPluginsStatus is GET /api/v1/plugins/status.
func GetPluginsStatus(c *gin.Context) {
	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := context.Background()

	// Release name + namespace. v0.8.13 hardcodes the idiomatic
	// defaults; v0.9 will read these from the backend Deployment's
	// `app.kubernetes.io/instance` Helm-injected label so the
	// displayed commands are correct for non-default release names.
	releaseName := "supkube"
	releaseNS := "supkube"

	plugins := []PluginStatus{}

	// ── Velero ───────────────────────────────────────────────────────
	veleroInstalled := false
	if d, err := k8sCli.AppsV1().Deployments(velerons.Namespace()).Get(ctx, "velero", metav1.GetOptions{}); err == nil && d.Status.ReadyReplicas >= 1 {
		veleroInstalled = true
	}
	plugins = append(plugins, PluginStatus{
		ID:          "velero",
		DisplayName: "Velero",
		Description: "Core backup engine. Required.",
		Required:    true,
		Installed:   veleroInstalled,
		HelmValue:   "velero.enabled",
		EnableCmd:   helmCmd(releaseName, releaseNS, "velero.enabled", "true"),
	})

	// ── Dex (embedded OIDC) ─────────────────────────────────────────
	dexInstalled := false
	if _, err := k8sCli.AppsV1().Deployments(releaseNS).Get(ctx, releaseName+"-dex", metav1.GetOptions{}); err == nil {
		dexInstalled = true
	}
	plugins = append(plugins, PluginStatus{
		ID:          "dex",
		DisplayName: "Dex (Embedded OIDC)",
		Description: "Embedded identity provider. Disable when using an external OIDC issuer (Keycloak, Okta, Azure AD).",
		Installed:   dexInstalled,
		HelmValue:   "auth.dex.enabled",
		EnableCmd:   helmCmd(releaseName, releaseNS, "auth.dex.enabled", "true"),
		DisableCmd:  helmCmd(releaseName, releaseNS, "auth.dex.enabled", "false"),
	})

	// ── Local Backup Store (MinIO) ──────────────────────────────────
	localInstalled := false
	if d, err := k8sCli.AppsV1().Deployments(releaseNS).Get(ctx, "supkube-local-store", metav1.GetOptions{}); err == nil && d.Status.ReadyReplicas >= 1 {
		localInstalled = true
	}
	plugins = append(plugins, PluginStatus{
		ID:          "localStore",
		DisplayName: "Local Backup Store (MinIO)",
		Description: "In-cluster MinIO acting as Local BSL — fast recovery tier of the 3-2-1-1-0 model. Disable to save 100Gi PVC + ~256Mi memory.",
		Installed:   localInstalled,
		HelmValue:   "localStore.enabled",
		EnableCmd:   helmCmd(releaseName, releaseNS, "localStore.enabled", "true"),
		DisableCmd:  helmCmd(releaseName, releaseNS, "localStore.enabled", "false"),
	})

	c.JSON(http.StatusOK, gin.H{"items": plugins})
}

// helmCmd builds the install/upgrade command the admin pastes to flip
// the toggle. We use --reuse-values so the admin's existing customisations
// (auth bindings, branding, etc.) aren't wiped.
func helmCmd(release, namespace, key, value string) string {
	return "helm upgrade " + release + " supkube/supkube" +
		" --namespace " + namespace +
		" --reuse-values" +
		" --set " + key + "=" + value
}

// Compile-time anchor for the apierrors import — keeps it linked for the
// future "warn if a plugin Deployment is in CrashLoopBackOff" feature.
var _ = apierrors.IsNotFound
