package config

import (
	"flag"

	"k8s.io/utils/env"
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

	// Provider config
	Provider ProviderType

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

	c := &Config{
		Name:      env.GetString("INSTANCE_NAME", "openshift-ai-inference"),
		Namespace: env.GetString("NAMESPACE", "maas-api"),
		Port:      env.GetString("PORT", "8080"),
		Provider:  ProviderType(env.GetString("PROVIDER", string(SATokens))),
		DebugMode: debugMode,
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
	fs.BoolVar(&c.DebugMode, "debug", c.DebugMode, "Enable debug mode")
	fs.Var(&c.Provider, "provider", "Provider type to use for API keys")
}
