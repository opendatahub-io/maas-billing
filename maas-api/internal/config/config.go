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

	DebugMode bool
	// Server configuration
	Port string
	// Optional HTTPS listener configuration
	TLSPort     string
	TLSCertFile string
	TLSKeyFile  string
	DisableHTTP bool

	// Kubernetes configuration
	KeyNamespace        string
	SecretSelectorLabel string
	SecretSelectorValue string

	// Kuadrant configuration
	TokenRateLimitPolicyName string
	AuthPolicyName           string

	// Default team configuration
	CreateDefaultTeam bool
	AdminAPIKey       string
}

// Load loads configuration from environment variables
func Load() *Config {
	debugMode, _ := env.GetBool("DEBUG_MODE", false)
	defaultTeam, _ := env.GetBool("CREATE_DEFAULT_TEAM", true)
	disableHTTP, _ := env.GetBool("DISABLE_HTTP", false)

	c := &Config{
		Name:        env.GetString("INSTANCE_NAME", constant.DefaultGatewayName),
		Namespace:   env.GetString("NAMESPACE", constant.DefaultNamespace),
		Port:        env.GetString("PORT", "8080"),
		TLSPort:     env.GetString("TLS_PORT", "8443"),
		TLSCertFile: env.GetString("TLS_CERT_FILE", ""),
		TLSKeyFile:  env.GetString("TLS_KEY_FILE", ""),
		DisableHTTP: disableHTTP,
		DebugMode:   debugMode,
		// Secrets provider configuration
		KeyNamespace:             env.GetString("KEY_NAMESPACE", "llm"),
		SecretSelectorLabel:      env.GetString("SECRET_SELECTOR_LABEL", "kuadrant.io/apikeys-by"),
		SecretSelectorValue:      env.GetString("SECRET_SELECTOR_VALUE", "rhcl-keys"),
		TokenRateLimitPolicyName: env.GetString("TOKEN_RATE_LIMIT_POLICY_NAME", "gateway-token-rate-limits"),
		AuthPolicyName:           env.GetString("AUTH_POLICY_NAME", "gateway-auth-policy"),
		CreateDefaultTeam:        defaultTeam,
		AdminAPIKey:              env.GetString("ADMIN_API_KEY", ""),
	}
	c.bindFlags(flag.CommandLine)

	return c
}

// bindFlags will parse the given flagset and bind values to selected config options
func (c *Config) bindFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Name, "name", c.Name, "Name of the MaaS instance")
	fs.StringVar(&c.Namespace, "namespace", c.Namespace, "Namespace")
	fs.StringVar(&c.Port, "port", c.Port, "Port to listen on")
	fs.StringVar(&c.TLSPort, "tls-port", c.TLSPort, "HTTPS port to listen on")
	fs.StringVar(&c.TLSCertFile, "tls-cert-file", c.TLSCertFile, "Path to TLS certificate")
	fs.StringVar(&c.TLSKeyFile, "tls-key-file", c.TLSKeyFile, "Path to TLS private key")
	fs.BoolVar(&c.DisableHTTP, "disable-http", c.DisableHTTP, "Disable plain HTTP listener")
	fs.BoolVar(&c.DebugMode, "debug", c.DebugMode, "Enable debug mode")
}

// TLSEnabled reports whether both TLS cert and key have been configured.
func (c *Config) TLSEnabled() bool {
	return c.TLSCertFile != "" && c.TLSKeyFile != ""
}
