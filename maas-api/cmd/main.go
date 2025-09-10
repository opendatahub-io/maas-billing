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

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/auth"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/config"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/handlers"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/keys"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/teams"
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

	ctx, cancel := context.WithCancel(context.Background())

	router := registerHandlers(ctx, cfg)

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

func registerHandlers(ctx context.Context, cfg *config.Config) *gin.Engine {
	router := gin.Default()

	router.GET("/health", handlers.NewHealthHandler().HealthCheck)

	clusterConfig, err := config.NewClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	modelMgr := models.NewManager(clusterConfig.DynClient)
	modelsHandler := handlers.NewModelsHandler(modelMgr)
	router.GET("/models", modelsHandler.ListModels)
	router.GET("/v1/models", modelsHandler.ListLLMs)

	switch cfg.Provider {
	case config.Secrets:
		configureSecretsProvider(cfg, router, clusterConfig)
	case config.SATokens:
		configureSATokenProvider(ctx, cfg, router, clusterConfig)
	default:
		log.Fatalf("Invalid provider: %s. Available providers: [secrets, sa-tokens]", cfg.Provider)
	}

	return router
}

func configureSATokenProvider(ctx context.Context, cfg *config.Config, router *gin.Engine, clusterConfig *config.K8sClusterConfig) {
	tierMapper := tier.NewMapper(clusterConfig.ClientSet, cfg.Name, cfg.Namespace)
	tierHandler := tier.NewHandler(tierMapper)
	router.POST("/tiers/lookup", tierHandler.TierLookup)

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

	manager := token.NewManager(
		cfg.Name,
		tierMapper,
		clusterConfig.ClientSet,
		namespaceInformer.Lister(),
		serviceAccountInformer.Lister(),
	)
	tokenHandler := token.NewHandler(cfg.Name, manager)

	v1Routes := router.Group("/v1")

	tokenRoutes := v1Routes.Group("/tokens", token.ExtractUserInfo(token.NewReviewer(clusterConfig.ClientSet)))
	tokenRoutes.POST("", tokenHandler.IssueToken)
	tokenRoutes.DELETE("", tokenHandler.RevokeAllTokens)
}

func configureSecretsProvider(cfg *config.Config, router *gin.Engine, clusterConfig *config.K8sClusterConfig) {
	policyMgr := teams.NewPolicyManager(
		clusterConfig.DynClient,
		clusterConfig.ClientSet,
		cfg.KeyNamespace,
		cfg.TokenRateLimitPolicyName,
		cfg.AuthPolicyName,
	)

	teamMgr := teams.NewManager(clusterConfig.ClientSet, cfg.KeyNamespace, policyMgr)
	keyMgr := keys.NewManager(clusterConfig.ClientSet, cfg.KeyNamespace, teamMgr)

	usageHandler := handlers.NewUsageHandler(clusterConfig.ClientSet, clusterConfig.RestConfig, cfg.KeyNamespace)
	teamsHandler := handlers.NewTeamsHandler(teamMgr)
	keysHandler := handlers.NewKeysHandler(keyMgr, teamMgr)

	if cfg.CreateDefaultTeam {
		if err := teamMgr.CreateDefaultTeam(); err != nil {
			log.Printf("Warning: Failed to create default team: %v", err)
		} else {
			log.Printf("Default team created successfully")
		}
	}

	// Team management endpoints
	teamRoutes := router.Group("/teams", auth.AdminAuthMiddleware())
	teamRoutes.POST("", teamsHandler.CreateTeam)
	teamRoutes.GET("", teamsHandler.ListTeams)
	teamRoutes.GET("/:team_id", teamsHandler.GetTeam)
	teamRoutes.PATCH("/:team_id", teamsHandler.UpdateTeam)
	teamRoutes.DELETE("/:team_id", teamsHandler.DeleteTeam)
	teamRoutes.POST("/:team_id/keys", keysHandler.CreateTeamKey)
	teamRoutes.GET("/:team_id/keys", keysHandler.ListTeamKeys)
	teamRoutes.GET("/:team_id/usage", usageHandler.GetTeamUsage)

	// User management endpoints
	userRoutes := router.Group("/users", auth.AdminAuthMiddleware())
	userRoutes.GET("/:user_id/keys", keysHandler.ListUserKeys)
	userRoutes.GET("/:user_id/usage", usageHandler.GetUserUsage)

	// Key management endpoints
	keyRoutes := router.Group("/keys", auth.AdminAuthMiddleware())
	keyRoutes.DELETE("/:key_name", keysHandler.DeleteTeamKey)
}
