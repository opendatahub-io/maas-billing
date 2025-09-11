package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/pagination"

	"github.com/opendatahub-io/maas-billing/key-manager/internal/models"
)

// ModelsHandler handles model-related endpoints
type ModelsHandler struct {
	modelMgr *models.Manager
}

// NewModelsHandler creates a new models handler
func NewModelsHandler(modelMgr *models.Manager) *ModelsHandler {
	return &ModelsHandler{
		modelMgr: modelMgr,
	}
}

// ListModels handles GET /models
func (h *ModelsHandler) ListModels(c *gin.Context) {
	modelList, err := h.modelMgr.ListAvailableModels(c.Request.Context())
	if err != nil {
		log.Printf("Failed to get available models: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve models"})
		return
	}

	response := models.ModelsResponse{
		Models: modelList,
	}

	c.JSON(http.StatusOK, response)
}

func (h *ModelsHandler) ListLLMs(c *gin.Context) {
	modelList, err := h.modelMgr.ListAvailableLLMs(c.Request.Context())
	if err != nil {
		log.Printf("Failed to get available LLM models: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to retrieve LLM models",
				"type":    "server_error",
			}})
		return
	}

	var response pagination.Page[openai.Model]
	response.Object = "list"
	response.Data = modelList

	c.JSON(http.StatusOK, response)
}
