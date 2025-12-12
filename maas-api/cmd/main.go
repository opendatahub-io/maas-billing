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

	// Initialize store in main for proper cleanup
	store, err := api_keys.NewStore(ctx, cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize token store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("Failed to close token store: %v", err)
		}
	}()

	registerHandlers(ctx, router, cfg, store)

	listeners := buildListeners(cfg, router)

	listenerErrors := make(chan listenerError, len(listeners))
	for _, l := range listeners {
		go func(l serverListener) {
			log.Printf("%s server starting on %s", l.protocol, l.server.Addr)
			var err error
			if l.tls {
				err = l.server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
			} else {
				err = l.server.ListenAndServe()
			}
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				listenerErrors <- listenerError{listener: l, err: err}
			}
		}(l)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var listenerFailed bool
	select {
	case sig := <-quit:
		log.Printf("Shutdown signal (%s) received, shutting down server(s)...", sig)
	case listenerErr := <-listenerErrors:
		log.Printf("%s server failed: %v", listenerErr.listener.protocol, listenerErr.err)
		listenerFailed = true
	}

	cancel()
	for _, l := range listeners {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
		if err := l.server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("%s server forced to shutdown: %v", l.protocol, err)
		}
		cancelShutdown()
	}

	if listenerFailed {
		log.Fatal("exiting due to listener failure") //nolint:gocritic // exits immediately after graceful cleanup
	}
	log.Println("Server(s) exited gracefully")
}

func registerHandlers(ctx context.Context, router *gin.Engine, cfg *config.Config, store *api_keys.Store) {
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

	// Create api key service (persistent, SQLite logic)
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

type serverListener struct {
	server   *http.Server
	protocol string
	tls      bool
}

type listenerError struct {
	listener serverListener
	err      error
}

func buildListeners(cfg *config.Config, handler http.Handler) []serverListener {
	listeners := make([]serverListener, 0, 2)

	if !cfg.DisableHTTP {
		listeners = append(listeners, serverListener{
			server:   newHTTPServer(cfg.Port, handler),
			protocol: "HTTP",
			tls:      false,
		})
	}

	if cfg.TLSEnabled() {
		listeners = append(listeners, serverListener{
			server:   newHTTPServer(cfg.TLSPort, handler),
			protocol: "HTTPS",
			tls:      true,
		})
	}

	if len(listeners) == 0 {
		log.Fatal("no listeners configured; enable HTTP or provide TLS configuration")
	}

	return listeners
}

func newHTTPServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
