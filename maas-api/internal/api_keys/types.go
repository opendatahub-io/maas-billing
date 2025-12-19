package api_keys

import "github.com/opendatahub-io/models-as-a-service/maas-api/internal/token"

// APIKey represents a full API key with token and metadata.
// It embeds token.Token and adds API key-specific fields.
type APIKey struct {
	token.Token

	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ApiKeyMetadata represents metadata for a single API key (without the token itself).
// Used for listing and retrieving API key metadata from the database.
type ApiKeyMetadata struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	CreationDate   string `json:"creationDate"`
	ExpirationDate string `json:"expirationDate"`
	Status         string `json:"status"` // "active", "expired"
}
