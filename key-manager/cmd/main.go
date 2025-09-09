package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/opendatahub-io/maas-billing/key-manager/internal/auth"
	"github.com/opendatahub-io/maas-billing/key-manager/internal/config"
	"github.com/opendatahub-io/maas-billing/key-manager/internal/handlers"
	"github.com/opendatahub-io/maas-billing/key-manager/internal/keys"
	"github.com/opendatahub-io/maas-billing/key-manager/internal/models"
	"github.com/opendatahub-io/maas-billing/key-manager/internal/teams"
)

func main() {
	cfg := config.Load()

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
	modelMgr := models.NewManager(clusterConfig.DynClient)

	// Initialize handlers
	usageHandler := handlers.NewUsageHandler(clusterConfig.ClientSet, clusterConfig.RestConfig, cfg.KeyNamespace)
	teamsHandler := handlers.NewTeamsHandler(teamMgr)
	keysHandler := handlers.NewKeysHandler(keyMgr, teamMgr)
	modelsHandler := handlers.NewModelsHandler(modelMgr)
	legacyHandler := handlers.NewLegacyHandler(keyMgr)

	// Create default team if enabled
	if cfg.CreateDefaultTeam {
		if err := teamMgr.CreateDefaultTeam(); err != nil {
			log.Printf("Warning: Failed to create default team: %v", err)
		} else {
			log.Printf("Default team created successfully")
		}
	}

	// Setup API routes with admin authentication
	adminRoutes := router.Group("/", auth.AdminAuthMiddleware())

	// Legacy endpoints (backward compatibility)
	adminRoutes.POST("/generate_key", legacyHandler.GenerateKey)
	adminRoutes.DELETE("/delete_key", legacyHandler.DeleteKey)

	// Team management endpoints
	adminRoutes.POST("/teams", teamsHandler.CreateTeam)
	adminRoutes.GET("/teams", teamsHandler.ListTeams)
	adminRoutes.GET("/teams/:team_id", teamsHandler.GetTeam)
	adminRoutes.PATCH("/teams/:team_id", teamsHandler.UpdateTeam)
	adminRoutes.DELETE("/teams/:team_id", teamsHandler.DeleteTeam)

	// Team-scoped API key management
	adminRoutes.POST("/teams/:team_id/keys", keysHandler.CreateTeamKey)
	adminRoutes.GET("/teams/:team_id/keys", keysHandler.ListTeamKeys)
	adminRoutes.DELETE("/keys/:key_name", keysHandler.DeleteTeamKey)

	// User key management
	adminRoutes.GET("/users/:user_id/keys", keysHandler.ListUserKeys)

	// Usage endpoints
	adminRoutes.GET("/users/:user_id/usage", usageHandler.GetUserUsage)
	adminRoutes.GET("/teams/:team_id/usage", usageHandler.GetTeamUsage)

	// Model listing endpoint
	adminRoutes.GET("/models", modelsHandler.ListModels)

	return router
}
