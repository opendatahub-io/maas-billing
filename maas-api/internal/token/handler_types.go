package token

type Request struct {
	TTL string `json:"ttl,omitempty"` // TTL duration, e.g. 1h, 10m, 90d
}

type Response struct {
	Token string `json:"token"`
}
