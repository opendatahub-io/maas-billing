package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v2/packages/pagination"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
)

// ModelsHandler handles model-related endpoints.
type ModelsHandler struct {
	modelMgr *models.Manager
}

// NewModelsHandler creates a new models handler.
func NewModelsHandler(modelMgr *models.Manager) *ModelsHandler {
	return &ModelsHandler{
		modelMgr: modelMgr,
	}
}

// ListModels handles GET /models.
func (h *ModelsHandler) ListModels(c *gin.Context) {
	modelList, err := h.modelMgr.ListAvailableModels()
	if err != nil {
		log.Printf("Failed to get available models: %v", err)
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
		log.Printf("Failed to get available LLM models: %v", err)
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
