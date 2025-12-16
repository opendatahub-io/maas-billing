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

	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/api_keys"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/config"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/constant"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/handlers"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/logger"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/models"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/tier"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/token"
)

func main() {
	cfg := config.Load()
	flag.Parse()

	// Initialize structured logger aligned with KServe conventions
	appLogger := logger.New(cfg.DebugMode)
	defer func() {
		_ = appLogger.Sync() // Ignore sync errors on close, as per zap documentation
	}()

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
		appLogger.Fatal("Failed to initialize token store",
			"error", err,
		)
	}
	defer func() {
		if err := store.Close(); err != nil {
			appLogger.Error("Failed to close token store",
				"error", err,
			)
		}
	}()

	registerHandlers(ctx, router, cfg, store, appLogger)

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
		appLogger.Info("Server starting",
			"port", cfg.Port,
			"debug_mode", cfg.DebugMode,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Fatal("Server failed to start",
				"error", err,
			)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info("Shutdown signal received, shutting down server...")

	cancel()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		appLogger.Fatal("Server forced to shutdown",
			"error", err,
		)
	}

	appLogger.Info("Server exited gracefully")
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

func registerHandlers(ctx context.Context, router *gin.Engine, cfg *config.Config, store api_keys.MetadataStore, appLogger *logger.Logger) {
	router.GET("/health", handlers.NewHealthHandler().HealthCheck)

	cluster, err := config.NewClusterConfig(cfg.Namespace, constant.DefaultResyncPeriod)
	if err != nil {
		appLogger.Fatal("Failed to create cluster config",
			"error", err,
		)
	}

	if !cluster.StartAndWaitForSync(ctx.Done()) {
		appLogger.Fatal("Failed to sync informer caches")
	}

	v1Routes := router.Group("/v1")

	tierMapper := tier.NewMapper(appLogger, cluster.ConfigMapLister, cfg.Name, cfg.Namespace)
	v1Routes.POST("/tiers/lookup", tier.NewHandler(tierMapper).TierLookup)

	modelMgr, errMgr := models.NewManager(
		appLogger,
		cluster.InferenceServiceLister,
		cluster.LLMInferenceServiceLister,
		cluster.HTTPRouteLister,
		models.GatewayRef{Name: cfg.GatewayName, Namespace: cfg.GatewayNamespace},
	)

	if errMgr != nil {
		appLogger.Fatal("Failed to create model manager",
			"error", errMgr,
		)
	}

	modelsHandler := handlers.NewModelsHandler(appLogger, modelMgr)

	tokenManager := token.NewManager(
		appLogger,
		cfg.Name,
		tierMapper,
		cluster.ClientSet,
		cluster.NamespaceLister,
		cluster.ServiceAccountLister,
	)
	tokenHandler := token.NewHandler(appLogger, cfg.Name, tokenManager)

	apiKeyService := api_keys.NewService(tokenManager, store)
	apiKeyHandler := api_keys.NewHandler(appLogger, apiKeyService)

	// Model listing endpoint (v1Routes is grouped under /v1, so this creates /v1/models)
	//nolint:contextcheck // Context is properly accessed via gin.Context in the returned handler
	v1Routes.GET("/models", tokenHandler.ExtractUserInfo(), modelsHandler.ListLLMs)

	//nolint:contextcheck
	tokenRoutes := v1Routes.Group("/tokens", tokenHandler.ExtractUserInfo())
	tokenRoutes.POST("", tokenHandler.IssueToken)
	tokenRoutes.DELETE("", apiKeyHandler.RevokeAllTokens)

	//nolint:contextcheck
	apiKeyRoutes := v1Routes.Group("/api-keys", tokenHandler.ExtractUserInfo())
	apiKeyRoutes.POST("", apiKeyHandler.CreateAPIKey)
	apiKeyRoutes.GET("", apiKeyHandler.ListAPIKeys)
	apiKeyRoutes.GET("/:id", apiKeyHandler.GetAPIKey)
	// Note: Single key deletion removed for initial release - use DELETE /v1/tokens to revoke all tokens
}
