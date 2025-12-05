package fixtures

import (
	kservelistersv1alpha1 "github.com/kserve/kserve/pkg/client/listers/serving/v1alpha1"
	kservelistersv1beta1 "github.com/kserve/kserve/pkg/client/listers/serving/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	gatewaylisters "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"
)

//nolint:ireturn // test helper
func newIndexer() cache.Indexer {
	return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
}

//nolint:ireturn // test helper
func NewLLMInferenceServiceLister(items ...runtime.Object) kservelistersv1alpha1.LLMInferenceServiceLister {
	indexer := newIndexer()
	for _, item := range items {
		_ = indexer.Add(item)
	}
	return kservelistersv1alpha1.NewLLMInferenceServiceLister(indexer)
}

//nolint:ireturn // test helper
func NewInferenceServiceLister(items ...runtime.Object) kservelistersv1beta1.InferenceServiceLister {
	indexer := newIndexer()
	for _, item := range items {
		_ = indexer.Add(item)
	}
	return kservelistersv1beta1.NewInferenceServiceLister(indexer)
}

//nolint:ireturn // test helper
func NewConfigMapLister(items ...*corev1.ConfigMap) corelisters.ConfigMapLister {
	indexer := newIndexer()
	for _, item := range items {
		_ = indexer.Add(item)
	}
	return corelisters.NewConfigMapLister(indexer)
}

//nolint:ireturn // test helper
func NewHTTPRouteLister(items ...runtime.Object) gatewaylisters.HTTPRouteLister {
	indexer := newIndexer()
	for _, item := range items {
		_ = indexer.Add(item)
	}
	return gatewaylisters.NewHTTPRouteLister(indexer)
}
