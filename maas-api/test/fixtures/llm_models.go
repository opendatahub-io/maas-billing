package fixtures

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// CreateLLMInferenceService creates a test LLMInferenceService unstructured object
func CreateLLMInferenceService(name, namespace, url string, ready bool) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.Object = map[string]any{}
	obj.SetAPIVersion("serving.kserve.io/v1alpha1")
	obj.SetKind("LLMInferenceService")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetCreationTimestamp(metav1.NewTime(time.Now().Add(-time.Hour)))
	obj.SetGeneration(1)

	// Set status with URL and conditions
	status := map[string]any{
		"observedGeneration": int64(1),
	}

	if url != "" {
		status["url"] = url
	}

	// Set conditions based on ready state - using actual LLMInferenceService condition types
	conditions := []any{}
	if ready {
		conditions = append(conditions, map[string]any{
			"type":               "HTTPRoutesReady",
			"status":             "True",
			"lastTransitionTime": "2025-09-18T10:57:50Z",
			"severity":           "Info",
		})
		conditions = append(conditions, map[string]any{
			"type":               "InferencePoolReady",
			"status":             "True",
			"lastTransitionTime": "2025-09-18T10:57:50Z",
			"severity":           "Info",
		})
		conditions = append(conditions, map[string]any{
			"type":               "MainWorkloadReady",
			"status":             "True",
			"lastTransitionTime": "2025-09-18T11:04:20Z",
			"severity":           "Info",
		})
		conditions = append(conditions, map[string]any{
			"type":               "PresetsCombined",
			"status":             "True",
			"lastTransitionTime": "2025-09-18T10:57:50Z",
		})
		conditions = append(conditions, map[string]any{
			"type":               "Ready",
			"status":             "True",
			"lastTransitionTime": "2025-09-18T11:04:20Z",
		})
		conditions = append(conditions, map[string]any{
			"type":               "RouterReady",
			"status":             "True",
			"lastTransitionTime": "2025-09-18T10:57:50Z",
		})
		conditions = append(conditions, map[string]any{
			"type":               "WorkloadsReady",
			"status":             "True",
			"lastTransitionTime": "2025-09-18T11:04:20Z",
		})
	} else {
		conditions = append(conditions, map[string]any{
			"type":               "Ready",
			"status":             "False",
			"lastTransitionTime": "2025-09-18T11:04:20Z",
			"reason":             "ServiceNotReady",
		})
		conditions = append(conditions, map[string]any{
			"type":               "WorkloadsReady",
			"status":             "False",
			"lastTransitionTime": "2025-09-18T11:04:20Z",
			"reason":             "NotReady",
		})
	}

	status["conditions"] = conditions
	obj.Object["status"] = status

	return obj
}

// LLMTestScenario defines a test scenario for LLM models
type LLMTestScenario struct {
	Name      string
	Namespace string
	URL       string
	Ready     bool
}

// CreateLLMTestObjects creates a set of test LLM objects for testing
func CreateLLMTestObjects() []runtime.Object {
	scenarios := []LLMTestScenario{
		{
			Name:      "llama-7b",
			Namespace: "model-serving",
			URL:       "http://llama-7b.model-serving.svc.cluster.local/v1",
			Ready:     true,
		},
		{
			Name:      "gpt-3-turbo",
			Namespace: "openai-models",
			URL:       "http://gpt-3-turbo.openai-models.svc.cluster.local/v1",
			Ready:     true,
		},
		{
			Name:      "bert-base",
			Namespace: "nlp-models",
			URL:       "http://bert-base.nlp-models.svc.cluster.local/v1",
			Ready:     false,
		},
		{
			Name:      "model-without-url",
			Namespace: TestNamespace,
			URL:       "",
			Ready:     false,
		},
	}

	var objects []runtime.Object
	for _, scenario := range scenarios {
		obj := CreateLLMInferenceService(scenario.Name, scenario.Namespace, scenario.URL, scenario.Ready)
		objects = append(objects, obj)
	}

	return objects
}
