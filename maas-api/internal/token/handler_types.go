package token

type Request struct {
	Expiration *Duration `json:"expiration,omitempty"` // Expiration duration object
}

type Response struct {
	Token      string `json:"token"`
	Expiration string `json:"expiration"` // e.g. "4h"
}
