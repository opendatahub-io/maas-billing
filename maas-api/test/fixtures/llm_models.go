package fixtures

import (
	"time"

	kservev1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
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

type LLMInferenceServiceOption func(*unstructured.Unstructured)

// WithSpecModelName sets .spec.model.name (can be an empty string "" to test fallback logic).
func WithSpecModelName(name string) LLMInferenceServiceOption {
	return func(obj *unstructured.Unstructured) {
		_ = unstructured.SetNestedField(obj.Object, name, "spec", "model", "name")
	}
}

// CreateLLMInferenceService creates a test LLMInferenceService unstructured object
func CreateLLMInferenceService(name, namespace string, url ModelURL, ready bool, opts ...LLMInferenceServiceOption) *unstructured.Unstructured {
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

	// Apply options (e.g., WithSpecModelName)
	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

// LLMTestScenario defines a test scenario for LLM models
type LLMTestScenario struct {
	Name          string
	Namespace     string
	URL           ModelURL
	Ready         bool
	SpecModelName *string
}

// CreateTypedLLMInferenceService creates a test LLMInferenceService typed object
func CreateTypedLLMInferenceService(name, namespace string, url ModelURL, ready bool, specModelName *string) *kservev1alpha1.LLMInferenceService {
	llm := &kservev1alpha1.LLMInferenceService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "serving.kserve.io/v1alpha1",
			Kind:       "LLMInferenceService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour)),
			Generation:        1,
		},
		Spec: kservev1alpha1.LLMInferenceServiceSpec{
			Model: kservev1alpha1.LLMModelSpec{},
		},
		Status: kservev1alpha1.LLMInferenceServiceStatus{
			Status: duckv1.Status{
				ObservedGeneration: 1,
			},
		},
	}

	// Set spec.model.name if provided
	if specModelName != nil {
		llm.Spec.Model.Name = specModelName
	}

	// Parse and set URL
	if urlStr := url.String(); urlStr != "" {
		parsedURL, _ := apis.ParseURL(urlStr)
		llm.Status.URL = parsedURL
		llm.Status.AddressStatus = duckv1.AddressStatus{
			Address: &duckv1.Addressable{URL: parsedURL},
		}
	}

	// Set conditions based on ready state
	if ready {
		llm.Status.Conditions = []apis.Condition{
			{Type: "HTTPRoutesReady", Status: v1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "InferencePoolReady", Status: v1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "MainWorkloadReady", Status: v1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "PresetsCombined", Status: v1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: apis.ConditionReady, Status: v1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "RouterReady", Status: v1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "WorkloadsReady", Status: v1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
		}
	} else {
		llm.Status.Conditions = []apis.Condition{
			{Type: apis.ConditionReady, Status: v1.ConditionFalse, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}, Reason: "ServiceNotReady"},
			{Type: "WorkloadsReady", Status: v1.ConditionFalse, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}, Reason: "NotReady"},
		}
	}

	return llm
}

// CreateLLMInferenceServices creates a set of test LLM objects for testing
func CreateLLMInferenceServices(scenarios ...LLMTestScenario) []runtime.Object {
	var objects []runtime.Object
	for _, scenario := range scenarios {
		obj := CreateTypedLLMInferenceService(
			scenario.Name,
			scenario.Namespace,
			scenario.URL,
			scenario.Ready,
			scenario.SpecModelName,
		)

		objects = append(objects, obj)
	}

	return objects
}
