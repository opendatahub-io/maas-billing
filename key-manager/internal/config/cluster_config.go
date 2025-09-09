package config

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sClusterConfig struct {
	RestConfig *rest.Config
	ClientSet  *kubernetes.Clientset
	DynClient  *dynamic.DynamicClient
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

	k8sClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &K8sClusterConfig{
		RestConfig: restConfig,
		ClientSet:  clientset,
		DynClient:  k8sClient,
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
