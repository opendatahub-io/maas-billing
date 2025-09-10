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

// GetTierLookup handles GET /tiers/lookup?group={group}
func (h *Handler) GetTierLookup(c *gin.Context) {
	group := c.Query("group")
	if group == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "group query parameter is required",
		})
		return
	}

	h.lookupTier(c, group)
}

func (h *Handler) lookupTier(c *gin.Context, group string) {
	tier, err := h.mapper.GetTierForGroup(c.Request.Context(), group)
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

	response := LookupResponse{
		Group: group,
		Tier:  tier,
	}

	c.JSON(http.StatusOK, response)
}
