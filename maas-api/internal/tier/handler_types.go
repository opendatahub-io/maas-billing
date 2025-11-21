package tier

type LookupRequest struct {
	Groups []string `binding:"required,min=1" json:"groups"` // Array of user groups to lookup
}

type LookupResponse struct {
	Tier string `json:"tier,inline"`
}

type ErrorResponse struct {
	Error   string `json:"error"`   // Error code (e.g., "bad_request", "not_found")
	Message string `json:"message"` // Human-readable error message
}
