package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/opendatahub-io/maas-billing/maas-api/internal/logger"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
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

	// Initialize store in main for proper cleanup
	store, err := api_keys.NewStore(ctx, cfg.DBPath, appLogger)
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

func registerHandlers(ctx context.Context, router *gin.Engine, cfg *config.Config, store *api_keys.Store, appLogger *logger.Logger) {
	router.GET("/health", handlers.NewHealthHandler().HealthCheck)

	clusterConfig, err := config.NewClusterConfig()
	if err != nil {
		appLogger.Fatal("Failed to create Kubernetes client",
			"error", err,
		)
	}

	gatewayRef := models.GatewayRef{
		Name:      cfg.GatewayName,
		Namespace: cfg.GatewayNamespace,
	}
	modelMgr := models.NewManager(clusterConfig.KServeV1Beta1, clusterConfig.KServeV1Alpha1, clusterConfig.Gateway, gatewayRef, appLogger)
	modelsHandler := handlers.NewModelsHandler(modelMgr, appLogger)

	// V1 API routes
	v1Routes := router.Group("/v1")

	tierMapper := tier.NewMapper(ctx, clusterConfig.ClientSet, cfg.Name, cfg.Namespace, appLogger)
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
		appLogger.Fatal("Failed to sync informer caches")
	}

	// Create token manager (ephemeral, K8s logic)
	tokenManager := token.NewManager(
		cfg.Name,
		tierMapper,
		clusterConfig.ClientSet,
		namespaceInformer.Lister(),
		serviceAccountInformer.Lister(),
		appLogger,
	)
	tokenHandler := token.NewHandler(cfg.Name, tokenManager, appLogger)

	// Create api key service (persistent, SQLite logic)
	apiKeyService := api_keys.NewService(tokenManager, store)
	apiKeyHandler := api_keys.NewHandler(apiKeyService, appLogger)

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
