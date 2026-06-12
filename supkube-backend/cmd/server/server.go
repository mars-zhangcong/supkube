package server

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	v1 "github.com/supkube/supkube-backend/internal/api/v1"
	"github.com/supkube/supkube-backend/internal/auth"
	"github.com/supkube/supkube-backend/internal/clusterhealth"
	"github.com/supkube/supkube-backend/internal/csi"
	"github.com/supkube/supkube-backend/internal/drflow"
	"github.com/supkube/supkube-backend/internal/fingerprint"
	"github.com/supkube/supkube-backend/internal/gc"
	"github.com/supkube/supkube-backend/internal/importpolicy"
	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/policypair"
	"github.com/supkube/supkube-backend/internal/velerons"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func Run() error {
	v1.SeedBuiltinTransformSets()

	go func() {
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[gc] runtime client unavailable; orphan GC disabled: %v", err)
			return
		}
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[gc] kubernetes client unavailable; orphan GC disabled: %v", err)
			return
		}
		log.Printf("[gc] orphan GC runner started")
		gc.Run(context.Background(), runtimeCli, k8sCli)
	}()

	go func() {
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[policypair] runtime client unavailable; controller disabled: %v", err)
			return
		}
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[policypair] kubernetes client unavailable; controller disabled: %v", err)
			return
		}
		policypair.Run(context.Background(), runtimeCli, k8sCli)
	}()

	go func() {
		dynCli, err := k8s.GetDynamicClient()
		if err != nil {
			log.Printf("[clusterhealth] dynamic client unavailable; controller disabled: %v", err)
			return
		}
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[clusterhealth] kubernetes client unavailable; controller disabled: %v", err)
			return
		}
		clusterhealth.Run(context.Background(), dynCli, k8sCli)
	}()

	go func() {
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[csi-autoconfig] kubernetes client unavailable; auto-config disabled: %v", err)
			return
		}
		dynCli, err := k8s.GetDynamicClient()
		if err != nil {
			log.Printf("[csi-autoconfig] dynamic client unavailable; auto-config disabled: %v", err)
			return
		}
		csi.Run(context.Background(), k8sCli, dynCli, 10*time.Minute)
	}()

	go func() {
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[fingerprint] kubernetes client unavailable; fingerprint pipeline disabled: %v", err)
			return
		}
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[fingerprint] runtime client unavailable; fingerprint pipeline disabled: %v", err)
			return
		}
		clusterID, clusterName := fingerprint.ResolveClusterIdentity(context.Background(), k8sCli)
		if clusterID == "" {
			log.Printf("[fingerprint] could not resolve source cluster ID (kube-system UID); writer disabled, validator still available")
		} else {
			log.Printf("[fingerprint] source cluster: %s (%s)", clusterName, clusterID)
		}
		secretLoader := fingerprint.NewK8sSecretLoader(k8sCli)
		bslCli := fingerprint.NewBSLClient(runtimeCli)
		writer := fingerprint.NewWriter(bslCli, secretLoader, clusterID, clusterName)
		validator := fingerprint.NewValidator(bslCli, secretLoader)
		trustStore := fingerprint.NewTrustStore(k8sCli)
		v1.SetFingerprintDeps(validator, trustStore)
		if clusterID != "" {
			fingerprint.NewRunner(writer, runtimeCli).Run(context.Background())
		} else {
			<-context.Background().Done()
		}
	}()

	go func() {
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[importpolicy] runtime client unavailable; controller disabled: %v", err)
			return
		}
		dynCli, err := k8s.GetDynamicClient()
		if err != nil {
			log.Printf("[importpolicy] dynamic client unavailable; controller disabled: %v", err)
			return
		}
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[importpolicy] kubernetes client unavailable; controller disabled: %v", err)
			return
		}
		bslCli := fingerprint.NewBSLClient(runtimeCli)
		dedupeSource := importpolicy.NewVeleroBackupLister(runtimeCli)
		lister := importpolicy.NewS3BackupLister(bslCli, dedupeSource)
		importer := importpolicy.NewS3BackupImporter(bslCli, runtimeCli)
		validator := &fingerprintAdapter{inner: fingerprint.NewValidator(
			fingerprint.NewBSLClient(runtimeCli),
			fingerprint.NewK8sSecretLoader(k8sCli),
		)}
		bslCheck := func(ctx context.Context, name string) (bool, error) {
			bsl := &velerov1.BackupStorageLocation{}
			err := runtimeCli.Get(ctx, types.NamespacedName{Namespace: velerons.Namespace(), Name: name}, bsl)
			switch {
			case err == nil:
				return true, nil
			case apierrors.IsNotFound(err):
				return false, nil
			default:
				return false, err
			}
		}
		ctrl := &importpolicy.Controller{
			DynCli:    dynCli,
			Lister:    lister,
			Importer:  importer,
			Validator: validator,
		}
		importpolicy.RegisterController(ctrl, dynCli, bslCheck)
		log.Printf("[importpolicy] controller started")
		ctrl.Run(context.Background())
	}()

	var drflowK8s kubernetes.Interface
	var drflowDyn dynamic.Interface
	if k8sC, err := k8s.GetClient(); err != nil {
		log.Printf("[drflow] kubernetes client unavailable; DRFlow routes disabled: %v", err)
	} else if dynC, err := k8s.GetDynamicClient(); err != nil {
		log.Printf("[drflow] dynamic client unavailable; DRFlow routes disabled: %v", err)
	} else {
		drflowK8s = k8sC
		drflowDyn = dynC
		go drflow.RecoverInFlightRuns(context.Background(), drflowK8s, drflowDyn)
	}

	authCfg := auth.LoadConfigFromEnv()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api/v1")
	api.Use(authCfg.AuditMiddleware())
	api.Use(authCfg.Middleware())
	api.Use(authCfg.RBACMiddleware())
	{
		api.GET("/auth/providers", authCfg.ListProviders)
		api.POST("/auth/callback", authCfg.Callback)
		api.GET("/auth/me", authCfg.Me)
		api.POST("/auth/logout", authCfg.Logout)
		api.GET("/auth/rbac/bindings", authCfg.ListRBACBindings)
		api.GET("/audit-logs", auth.ListAuditLogs)

		api.GET("/status", v1.GetStatus)
		api.GET("/namespaces", v1.ListNamespaces)
		api.POST("/namespaces", v1.CreateNamespace)
		api.GET("/dashboard/summary", v1.GetDashboardSummary)
		api.GET("/applications", v1.ListApplications)
		api.GET("/applications/:namespace/details", v1.GetApplicationDetails)
		api.GET("/applications/:namespace/storage-capability", v1.GetNamespaceStorageCapability)
		api.GET("/storage/csi/status", v1.GetCSIStatus)
		api.GET("/restore-points", v1.ListRestorePoints)
	}

	return r.Run()
}
