package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/pagination"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/handlers"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
	"github.com/opendatahub-io/maas-billing/maas-api/test/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"knative.dev/pkg/apis"
)

func TestListingModels(t *testing.T) {
	strptr := func(s string) *string { return &s }

	llmTestScenarios := []fixtures.LLMTestScenario{
		{
			Name:      "llama-7b",
			Namespace: "model-serving",
			URL:       fixtures.PublicURL("http://llama-7b.model-serving.acme.com/v1"),
			Ready:     true,
		},
		{
			Name:      "gpt-3-turbo",
			Namespace: "openai-models",
			URL:       fixtures.PublicURL("http://gpt-3-turbo.openai-models.acme.com/v1"),
			Ready:     true,
		},
		{
			Name:      "bert-base",
			Namespace: "nlp-models",
			URL:       fixtures.PublicURL("http://bert-base.nlp-models.svc.acme.me/v1"),
			Ready:     false,
		},
		{
			Name:      "llama-7b-private-url",
			Namespace: "model-serving",
			URL:       fixtures.AddressEntry("http://10.0.32.128/model-serving/llama-7b-private-url/v1"),
			Ready:     true,
		},
		{
			Name:      "model-without-url",
			Namespace: fixtures.TestNamespace,
			URL:       fixtures.PublicURL(""),
			Ready:     false,
		},
		{
			Name:          "fallback-model-name",
			Namespace:     fixtures.TestNamespace,
			URL:           fixtures.PublicURL("http://fallback-model-name." + fixtures.TestNamespace + ".acme.com/v1"),
			Ready:         true,
			SpecModelName: strptr("fallback-model-name"),
		},
	}
	llmInferenceServices := fixtures.CreateLLMInferenceServices(llmTestScenarios...)

	config := fixtures.TestServerConfig{
		Objects: llmInferenceServices,
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
	require.Len(t, response.Data, len(llmInferenceServices), "Mismatched number of models returned")

	modelsByName := make(map[string]models.Model)
	for _, model := range response.Data {
		modelsByName[model.ID] = model
	}

	type expectedModel struct {
		name          string
		expectedModel models.Model
	}
	var testCases []expectedModel

	for _, llmTestScenario := range llmTestScenarios {
		// expected ID mirrors toModels(): fallback to metadata.name unless spec.model.name is non-empty
		expectedModelID := llmTestScenario.Name
		if llmTestScenario.SpecModelName != nil && *llmTestScenario.SpecModelName != "" {
			expectedModelID = *llmTestScenario.SpecModelName
		}

		testCases = append(testCases, expectedModel{
			name: expectedModelID,
			expectedModel: models.Model{
				Model: openai.Model{
					ID:      expectedModelID,
					Object:  "model",
					OwnedBy: llmTestScenario.Namespace,
				},
				URL:   mustParseURL(llmTestScenario.URL.String()),
				Ready: llmTestScenario.Ready,
			},
		})
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
