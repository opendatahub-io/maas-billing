package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v2/packages/pagination"

	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/logger"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/models"
)

// ModelsHandler handles model-related endpoints.
type ModelsHandler struct {
	modelMgr *models.Manager
	logger   *logger.Logger
}

// NewModelsHandler creates a new models handler.
func NewModelsHandler(log *logger.Logger, modelMgr *models.Manager) *ModelsHandler {
	if log == nil {
		log = logger.Production()
	}
	return &ModelsHandler{
		modelMgr: modelMgr,
		logger:   log,
	}
}

// ListModels handles GET /models.
func (h *ModelsHandler) ListModels(c *gin.Context) {
	modelList, err := h.modelMgr.ListAvailableModels()
	if err != nil {
		h.logger.Error("Failed to get available models",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve models"})
		return
	}

	c.JSON(http.StatusOK, pagination.Page[models.Model]{
		Object: "list",
		Data:   modelList,
	})
}

// ListLLMs handles GET /v1/models.
func (h *ModelsHandler) ListLLMs(c *gin.Context) {
	modelList, err := h.modelMgr.ListAvailableLLMs()
	if err != nil {
		h.logger.Error("Failed to get available LLM models",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to retrieve LLM models",
				"type":    "server_error",
			}})
		return
	}

	c.JSON(http.StatusOK, pagination.Page[models.Model]{
		Object: "list",
		Data:   modelList,
	})
}
