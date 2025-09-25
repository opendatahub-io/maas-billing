package fixtures

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type ModelURL interface {
	AddTo(obj *unstructured.Unstructured)
	String() string
}

var _ ModelURL = PublicURL("")

type PublicURL string

func (p PublicURL) String() string {
	return string(p)
}

func (p PublicURL) AddTo(obj *unstructured.Unstructured) {
	_ = unstructured.SetNestedField(obj.Object, p.String(), "status", "url")
	AddressEntry(p.String()).AddTo(obj)
}

var _ ModelURL = AddressEntry("")

type AddressEntry string

func (i AddressEntry) String() string {
	return string(i)
}

func (i AddressEntry) AddTo(obj *unstructured.Unstructured) {
	_ = unstructured.SetNestedSlice(obj.Object, []any{
		map[string]any{
			"url": i.String(),
		},
	}, "status", "addresses")
}

// CreateLLMInferenceService creates a test LLMInferenceService unstructured object
func CreateLLMInferenceService(name, namespace string, url ModelURL, ready bool) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.Object = map[string]any{}
	obj.SetAPIVersion("serving.kserve.io/v1alpha1")
	obj.SetKind("LLMInferenceService")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetCreationTimestamp(metav1.NewTime(time.Now().Add(-time.Hour)))
	obj.SetGeneration(1)

	_ = unstructured.SetNestedField(obj.Object, "1", "status", "observedGeneration")
	url.AddTo(obj)

	// Set conditions based on ready state - using actual LLMInferenceService condition types
	var conditions []any
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

	_ = unstructured.SetNestedSlice(obj.Object, conditions, "status", "conditions")

	return obj
}

// LLMTestScenario defines a test scenario for LLM models
type LLMTestScenario struct {
	Name      string
	Namespace string
	URL       ModelURL
	Ready     bool
}

// CreateLLMInferenceServices creates a set of test LLM objects for testing
func CreateLLMInferenceServices(scenarios ...LLMTestScenario) []runtime.Object {
	var objects []runtime.Object
	for _, scenario := range scenarios {
		obj := CreateLLMInferenceService(scenario.Name, scenario.Namespace, scenario.URL, scenario.Ready)
		objects = append(objects, obj)
	}

	return objects
}
