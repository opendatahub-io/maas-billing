package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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

	store, err := initStore(ctx, appLogger, cfg)
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

	registerHandlers(ctx, appLogger, router, cfg, store)

	listeners := buildListeners(appLogger, cfg, router)

	listenerErrors := make(chan listenerError, len(listeners))
	for _, l := range listeners {
		go func(l serverListener) {
			appLogger.Infof("%s server starting on %s", l.protocol, l.server.Addr)
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
		appLogger.Infof("Shutdown signal (%s) received, shutting down server(s)...", sig)
	case listenerErr := <-listenerErrors:
		appLogger.Errorf("%s server failed: %v", listenerErr.listener.protocol, listenerErr.err)
		listenerFailed = true
	}

	cancel()
	for _, l := range listeners {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
		if err := l.server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Errorf("%s server forced to shutdown: %v", l.protocol, err)
		}
		cancelShutdown()
	}

	if listenerFailed {
		appLogger.Fatal("exiting due to listener failure")
	}

	appLogger.Info("Server(s) exited gracefully")
}

// initStore creates the store based on the configured storage mode.
//
// Storage modes:
//   - in-memory (default): Ephemeral storage, data lost on restart
//   - disk: Persistent local storage using a file (single replica only)
//   - external: External database (PostgreSQL), supports multiple replicas
//
//nolint:ireturn // Returns MetadataStore interface by design for pluggable storage backends.
func initStore(ctx context.Context, log *logger.Logger, cfg *config.Config) (api_keys.MetadataStore, error) {
	switch cfg.StorageMode {
	case config.StorageModeInMemory, "":
		log.Info("Using in-memory storage (data will be lost on restart). " +
			"For persistent storage, use --storage=disk or --storage=external")
		return api_keys.NewSQLiteStore(ctx, log, ":memory:")

	case config.StorageModeDisk:
		dataPath := strings.TrimSpace(cfg.DataPath)
		if dataPath == "" {
			dataPath = config.DefaultDataPath
		}
		log.Info("Using persistent disk storage", "path", dataPath)
		return api_keys.NewSQLiteStore(ctx, log, dataPath)

	case config.StorageModeExternal:
		dbURL := strings.TrimSpace(cfg.DBConnectionURL)
		if dbURL == "" {
			return nil, errors.New("--db-connection-url is required when using --storage=external")
		}
		log.Info("Connecting to external database...")
		return api_keys.NewExternalStore(ctx, log, dbURL)

	default:
		return nil, fmt.Errorf("unknown storage mode: %q (valid modes: in-memory, disk, external)", cfg.StorageMode)
	}
}

func registerHandlers(ctx context.Context, log *logger.Logger, router *gin.Engine, cfg *config.Config, store api_keys.MetadataStore) {
	router.GET("/health", handlers.NewHealthHandler().HealthCheck)

	cluster, err := config.NewClusterConfig(cfg.Namespace, constant.DefaultResyncPeriod)
	if err != nil {
		log.Fatal("Failed to create cluster config",
			"error", err,
		)
	}

	if !cluster.StartAndWaitForSync(ctx.Done()) {
		log.Fatal("Failed to sync informer caches")
	}

	v1Routes := router.Group("/v1")

	tierMapper := tier.NewMapper(log, cluster.ConfigMapLister, cfg.Name, cfg.Namespace)
	v1Routes.POST("/tiers/lookup", tier.NewHandler(tierMapper).TierLookup)

	modelMgr, errMgr := models.NewManager(
		log,
		cluster.InferenceServiceLister,
		cluster.LLMInferenceServiceLister,
		cluster.HTTPRouteLister,
		models.GatewayRef{Name: cfg.GatewayName, Namespace: cfg.GatewayNamespace},
	)

	if errMgr != nil {
		log.Fatal("Failed to create model manager",
			"error", errMgr,
		)
	}

	modelsHandler := handlers.NewModelsHandler(log, modelMgr)

	tokenManager := token.NewManager(
		log,
		cfg.Name,
		tierMapper,
		cluster.ClientSet,
		cluster.NamespaceLister,
		cluster.ServiceAccountLister,
	)
	tokenHandler := token.NewHandler(log, cfg.Name, tokenManager)

	apiKeyService := api_keys.NewService(tokenManager, store)
	apiKeyHandler := api_keys.NewHandler(log, apiKeyService)

	// Model listing endpoint (v1Routes is grouped under /v1, so this creates /v1/models)
	v1Routes.GET("/models", tokenHandler.ExtractUserInfo(), modelsHandler.ListLLMs)

	tokenRoutes := v1Routes.Group("/tokens", tokenHandler.ExtractUserInfo())
	tokenRoutes.POST("", tokenHandler.IssueToken)
	tokenRoutes.DELETE("", apiKeyHandler.RevokeAllTokens)

	apiKeyRoutes := v1Routes.Group("/api-keys", tokenHandler.ExtractUserInfo())
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

func buildListeners(logger *logger.Logger, cfg *config.Config, handler http.Handler) []serverListener {
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
		logger.Fatal("no listeners configured; enable HTTP or provide TLS configuration")
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
