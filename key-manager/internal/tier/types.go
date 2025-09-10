package tier

import "fmt"

// Tier represents a subscription tier with associated user groups
type Tier struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Groups      []string `yaml:"groups"`
}

// GroupNotFoundError indicates that a group was not found in any tier
type GroupNotFoundError struct {
	Group string
}

func (e *GroupNotFoundError) Error() string {
	return fmt.Sprintf("group %s not found in any tier", e.Group)
}
