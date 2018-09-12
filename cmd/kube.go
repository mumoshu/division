package cmd

import (
	// Kube client doesn't support all auth providers by default.
	// this ensures we include all backends supported by the client.
	"k8s.io/client-go/kubernetes"
	// auth is a side-effect import for Client-Go
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	globalKubeConfig  string
	globalKubeContext string
)

// kubeClient returns a Kubernetes clientset.
func kubeClient() (*kubernetes.Clientset, error) {
	cfg, err := getKubeConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

// getKubeConfig returns a Kubernetes client config.
func getKubeConfig() (*rest.Config, error) {
	//rules := clientcmd.NewDefaultClientConfigLoadingRules()
	//rules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	//rules.ExplicitPath = globalKubeConfig
	//
	//overrides := &clientcmd.ConfigOverrides{
	//	ClusterDefaults: clientcmd.ClusterDefaults,
	//	CurrentContext:  globalKubeContext,
	//}
	//
	//return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	return clientcmd.BuildConfigFromFlags("", "/Users/kuoka-yusuke/.kube/config")
}
