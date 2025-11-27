package config

import (
	"fmt"

	kserveclientv1alpha1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1alpha1"
	kserveclientv1beta1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1"
)

type K8sClusterConfig struct {
	RestConfig     *rest.Config
	ClientSet      *kubernetes.Clientset
	KServeV1Beta1  kserveclientv1beta1.ServingV1beta1Interface
	KServeV1Alpha1 kserveclientv1alpha1.ServingV1alpha1Interface
	Gateway        gatewayclient.GatewayV1Interface
}

func NewClusterConfig() (*K8sClusterConfig, error) {
	restConfig, err := LoadRestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes ClientSet: %w", err)
	}

	kserveV1Beta1, err := kserveclientv1beta1.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KServe v1beta1 client: %w", err)
	}

	kserveV1Alpha1, err := kserveclientv1alpha1.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KServe v1alpha1 client: %w", err)
	}

	gatewayClient, err := gatewayclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gateway API client: %w", err)
	}

	return &K8sClusterConfig{
		RestConfig:     restConfig,
		ClientSet:      clientset,
		KServeV1Beta1:  kserveV1Beta1,
		KServeV1Alpha1: kserveV1Alpha1,
		Gateway:        gatewayClient,
	}, nil
}

// LoadRestConfig creates a *rest.Config using client-go loading rules.
// Order:
// 1) KUBECONFIG or $HOME/.kube/config (if present and non-default)
// 2) If kubeconfig is empty/default (or IsEmptyConfig), fall back to in-cluster
// Note: if kubeconfig is set but invalid (non-empty error), the error is returned.
func LoadRestConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, kubeconfigErr := kubeConfig.ClientConfig()
	if kubeconfigErr != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", kubeconfigErr)
	}

	return config, nil
}
