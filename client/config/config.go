package config

import (
	_env "github.com/appscode/go/env"
	rest "k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
)

func GetKubeConfig(masterUrl, kubeconfigPath string) (config *rest.Config, err error) {
	debugEnabled := _env.FromHost().DebugEnabled()
	if !debugEnabled {
		config, err = clientcmd.BuildConfigFromFlags(masterUrl, kubeconfigPath)
	} else {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.DefaultClientConfig = &clientcmd.DefaultClientConfig
		overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	}
	return
}
