package tier

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	mapper *Mapper
}

func NewHandler(mapper *Mapper) *Handler {
	return &Handler{
		mapper: mapper,
	}
}

// TierLookup handles POST /tiers/lookup with JSON body containing groups array.
//
// This endpoint determines the highest level tier for a user with multiple group memberships following the rules:
// 1. Finds all tiers that contain any of the user's groups
// 2. Selects the tier with the highest level value (higher numbers win)
// 3. If multiple tiers have the same level, the first one found wins (order of tiers in the configuration source).
func (h *Handler) TierLookup(c *gin.Context) {
	var req LookupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "invalid request body: " + err.Error(),
		})
		return
	}

	tier, err := h.mapper.GetTierForGroups(req.Groups...)
	if err != nil {
		var groupNotFoundErr *GroupNotFoundError
		if errors.As(err, &groupNotFoundErr) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
			return
		}

		// All other errors are internal server errors
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "failed to lookup tier: " + err.Error(),
		})
		return
	}

	displayName := tier.DisplayName
	if displayName == "" {
		displayName = tier.Name
	}

	response := LookupResponse{
		Tier:        tier.Name,
		DisplayName: displayName,
	}

	c.JSON(http.StatusOK, response)
}
