package token

import (
	"encoding/json"
	"fmt"
	"time"
)

// UserContext holds user information extracted from the token.
type UserContext struct {
	Username        string   `json:"username"`
	UID             string   `json:"uid"`
	Groups          []string `json:"groups"`
	IsAuthenticated bool     `json:"isAuthenticated"`
	JTI             string   `json:"jti"` // JWT ID
}

type Token struct {
	Token      string   `json:"token"`
	Expiration Duration `json:"expiration"`
	ExpiresAt  int64    `json:"expiresAt"`
	JTI        string   `json:"jti,omitempty"`
	Name       string   `json:"name,omitempty"`
	Namespace  string   `json:"-"` // Internal use only
}

type Duration struct {
	time.Duration
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	if d == nil {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value) * time.Second
		return nil
	case string:
		if value == "" {
			d.Duration = 0
			return nil
		}
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid duration")
	}
}
