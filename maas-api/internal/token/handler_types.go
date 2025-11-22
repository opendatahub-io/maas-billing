package token

type Request struct {
	// Accepts either:
	// - String: Go-style duration starting from seconds (e.g. `"30s"`, `"2h45m"`)
	// - Number: Seconds (e.g. `3600`)
	Expiration *Duration `json:"expiration,omitempty"`
	// Name is an optional identifier for the token. If provided, the token's
	// metadata will be persisted in the database.
	Name string `json:"name,omitempty"`
}

type Response struct {
	*Token `json:",inline,omitempty"`
}
