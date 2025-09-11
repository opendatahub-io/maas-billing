package config

import (
	"flag"
	"os"
)

// Config holds application configuration
type Config struct {
	// Name of the "MaaS Instance" maas-api handles keys for
	Name string
	// Namespace where maas-api is deployed
	Namespace string

	// Server configuration
	Port string

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
	c := &Config{
		Name:      getEnvOrDefault("INSTANCE_NAME", "openshift-ai-inference"),
		Namespace: getEnvOrDefault("NAMESPACE", "maas-api"),
		Port:      getEnvOrDefault("PORT", "8080"),
		// Secrets provider configuration
		KeyNamespace:             getEnvOrDefault("KEY_NAMESPACE", "llm"),
		SecretSelectorLabel:      getEnvOrDefault("SECRET_SELECTOR_LABEL", "kuadrant.io/apikeys-by"),
		SecretSelectorValue:      getEnvOrDefault("SECRET_SELECTOR_VALUE", "rhcl-keys"),
		TokenRateLimitPolicyName: getEnvOrDefault("TOKEN_RATE_LIMIT_POLICY_NAME", "gateway-token-rate-limits"),
		AuthPolicyName:           getEnvOrDefault("AUTH_POLICY_NAME", "gateway-auth-policy"),
		CreateDefaultTeam:        getEnvOrDefault("CREATE_DEFAULT_TEAM", "true") == "true",
		AdminAPIKey:              getEnvOrDefault("ADMIN_API_KEY", ""),
	}

	c.bindFlags(flag.CommandLine)

	return c
}

// bindFlags will parse the given flagset and bind values to selected config options
func (c *Config) bindFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Name, "name", c.Name, "Name of the MaaS instance")
	fs.StringVar(&c.Namespace, "namespace", c.Namespace, "Namespace")
	fs.StringVar(&c.Port, "port", c.Port, "Port to listen on")
}

// getEnvOrDefault gets environment variable or returns default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
