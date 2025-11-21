package tier

import "fmt"

// Tier represents a subscription tier with associated user groups and level.
//
// Level determines precedence, where higher values take precedence over lower values.
// This can be needed in scenarios when users belong to multiple groups across different tiers.
type Tier struct {
	Name        string   `yaml:"name"`                  // Tier name (e.g., "free", "premium", "enterprise")
	Description string   `yaml:"description,omitempty"` // Human-readable description
	Groups      []string `yaml:"groups"`                // List of groups that belong to this tier
	Level       int      `yaml:"level,omitempty"`       // Level for importance (higher wins)
}

// GroupNotFoundError indicates that a group was not found in any tier.
type GroupNotFoundError struct {
	Group string
}

func (e *GroupNotFoundError) Error() string {
	return fmt.Sprintf("group %s not found in any tier", e.Group)
}
