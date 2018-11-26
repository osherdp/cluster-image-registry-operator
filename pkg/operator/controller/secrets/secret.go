package secret

import (
	"fmt"

	coreapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	kubeset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/openshift/cluster-image-registry-operator/pkg/client"
	opcontroller "github.com/openshift/cluster-image-registry-operator/pkg/operator/controller"
)

var _ opcontroller.Watcher = &Controller{}

type Controller struct {
	lister corelisters.SecretLister
	synced cache.InformerSynced
}

func (c *Controller) Start(handler opcontroller.Handler, namespace string, stopCh <-chan struct{}) error {
	klog.Info("Starting secrets controller")

	kubeconfig, err := client.GetConfig()
	if err != nil {
		return err
	}

	kubeclient, err := kubeset.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeclient, opcontroller.DefaultResyncDuration, kubeinformers.WithNamespace(namespace))
	informer := kubeInformerFactory.Core().V1().Secrets()

	c.lister = informer.Lister()
	c.synced = informer.Informer().HasSynced

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			handler("add", o)
		},
		UpdateFunc: func(o, n interface{}) {
			newObj := n.(*coreapi.Secret)
			oldObj := o.(*coreapi.Secret)
			if newObj.ResourceVersion == oldObj.ResourceVersion {
				return
			}
			handler("update", n)
		},
		DeleteFunc: func(o interface{}) {
			handler("delete", o)
		},
	})

	kubeInformerFactory.Start(stopCh)

	klog.Info("Waiting for secrets informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.synced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	return nil
}

func (c *Controller) Get(name, namespace string) (runtime.Object, error) {
	return c.lister.Secrets(namespace).Get(name)
}