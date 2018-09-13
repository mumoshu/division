package cmd

import (
	// Kube client doesn't support all auth providers by default.
	// this ensures we include all backends supported by the client.
	"k8s.io/client-go/kubernetes"
	// auth is a side-effect import for Client-Go
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"flag"
)

func init() {
	flag.Parse()
}

// kubeClient returns a Kubernetes clientset.
func kubeClient() (*kubernetes.Clientset, error) {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
