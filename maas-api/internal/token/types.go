package token

// UserContext contains user information extracted from a token
type UserContext struct {
	Username        string   `json:"username"`
	UID             string   `json:"uid"`
	Groups          []string `json:"groups"`
	IsAuthenticated bool     `json:"is_authenticated"`
}
