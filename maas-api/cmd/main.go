package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/config"
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

	cleanup, _ := registerHandlers(ctx, router, cfg)
	defer cleanup()

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
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited gracefully")
}

func registerHandlers(ctx context.Context, router *gin.Engine, cfg *config.Config) (func(), *token.Store) {
	router.GET("/health", handlers.NewHealthHandler().HealthCheck)

	clusterConfig, err := config.NewClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	modelMgr := models.NewManager(clusterConfig.KServeV1Beta1, clusterConfig.KServeV1Alpha1)
	modelsHandler := handlers.NewModelsHandler(modelMgr)

	return configureSATokenProvider(ctx, cfg, router, clusterConfig, modelsHandler)
}

func configureSATokenProvider(ctx context.Context, cfg *config.Config, router *gin.Engine, clusterConfig *config.K8sClusterConfig, modelsHandler *handlers.ModelsHandler) (func(), *token.Store) {
	// V1 API routes
	v1Routes := router.Group("/v1")

	tierMapper := tier.NewMapper(clusterConfig.ClientSet, cfg.Name, cfg.Namespace)
	tierHandler := tier.NewHandler(tierMapper)
	v1Routes.POST("/tiers/lookup", tierHandler.TierLookup)

	informerFactory := informers.NewSharedInformerFactory(clusterConfig.ClientSet, 30*time.Second)

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

	store, err := token.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize token store: %v", err)
	}

	manager := token.NewManager(
		cfg.Name,
		tierMapper,
		clusterConfig.ClientSet,
		namespaceInformer.Lister(),
		serviceAccountInformer.Lister(),
		store,
	)
	tokenHandler := token.NewHandler(cfg.Name, manager)
	// Create reviewer with audience to properly validate Service Account tokens
	reviewer := token.NewReviewerWithAudience(clusterConfig.ClientSet, cfg.Name+"-sa")

	// Protect model listing endpoints with token validation (includes deny list check)
	router.GET("/models", tokenHandler.ExtractUserInfo(reviewer), modelsHandler.ListModels)
	v1Routes.GET("/models", tokenHandler.ExtractUserInfo(reviewer), modelsHandler.ListLLMs)

	tokenRoutes := v1Routes.Group("/tokens", tokenHandler.ExtractUserInfo(reviewer))
	tokenRoutes.POST("", tokenHandler.IssueToken)
	tokenRoutes.DELETE("", tokenHandler.RevokeAllTokens)

	apiKeyRoutes := v1Routes.Group("/api-keys", tokenHandler.ExtractUserInfo(reviewer))
	apiKeyRoutes.POST("", tokenHandler.CreateAPIKey)
	apiKeyRoutes.GET("", tokenHandler.ListAPIKeys)
	apiKeyRoutes.GET("/:id", tokenHandler.GetAPIKey)

	cleanup := func() {
		if err := store.Close(); err != nil {
			log.Printf("Failed to close token store: %v", err)
		}
	}
	return cleanup, store
}
