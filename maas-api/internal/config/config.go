package config

import (
	"flag"

	"k8s.io/utils/env"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/constant"
)

// Config holds application configuration
type Config struct {
	// Name of the "MaaS Instance" maas-api handles keys for
	Name string
	// Namespace where maas-api is deployed
	Namespace string

	// MaaS enabled Gateway configuration
	GatewayName      string
	GatewayNamespace string

	// Server configuration
	Port string

	// Executable-specific configuration
	DebugMode bool

	// Kubernetes configuration
	KeyNamespace string

	// Kuadrant configuration
	TokenRateLimitPolicyName string
	AuthPolicyName           string

	// Default team configuration
	CreateDefaultTeam bool
	AdminAPIKey       string

	// Database configuration
	DBPath string
}

// Load loads configuration from environment variables
func Load() *Config {
	debugMode, _ := env.GetBool("DEBUG_MODE", false)
	defaultTeam, _ := env.GetBool("CREATE_DEFAULT_TEAM", true)
	gatewayName := env.GetString("GATEWAY_NAME", constant.DefaultGatewayName)

	c := &Config{
		Name:             env.GetString("INSTANCE_NAME", gatewayName),
		Namespace:        env.GetString("NAMESPACE", constant.DefaultNamespace),
		GatewayName:      env.GetString("GATEWAY_NAME", gatewayName),
		GatewayNamespace: env.GetString("GATEWAY_NAMESPACE", constant.DefaultGatewayNamespace),
		Port:             env.GetString("PORT", "8080"),
		DebugMode:        debugMode,
		// Secrets provider configuration
		KeyNamespace:             env.GetString("KEY_NAMESPACE", "llm"),
		TokenRateLimitPolicyName: env.GetString("TOKEN_RATE_LIMIT_POLICY_NAME", "gateway-token-rate-limits"),
		AuthPolicyName:           env.GetString("AUTH_POLICY_NAME", "gateway-auth-policy"),
		CreateDefaultTeam:        defaultTeam,
		AdminAPIKey:              env.GetString("ADMIN_API_KEY", ""),
		DBPath:                   env.GetString("DB_PATH", "/data/maas.db"),
	}
	c.bindFlags(flag.CommandLine)

	return c
}

// bindFlags will parse the given flagset and bind values to selected config options
func (c *Config) bindFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Name, "name", c.Name, "Name of the MaaS instance")
	fs.StringVar(&c.Namespace, "namespace", c.Namespace, "Namespace of the MaaS instance")
	fs.StringVar(&c.GatewayName, "gateway-name", c.GatewayName, "Name of the Gateway that has MaaS capabilities")
	fs.StringVar(&c.GatewayNamespace, "gateway-namespace", c.GatewayNamespace, "Namespace where MaaS-enabled Gateway is deployed")
	fs.StringVar(&c.Port, "port", c.Port, "Port to listen on")
	fs.BoolVar(&c.DebugMode, "debug", c.DebugMode, "Enable debug mode")
	fs.StringVar(&c.DBPath, "db-path", c.DBPath, "Path to SQLite database file")
}
