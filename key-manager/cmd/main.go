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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

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
	// Create in-cluster config
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create in-cluster config: %v", err)
	}

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	// Create dynamic client for Kuadrant CRDs
	k8sClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to create dynamic client: %v", err)
	}

	// Initialize managers
	policyMgr := teams.NewPolicyManager(
		k8sClient,
		clientset,
		cfg.KeyNamespace,
		cfg.TokenRateLimitPolicyName,
		cfg.AuthPolicyName,
	)

	teamMgr := teams.NewManager(clientset, cfg.KeyNamespace, policyMgr)
	keyMgr := keys.NewManager(clientset, cfg.KeyNamespace, teamMgr)
	modelMgr := models.NewManager(k8sClient)

	// Initialize handlers
	usageHandler := handlers.NewUsageHandler(clientset, restConfig, cfg.KeyNamespace)
	teamsHandler := handlers.NewTeamsHandler(teamMgr)
	keysHandler := handlers.NewKeysHandler(keyMgr, teamMgr)
	modelsHandler := handlers.NewModelsHandler(modelMgr)
	legacyHandler := handlers.NewLegacyHandler(keyMgr)
	healthHandler := handlers.NewHealthHandler()

	// Create default team if enabled
	if cfg.CreateDefaultTeam {
		if err := teamMgr.CreateDefaultTeam(); err != nil {
			log.Printf("Warning: Failed to create default team: %v", err)
		} else {
			log.Printf("Default team created successfully")
		}
	}

	// Initialize Gin router
	router := gin.Default()

	// Health check endpoint (no auth required)
	router.GET("/health", healthHandler.HealthCheck)

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
