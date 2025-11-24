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
}

// Load loads configuration from environment variables
func Load() *Config {
	debugMode, _ := env.GetBool("DEBUG_MODE", false)

	c := &Config{
		Name:      env.GetString("INSTANCE_NAME", constant.DefaultGatewayName),
		Namespace: env.GetString("NAMESPACE", constant.DefaultNamespace),
		Port:      env.GetString("PORT", "8080"),
		DebugMode: debugMode,
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
}
