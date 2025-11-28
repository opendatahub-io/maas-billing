package fixtures

import (
	"time"

	kservev1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
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

// LLMInferenceServiceOption is a functional option for CreateLLMInferenceService.
type LLMInferenceServiceOption func(*kservev1alpha1.LLMInferenceService)

// WithSpecModelName sets the spec.model.name field.
func WithSpecModelName(name string) LLMInferenceServiceOption {
	return func(llm *kservev1alpha1.LLMInferenceService) {
		llm.Spec.Model.Name = &name
	}
}

// WithURL sets the status URL and address.
func WithURL(url ModelURL) LLMInferenceServiceOption {
	return func(llm *kservev1alpha1.LLMInferenceService) {
		if urlStr := url.String(); urlStr != "" {
			parsedURL, err := apis.ParseURL(urlStr)
			if err != nil {
				panic("invalid URL: " + err.Error())
			}
			llm.Status.URL = parsedURL
			llm.Status.AddressStatus = duckv1.AddressStatus{
				Address: &duckv1.Addressable{URL: parsedURL},
			}
		}
	}
}

// WithGatewaySpec sets the router gateway specification.
func WithGatewaySpec(name, namespace string) LLMInferenceServiceOption {
	return func(llm *kservev1alpha1.LLMInferenceService) {
		llm.Spec.Router = &kservev1alpha1.RouterSpec{
			Gateway: &kservev1alpha1.GatewaySpec{
				Refs: []kservev1alpha1.UntypedObjectReference{
					{
						Name:      gwapiv1.ObjectName(name),
						Namespace: gwapiv1.Namespace(namespace),
					},
				},
			},
		}
	}
}

// LLMTestScenario defines a test scenario for LLM models.
type LLMTestScenario struct {
	Name             string
	Namespace        string
	URL              ModelURL
	Ready            bool
	SpecModelName    *string
	GatewayName      string
	GatewayNamespace string
}

// CreateLLMInferenceService creates a test LLMInferenceService object with optional configuration.
func CreateLLMInferenceService(name, namespace string, ready bool, opts ...LLMInferenceServiceOption) *kservev1alpha1.LLMInferenceService {
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

	for _, opt := range opts {
		opt(llm)
	}

	if ready {
		llm.Status.Conditions = []apis.Condition{
			{Type: "HTTPRoutesReady", Status: corev1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "InferencePoolReady", Status: corev1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "MainWorkloadReady", Status: corev1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "PresetsCombined", Status: corev1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: apis.ConditionReady, Status: corev1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "RouterReady", Status: corev1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
			{Type: "WorkloadsReady", Status: corev1.ConditionTrue, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}},
		}
	} else {
		llm.Status.Conditions = []apis.Condition{
			{Type: apis.ConditionReady, Status: corev1.ConditionFalse, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}, Reason: "ServiceNotReady"},
			{Type: "WorkloadsReady", Status: corev1.ConditionFalse, LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(-time.Hour))}, Reason: "NotReady"},
		}
	}

	return llm
}

// CreateLLMInferenceServices creates a set of test LLM objects for testing.
func CreateLLMInferenceServices(scenarios ...LLMTestScenario) []runtime.Object {
	objects := make([]runtime.Object, 0, len(scenarios))
	for _, scenario := range scenarios {
		var opts []LLMInferenceServiceOption

		if scenario.SpecModelName != nil {
			opts = append(opts, WithSpecModelName(*scenario.SpecModelName))
		}

		if scenario.URL != nil {
			opts = append(opts, WithURL(scenario.URL))
		}

		if scenario.GatewayName != "" {
			opts = append(opts, WithGatewaySpec(scenario.GatewayName, scenario.GatewayNamespace))
		}

		obj := CreateLLMInferenceService(scenario.Name, scenario.Namespace, scenario.Ready, opts...)

		objects = append(objects, obj)
	}

	return objects
}
