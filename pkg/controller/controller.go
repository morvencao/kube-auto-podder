package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	kubelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	autopodderv1 "github.com/morvencao/kube-auto-podder/pkg/apis/autopodder/v1"
	autopodderclientset "github.com/morvencao/kube-auto-podder/pkg/client/clientset/versioned"
	autopodderinformer "github.com/morvencao/kube-auto-podder/pkg/client/informers/externalversions/autopodder/v1"
	autopodderlister "github.com/morvencao/kube-auto-podder/pkg/client/listers/autopodder/v1"
	autopodderscheme "github.com/morvencao/kube-auto-podder/pkg/client/clientset/versioned/scheme"
)

const controllerAgentName = "autopodder-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a AutoPodder is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a AutoPodder fails
	// to sync due to a pod of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a pod already existing
	MessageResourceExists = "Resource %q already exists and is not managed by AutoPodder"
	// MessageResourceSynced is the message used for an Event fired when a AutoPodder
	// is synced successfully
	MessageResourceSynced = "AutoPodder synced successfully"
)

type ControllerArgs struct {
	masterURL		string
	kubeConfigFile	string
}

// Controller is the controller implementation for AutoPodder resources
type Controller struct {
	// kubeclient is a standard kubernetes clientset
	kubeclient			kubernetes.Interface
	// autopodderclient is a clientset for AutoPodder API
	autopodderclient	autopodderclientset.Interface

	podLister			kubelister.PodLister
	podSynced			cache.InformerSynced
	autopodderLister	autopodderlister.AutoPodderLister
	autopodderSynced	cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue			workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder			record.EventRecorder
}

