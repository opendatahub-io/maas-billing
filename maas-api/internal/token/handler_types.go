package token

type Request struct {
	TTL string `json:"ttl,omitempty"` // TTL duration, e.g. 1h, 10m, 90d
}

type Response struct {
	Token     string `json:"token"`
	TTL       string `json:"ttl"`        // e.g. "4h"
	ExpiresAt string `json:"expires_at"` // e.g. "2025-09-17T05:15:30Z"
}
