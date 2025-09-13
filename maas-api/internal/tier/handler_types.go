package tier

type LookupRequest struct {
	Groups []string `json:"groups" binding:"required,min=1"` // Array of user groups to lookup
}

type LookupResponse struct {
	Tier string `json:"tier,inline"`
}

type ErrorResponse struct {
	Error   string `json:"error"`   // Error code (e.g., "bad_request", "not_found")
	Message string `json:"message"` // Human-readable error message
}
