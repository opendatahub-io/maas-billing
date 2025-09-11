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

	"github.com/opendatahub-io/maas-billing/maas-api/internal/auth"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/config"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/handlers"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/keys"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/teams"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
)

func main() {
	cfg := config.Load()
	flag.Parse()

	router := registerHandlers(cfg)

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

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited gracefully")
}

func registerHandlers(cfg *config.Config) *gin.Engine {
	router := gin.Default()

	// Health check endpoint (no auth required)
	router.GET("/health", handlers.NewHealthHandler().HealthCheck)

	clusterConfig, err := config.NewClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	tierMapper := tier.NewMapper(clusterConfig.ClientSet, cfg.Namespace)
	tierHandler := tier.NewHandler(tierMapper)
	router.POST("/tiers/lookup", tierHandler.TierLookup)

	modelMgr := models.NewManager(clusterConfig.DynClient)
	modelsHandler := handlers.NewModelsHandler(modelMgr)
	router.GET("/models", modelsHandler.ListModels)
	router.GET("/v1/models", modelsHandler.ListLLMs)

	// Initialize managers
	policyMgr := teams.NewPolicyManager(
		clusterConfig.DynClient,
		clusterConfig.ClientSet,
		cfg.KeyNamespace,
		cfg.TokenRateLimitPolicyName,
		cfg.AuthPolicyName,
	)

	teamMgr := teams.NewManager(clusterConfig.ClientSet, cfg.KeyNamespace, policyMgr)
	keyMgr := keys.NewManager(clusterConfig.ClientSet, cfg.KeyNamespace, teamMgr)

	// Initialize handlers
	usageHandler := handlers.NewUsageHandler(clusterConfig.ClientSet, clusterConfig.RestConfig, cfg.KeyNamespace)
	teamsHandler := handlers.NewTeamsHandler(teamMgr)
	keysHandler := handlers.NewKeysHandler(keyMgr, teamMgr)

	// Create default team if enabled
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

	return router
}
