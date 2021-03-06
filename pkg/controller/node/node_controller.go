package node

import (

  "context"
  "strconv"

  // "github.com/acidlemon/go-dumper"

  "time"
  "os"

	corev1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	// "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
  "sigs.k8s.io/controller-runtime/pkg/predicate"
  "sigs.k8s.io/controller-runtime/pkg/event"
)

var log = logf.Log.WithName("controller_node")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Node Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNode{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("node-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

  desiredAge, err := time.ParseDuration(os.Getenv("NODE_AGE"))
  log.WithValues("Node.DesiredAge", os.Getenv("NODE_AGE")).V(1).Info("Node Age defined")

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

var _ reconcile.Reconciler = &ReconcileNode{}

// ReconcileNode reconciles a Node object
type ReconcileNode struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Node object and makes changes based on the state read
// and what is in the Node.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNode) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Node.Name", request.Name)
	reqLogger.Info("Reconciling Node")

  // define an error var so we don't have to keep redeclaring it
  var err error

  // Get all the nodes
  nodes := &corev1.NodeList{}
  opts := &client.ListOptions{}
  err = r.client.List(context.TODO(), opts, nodes)
  if err != nil {
    reqLogger.Info("Failed to list nodes")
    return reconcile.Result{}, err
  }


  // Because the field selector doesn't currently seem to work for some reason
  // Loop through all the nodes and detect if there's an unscheduable flag
  // FIXME: also check the taints on the nodes
  c := 0
  for _, node := range nodes.Items {
    if node.Spec.Unschedulable == true {
      c++
    }
  }

  // Get the length of the nodes slice
  n := len(nodes.Items)

  // calculate the % of nodes currently cordoned and set the threshold
  percent := c / n  * 100
  possiblePercent := c+1 / n * 100
  var threshold int = 50

  cordonLogger := reqLogger.WithValues("CordonedPercent", strconv.Itoa(percent), "Threshold", strconv.Itoa(threshold), "CordonedNodes", strconv.Itoa(c), "TotalNodes", strconv.Itoa(n))
  // if the % is gt than threshold, bail
  if percent > threshold {
    cordonLogger.Info("Refusing to cordon node because threshold met")
    return reconcile.Result{}, err
  } else if possiblePercent > threshold {
    cordonLogger.Info("Refusing to cordon node because threshold would be met once cordoned")
    return reconcile.Result{}, err
  }

  // Get the node from the request
  node := &corev1.Node{}
  err = r.client.Get(context.TODO(), request.NamespacedName, node)

  if err != nil {
    reqLogger.Info("Could not find node")
    return reconcile.Result{}, err
  }

  if node.Spec.Unschedulable {
    reqLogger.Info("Node already cordoned")
    return reconcile.Result{}, err
  }

  node.Spec.Unschedulable = true
  err = r.client.Update(context.TODO(), node)

  if err != nil {
    reqLogger.Info("Could not update node")
    return reconcile.Result{}, err
  }


	reqLogger.Info("Node cordoned")
	return reconcile.Result{}, nil
}