// NewController creates a new instance of controller with parameters
func NewController(
	kubeclientset kubernetes.Interface,
	apclientset autopodderclientset.Interface,
	podInformer coreinformer.PodInformer,
	autopodderInformer autopodderinformer.AutoPodderInformer) (*Controller, error) {
	
	// create event broadcaster
	// add autopodder-controller types to the default kubernetes scheme so events can be logged for 
	// autopodder-controller types.
	utilruntime.Must(autopodderscheme.AddToScheme(scheme.Scheme))
	glog.Info("Creating event broadcaster...")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	
	controller := &Controller{
		kubeclient:			kubeclientset,
		autopodderclient:	apclientset,
		podLister:			podInformer.Lister(),
		podSynced:			podInformer.Informer().HasSynced,
		autopodderLister:	autopodderInformer.Lister(),
		autopodderSynced:	autopodderInformer.Informer().HasSynced,
		workqueue:			workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AutoPodder"),
		recorder:			recorder,
	}
	
	glog.Info("Setting up event handlers...")
	// setup an event handler for AutoPodder update
	autopodderInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:	controller.enqueueAutoPodder,
		UpdateFunc:	func(old, new interface{}) {
				controller.enqueueAutoPodder(new)
		},
	})
	
	// Set up an event handler for when Pod resources change. This
	// handler will lookup the owner of the given Pod, and if it is
	// owned by a AutoPodder resource will enqueue that AutoPodder 
	// resource for processing. This way, we don't need to implement 
	// custom logic for handling Pod resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:	controller.handleObject,
		UpdateFunc:	func(old, new interface{}) {
				newPod := new.(*corev1.Pod)
				oldPod := old.(*corev1.Pod)
				if newPod.ResourceVersion == oldPod.ResourceVersion {
					// Periodic resync will send update events for all known pods.
					// Two different versions of the same pod will always have different RVs.
					return
				}
				controller.handleObject(new)
		},
		DeleteFunc:	controller.handleObject,
	})
	
	return controller, nil
} 

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (controller *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	
	// handle panic with logging and exiting
	defer utilruntime.HandleCrash()
	
	// ignore new items in the queue but when all goroutines 
	// have completed existing items, then shutdown
	defer controller.workqueue.ShutDown()
	
	glog.Info("starting autopodder controller...")
	
	// wait for the caches to be synced before starting workers
	glog.Info("waiting for informer caches to sync...")
	if ok := cache.WaitForCacheSync(stopCh, controller.podSynced, controller.autopodderSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	
	glog.Info("starting workers...")
	// launch threadiness number of workers to process AutoPodder resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(controller.runWorker, time.Second, stopCh)
	}
	
	glog.Info("started workers")
	<- stopCh
	glog.Info("shutting down workers...")
	
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (controller *Controller) runWorker() {
	for controller.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (controller *Controller) processNextWorkItem() bool {
	obj, shutdown := controller.workqueue.Get()
	if shutdown {
		return false
	}

	// wrap this block in a func so we can defer 'c.workqueue.Done'.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer controller.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			controller.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// AudoPodder resource to be synced.
		if err := controller.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			controller.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		controller.workqueue.Forget(obj)
		glog.Infof("successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the 'status' block of the AutoPodder 
// resource with the current status of the resource.
func (controller *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the AutoPodder resource with this namespace/name
	autopodder, err := controller.autopodderLister.AutoPodders(namespace).Get(name)
	if err != nil {
		// The AutoPodder resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("autopodder '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}
	
	podName := autopodder.Spec.PodName
	image := autopodder.Spec.Image
	tag := autopodder.Spec.Tag
	if podName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: pod name must be specified", key))
		return nil
	}
	
	if image == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: image must be specified", key))
		return nil
	}
	
	if tag == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: tag must be specified", key))
		return nil
	}
	
	// Get the pod with the name specified in AutoPodder.spec
	pod, err := controller.podLister.Pods(autopodder.Namespace).Get(podName)
	// create the resource if it doesn't exist
	if errors.IsNotFound(err) {
		pod, err = controller.kubeclient.CoreV1().Pods(autopodder.Namespace).Create(newPod(autopodder))
	}
	
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Pod is not controlled by this AutoPodder resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(pod, autopodder) {
		msg := fmt.Sprintf(MessageResourceExists, pod.Name)
		controller.recorder.Event(autopodder, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}
	
	// Finally, we update the status block of the AutoPodder resource to reflect the
	// current state of the world
	err = controller.updateAutoPodderStatus(autopodder, pod)
	if err != nil {
		return err
	}
	
	controller.recorder.Event(autopodder, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil	
}

func (controller *Controller) updateAutoPodderStatus(autopodder *autopodderv1.AutoPodder, pod *corev1.Pod) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	autopodderCopy := autopodder.DeepCopy()
	autopodderCopy.Status.Phase = pod.Status.Phase
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the AutoPodder resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := controller.autopodderclient.MorvenV1().AutoPodders(autopodder.Namespace).Update(autopodderCopy)
	return err
}

// enqueueAutoPodder takes a AutoPodder resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be passed 
// resources of any type other than AutoPodder.
func (controller *Controller) enqueueAutoPodder(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	controller.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the AutoPodder resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that AutoPodder resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (controller *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}
	glog.Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a AutoPodder, we should not do anything more with it.
		if ownerRef.Kind != "AutoPodder" {
			return
		}
		
		autopodder, err := controller.autopodderLister.AutoPodders(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.Infof("ignoring orphaned object '%s' of autopodder '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		controller.enqueueAutoPodder(autopodder)
		return
	}
}

// newPod creates a new Pod for a AutoPodder resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the AutoPodder resource that 'owns' it.
func newPod(autopodder *autopodderv1.AutoPodder) *corev1.Pod {
	labels := map[string]string{
		"app":        autopodder.Spec.PodName,
		"controller": autopodder.Name,
	}
	return &corev1.Pod{
				ObjectMeta:	metav1.ObjectMeta{
					Name:	autopodder.Spec.PodName,
					Namespace:	autopodder.Namespace,
					Labels:	labels,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(autopodder, schema.GroupVersionKind{
							Group:		autopodderv1.SchemeGroupVersion.Group,
							Version:	autopodderv1.SchemeGroupVersion.Version,
							Kind:		"AutoPodder",
						}),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  autopodder.Spec.PodName,
							Image: autopodder.Spec.Image + ":" + autopodder.Spec.Tag,
						},
					},
				},
			}
}
