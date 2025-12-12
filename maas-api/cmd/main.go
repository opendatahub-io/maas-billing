package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/api_keys"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/config"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/constant"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/handlers"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
)

func main() {
	cfg := config.Load()
	flag.Parse()

	gin.SetMode(gin.ReleaseMode) // Explicitly set release mode
	if cfg.DebugMode {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.Default()
	if cfg.DebugMode {
		router.Use(cors.New(cors.Config{
			AllowMethods:  []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:  []string{"Authorization", "Content-Type", "Accept"},
			ExposeHeaders: []string{"Content-Type"},
			AllowOriginFunc: func(origin string) bool {
				return true
			},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	router.OPTIONS("/*path", func(c *gin.Context) { c.Status(204) })

	ctx, cancel := context.WithCancel(context.Background())

	store, err := initStore(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize token store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("Failed to close token store: %v", err)
		}
	}()

	registerHandlers(ctx, router, cfg, store)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received, shutting down server...")

	cancel()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown:", err) //nolint:gocritic // exits immediately
	}

	log.Println("Server exited gracefully")
}

// initStore creates the store based on the configured storage mode.
//
// Storage modes:
//   - in-memory (default): Ephemeral storage, data lost on restart
//   - disk: Persistent local storage using a file (single replica only)
//   - external: External database (PostgreSQL), supports multiple replicas
//
//nolint:ireturn // Returns MetadataStore interface by design for pluggable storage backends.
func initStore(ctx context.Context, cfg *config.Config) (api_keys.MetadataStore, error) {
	switch cfg.StorageMode {
	case config.StorageModeInMemory, "":
		log.Printf("Using in-memory storage (data will be lost on restart). " +
			"For persistent storage, use --storage=disk or --storage=external")
		return api_keys.NewSQLiteStore(ctx, ":memory:")

	case config.StorageModeDisk:
		dataPath := strings.TrimSpace(cfg.DataPath)
		if dataPath == "" {
			dataPath = config.DefaultDataPath
		}
		log.Printf("Using persistent disk storage at %s", dataPath)
		return api_keys.NewSQLiteStore(ctx, dataPath)

	case config.StorageModeExternal:
		dbURL := strings.TrimSpace(cfg.DBConnectionURL)
		if dbURL == "" {
			return nil, errors.New("--db-connection-url is required when using --storage=external")
		}
		log.Printf("Connecting to external database...")
		return api_keys.NewExternalStore(ctx, dbURL)

	default:
		return nil, fmt.Errorf("unknown storage mode: %q (valid modes: in-memory, disk, external)", cfg.StorageMode)
	}
}

func registerHandlers(ctx context.Context, router *gin.Engine, cfg *config.Config, store api_keys.MetadataStore) {
	router.GET("/health", handlers.NewHealthHandler().HealthCheck)

	clusterConfig, err := config.NewClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	gatewayRef := models.GatewayRef{
		Name:      cfg.GatewayName,
		Namespace: cfg.GatewayNamespace,
	}
	modelMgr := models.NewManager(clusterConfig.KServeV1Beta1, clusterConfig.KServeV1Alpha1, clusterConfig.Gateway, gatewayRef)
	modelsHandler := handlers.NewModelsHandler(modelMgr)

	// V1 API routes
	v1Routes := router.Group("/v1")

	tierMapper := tier.NewMapper(ctx, clusterConfig.ClientSet, cfg.Name, cfg.Namespace)
	tierHandler := tier.NewHandler(tierMapper)
	v1Routes.POST("/tiers/lookup", tierHandler.TierLookup)

	informerFactory := informers.NewSharedInformerFactory(clusterConfig.ClientSet, constant.DefaultResyncPeriod)

	namespaceInformer := informerFactory.Core().V1().Namespaces()
	serviceAccountInformer := informerFactory.Core().V1().ServiceAccounts()
	informersSynced := []cache.InformerSynced{
		namespaceInformer.Informer().HasSynced,
		serviceAccountInformer.Informer().HasSynced,
	}

	informerFactory.Start(ctx.Done())

	if !cache.WaitForNamedCacheSync("maas-api", ctx.Done(), informersSynced...) {
		log.Fatalf("Failed to sync informer caches")
	}

	// Create token manager (ephemeral, K8s logic)
	tokenManager := token.NewManager(
		cfg.Name,
		tierMapper,
		clusterConfig.ClientSet,
		namespaceInformer.Lister(),
		serviceAccountInformer.Lister(),
	)
	tokenHandler := token.NewHandler(cfg.Name, tokenManager)

	apiKeyService := api_keys.NewService(tokenManager, store)
	apiKeyHandler := api_keys.NewHandler(apiKeyService)

	// Create reviewer with audience to properly validate Service Account tokens
	reviewer := token.NewReviewer(clusterConfig.ClientSet, cfg.Name+"-sa")

	// Model listing endpoint (v1Routes is grouped under /v1, so this creates /v1/models)
	//nolint:contextcheck // Context is properly accessed via gin.Context in the returned handler
	v1Routes.GET("/models", tokenHandler.ExtractUserInfo(reviewer), modelsHandler.ListLLMs)

	//nolint:contextcheck // Context is properly accessed via gin.Context in the returned handler
	tokenRoutes := v1Routes.Group("/tokens", tokenHandler.ExtractUserInfo(reviewer))
	tokenRoutes.POST("", tokenHandler.IssueToken)
	tokenRoutes.DELETE("", apiKeyHandler.RevokeAllTokens)

	//nolint:contextcheck // Context is properly accessed via gin.Context in the returned handler
	apiKeyRoutes := v1Routes.Group("/api-keys", tokenHandler.ExtractUserInfo(reviewer))
	apiKeyRoutes.POST("", apiKeyHandler.CreateAPIKey)
	apiKeyRoutes.GET("", apiKeyHandler.ListAPIKeys)
	apiKeyRoutes.GET("/:id", apiKeyHandler.GetAPIKey)
	// Note: Single key deletion removed for initial release - use DELETE /v1/tokens to revoke all tokens
}
