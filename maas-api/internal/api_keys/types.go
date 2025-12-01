package api_keys

// NamedToken represents metadata for a single token
type NamedToken struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CreationDate   string `json:"creationDate"`
	ExpirationDate string `json:"expirationDate"`
	Status         string `json:"status"` // "active", "expired"
}

