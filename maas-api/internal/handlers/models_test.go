package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/pagination"
	"github.com/openai/openai-go/v2/shared/constant"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/handlers"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
	"github.com/opendatahub-io/maas-billing/maas-api/test/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"knative.dev/pkg/apis"
)

func TestListingModels(t *testing.T) {
	llmObjects := fixtures.CreateLLMTestObjects()
	config := fixtures.TestServerConfig{
		Objects: llmObjects,
	}
	router, clients := fixtures.SetupTestServer(t, config)
	modelMgr := models.NewManager(clients.DynamicClient)
	modelsHandler := handlers.NewModelsHandler(modelMgr)
	v1 := router.Group("/v1")
	v1.GET("/models", modelsHandler.ListLLMs)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/v1/models", nil)
	require.NoError(t, err, "Failed to create request")

	req.Header.Set("Authorization", "Bearer valid-token")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Expected status OK")

	var response pagination.Page[models.Model]
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Failed to unmarshal response body")

	assert.Equal(t, "list", response.Object, "Expected object type to be 'list'")
	require.Len(t, response.Data, len(llmObjects), "Mismatched number of models returned")

	modelsByName := make(map[string]models.Model)
	for _, model := range response.Data {
		modelsByName[model.ID] = model
	}

	testCases := []struct {
		name          string
		expectedModel models.Model
	}{
		{
			name: "llama-7b",
			expectedModel: models.Model{
				Model: openai.Model{
					ID:      "llama-7b",
					Object:  constant.Model("model"),
					OwnedBy: "model-serving",
				},
				URL:   mustParseURL("http://llama-7b.model-serving.svc.cluster.local/v1"),
				Ready: true,
			},
		},
		{
			name: "gpt-3-turbo",
			expectedModel: models.Model{
				Model: openai.Model{
					ID:      "gpt-3-turbo",
					Object:  constant.Model("model"),
					OwnedBy: "openai-models",
				},
				URL:   mustParseURL("http://gpt-3-turbo.openai-models.svc.cluster.local/v1"),
				Ready: true,
			},
		},
		{
			name: "bert-base",
			expectedModel: models.Model{
				Model: openai.Model{
					ID:      "bert-base",
					Object:  constant.Model("model"),
					OwnedBy: "nlp-models",
				},
				URL:   mustParseURL("http://bert-base.nlp-models.svc.cluster.local/v1"),
				Ready: false,
			},
		},
		{
			name: "model-without-url",
			expectedModel: models.Model{
				Model: openai.Model{
					ID:      "model-without-url",
					Object:  constant.Model("model"),
					OwnedBy: fixtures.TestNamespace,
				},
				URL:   nil,
				Ready: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualModel, exists := modelsByName[tc.name]
			require.True(t, exists, "Model '%s' not found in response", tc.name)

			assert.NotZero(t, actualModel.Created, "Expected 'Created' timestamp to be set")

			assert.Equal(t, tc.expectedModel.ID, actualModel.ID)
			assert.Equal(t, tc.expectedModel.Object, actualModel.Object)
			assert.Equal(t, tc.expectedModel.URL, actualModel.URL)
			assert.Equal(t, tc.expectedModel.Ready, actualModel.Ready)
		})
	}
}

func mustParseURL(rawURL string) *apis.URL {
	if rawURL == "" {
		return nil
	}
	u, err := apis.ParseURL(rawURL)
	if err != nil {
		panic("test setup failed: invalid URL: " + err.Error())
	}
	return u
}
