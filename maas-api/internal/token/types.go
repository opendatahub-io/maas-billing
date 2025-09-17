package token

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// UserContext contains user information extracted from a token
type UserContext struct {
	Username        string   `json:"username"`
	UID             string   `json:"uid"`
	Groups          []string `json:"groups"`
	IsAuthenticated bool     `json:"is_authenticated"`
}

type Token struct {
	Token      string   `json:"token"`
	Expiration Duration `json:"expiration"`
	ExpiresAt  int64    `json:"expires_at"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

var allowedUnits = regexp.MustCompile(`^(\d+(?:\.\d+)?[hms])+$`)

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case string:
		if value == "" || value == "0" {
			return nil
		}
		if !allowedUnits.MatchString(value) {
			return fmt.Errorf("invalid duration %q: must be a positive number ending in s, m, or h (e.g. \"10s\", \"5m\", \"2h\")", value)
		}
		dur, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration{dur}
		return nil
	case float64:
		// JSON numbers are unmarshaled as float64.
		*d = Duration{time.Duration(value * float64(time.Second))}
		return nil
	default:
		return fmt.Errorf("json: cannot unmarshal %T into Go value of type Duration", value)
	}

}
