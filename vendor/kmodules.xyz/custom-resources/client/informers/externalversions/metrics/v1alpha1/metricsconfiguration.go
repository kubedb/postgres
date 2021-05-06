/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	metricsv1alpha1 "kmodules.xyz/custom-resources/apis/metrics/v1alpha1"
	versioned "kmodules.xyz/custom-resources/client/clientset/versioned"
	internalinterfaces "kmodules.xyz/custom-resources/client/informers/externalversions/internalinterfaces"
	v1alpha1 "kmodules.xyz/custom-resources/client/listers/metrics/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// MetricsConfigurationInformer provides access to a shared informer and lister for
// MetricsConfigurations.
type MetricsConfigurationInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.MetricsConfigurationLister
}

type metricsConfigurationInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewMetricsConfigurationInformer constructs a new informer for MetricsConfiguration type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewMetricsConfigurationInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredMetricsConfigurationInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredMetricsConfigurationInformer constructs a new informer for MetricsConfiguration type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredMetricsConfigurationInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MetricsV1alpha1().MetricsConfigurations().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MetricsV1alpha1().MetricsConfigurations().Watch(context.TODO(), options)
			},
		},
		&metricsv1alpha1.MetricsConfiguration{},
		resyncPeriod,
		indexers,
	)
}

func (f *metricsConfigurationInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredMetricsConfigurationInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *metricsConfigurationInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&metricsv1alpha1.MetricsConfiguration{}, f.defaultInformer)
}

func (f *metricsConfigurationInformer) Lister() v1alpha1.MetricsConfigurationLister {
	return v1alpha1.NewMetricsConfigurationLister(f.Informer().GetIndexer())
}