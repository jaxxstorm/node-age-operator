package node

import (
	"context"
	//"strconv"

	// "github.com/acidlemon/go-dumper"

	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	// "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/drain"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_node")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Node Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return add(mgr, r)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	r := &ReconcileNode{client: mgr.GetClient(), scheme: mgr.GetScheme()}
	err := initDrainer(r, mgr.GetConfig())
	return r, err
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("node-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	desiredAge, err := time.ParseDuration(os.Getenv("NODE_AGE"))
	log.WithValues("Node.DesiredAge", desiredAge).V(1).Info("Node Age defined")

	if err != nil {
		log.Error(err, "Must set NODE_AGE environment variable")
		return err
	}

	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false // We're not going to want to reconcile new nodes
		},
		UpdateFunc: func(e event.UpdateEvent) bool {

			// Get current node age

			creationTime := e.MetaNew.GetCreationTimestamp()
			age := time.Since(creationTime.Time)
			log.WithValues("Node.Name", e.MetaNew.GetName(), "Node.DesiredAge", desiredAge.String(), "Node.CurrentAge", age.String()).V(1).Info("Determining if Node should be reconciled")

			if desiredAge > age {
				return false
			}
			return true

		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	}

	// Watch for changes to primary resource Node
	err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{}, pred)
	if err != nil {
		return err
	}

	return nil
}

func initDrainer(r *ReconcileNode, config *rest.Config) error {
	r.drainer = &drain.Helper{}
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	r.drainer.Client = cs
	r.drainer.DryRun = false

	return nil
}

var _ reconcile.Reconciler = &ReconcileNode{}

// ReconcileNode reconciles a Node object
type ReconcileNode struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client  client.Client
	scheme  *runtime.Scheme
	drainer *drain.Helper
}

// Reconcile reads that state of the cluster for a Node object and makes changes based on the state read
// and what is in the Node.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNode) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Node.Name", request.Name)
	reqLogger.Info("Reconciling Node")

	var err error

	node := &corev1.Node{}
	err = r.client.Get(context.TODO(), request.NamespacedName, node)

	if err != nil {
		reqLogger.Info("Could not find node - it was probably terminated")
		return reconcile.Result{}, err
	}

	if node.Spec.Unschedulable {
		reqLogger.Info("Node is already unscheduleable")
		return reconcile.Result{RequeueAfter: time.Second * 100}, nil
	}

	tainted := CheckTaints(r.drainer.Client, node)
	if tainted {
		reqLogger.Info("Node already has unschedulable taint")
		return reconcile.Result{RequeueAfter: time.Second * 100}, nil
	}
	err = AddTaint(r.drainer.Client, node)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Make cordon behaviour configurable
	// This is work around the issue with removing nodes from a service
	_, cordon := os.LookupEnv("CORDON")
	if cordon {
		if err = runCordonOrUncordon(r, node, true); err != nil {
			return reconcile.Result{}, err
		}
	}

	stop := make(chan struct{})
	defer close(stop)

	return reconcile.Result{RequeueAfter: time.Second * 30}, nil
	//return reconcile.Result{}, nil

}
