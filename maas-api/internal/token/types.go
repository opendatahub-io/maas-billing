package token

import (
	"encoding/json"
	"fmt"
	"time"
)

// UserContext holds user information extracted from the token.
type UserContext struct {
	Username        string
	UID             string
	Groups          []string
	IsAuthenticated bool
	JTI             string // JWT ID
}

type Token struct {
	Token      string   `json:"token"`
	Expiration Duration `json:"expiration"`
	ExpiresAt  int64    `json:"expiresAt"`
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
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
