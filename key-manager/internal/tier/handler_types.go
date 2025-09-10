package tier

// LookupResponse represents the response for tier lookup
type LookupResponse struct {
	Group string `json:"group"`
	Tier  string `json:"tier"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
