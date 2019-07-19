package node

import (
	corev1 "k8s.io/api/core/v1"
	//apierrors "k8s.io/apimachinery/pkg/api/errors"
	//utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"fmt"
	"k8s.io/kubernetes/pkg/kubectl/drain"
)

func runCordonOrUncordon(r *ReconcileNode, node *corev1.Node, desired bool) error {
	cordonOrUncordon := "cordon"
	if !desired {
		cordonOrUncordon = "un" + cordonOrUncordon
	}

	log.Info(fmt.Sprintf("%s Node: %s", cordonOrUncordon, node.Name))

	c := drain.NewCordonHelper(node)
	if updateRequired := c.UpdateIfRequired(desired); updateRequired {
		err, patchErr := c.PatchOrReplace(r.drainer.Client)
		if patchErr != nil {
			log.Error(err, fmt.Sprintf("Unable to %s Node %s \n", cordonOrUncordon, node.Name))
		}
		if err != nil {
			return err
		}
	}
	return nil
}
