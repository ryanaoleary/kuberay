// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	context "context"
	time "time"

	apisrayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	versioned "github.com/ray-project/kuberay/ray-operator/pkg/client/clientset/versioned"
	internalinterfaces "github.com/ray-project/kuberay/ray-operator/pkg/client/informers/externalversions/internalinterfaces"
	rayv1 "github.com/ray-project/kuberay/ray-operator/pkg/client/listers/ray/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// RayClusterInformer provides access to a shared informer and lister for
// RayClusters.
type RayClusterInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() rayv1.RayClusterLister
}

type rayClusterInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewRayClusterInformer constructs a new informer for RayCluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewRayClusterInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredRayClusterInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredRayClusterInformer constructs a new informer for RayCluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredRayClusterInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RayV1().RayClusters(namespace).List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RayV1().RayClusters(namespace).Watch(context.Background(), options)
			},
			ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RayV1().RayClusters(namespace).List(ctx, options)
			},
			WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RayV1().RayClusters(namespace).Watch(ctx, options)
			},
		},
		&apisrayv1.RayCluster{},
		resyncPeriod,
		indexers,
	)
}

func (f *rayClusterInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredRayClusterInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *rayClusterInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&apisrayv1.RayCluster{}, f.defaultInformer)
}

func (f *rayClusterInformer) Lister() rayv1.RayClusterLister {
	return rayv1.NewRayClusterLister(f.Informer().GetIndexer())
}
