package config

import (
	"fmt"
	"strings"
)

var (
	Secrets  ProviderType = "secrets"
	SATokens ProviderType = "sa-tokens"
)

type ProviderType string

func (p *ProviderType) Set(s string) error {
	switch strings.ToLower(s) {
	case string(Secrets):
		*p = Secrets
	case string(SATokens):
		*p = SATokens
	default:
		return fmt.Errorf("unknown provider type %q (valid: %s, %s)", s, Secrets, SATokens)
	}
	return nil
}

func (p *ProviderType) String() string {
	switch *p {
	case Secrets:
		return string(Secrets)
	case SATokens:
		return string(SATokens)
	default:
		return "unknown"
	}
}
