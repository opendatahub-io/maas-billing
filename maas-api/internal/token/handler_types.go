package token

type Request struct {
	// Accepts either:
	// - String: Go-style duration starting from seconds (e.g. `"30s"`, `"2h45m"`)
	// - Number: Seconds (e.g. `3600`)
	Expiration *Duration `json:"expiration,omitempty"`
}

type Response struct {
	*Token `json:",inline,omitempty"`
}
