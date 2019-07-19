package node

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	kubernetes "k8s.io/client-go/kubernetes"
)

var NodeUnschedulableTaint = &corev1.Taint{
	Key:    "node.kubernetes.io/unschedulable",
	Effect: corev1.TaintEffectNoSchedule,
}

// AddTaint Add taints to nodes
func AddTaint(clientset kubernetes.Interface, node *corev1.Node) error {

	taintStr := ""
	patch := ""
	client := clientset.Core().Nodes()

	oldTaints, err := json.Marshal(node.Spec.Taints)
	if err != nil {
		return err
	}

	newTaints, err := json.Marshal([]corev1.Taint{*NodeUnschedulableTaint})
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Applying %s taint %s on Node: %s", NodeUnschedulableTaint.Key, taintStr, node.Name))
	patch = fmt.Sprintf(`{ "op": "add", "path": "/spec/taints", "value": %s }`, string(newTaints))

	test := fmt.Sprintf(`{ "op": "test", "path": "/spec/taints", "value": %s }`, string(oldTaints))
	log.Info(fmt.Sprintf("Patching taints on Node: %s", node.Name))
	_, err := client.Patch(node.Name, types.JSONPatchType, []byte(fmt.Sprintf("[ %s, %s ]", test, patch)))
	if err != nil {
		return fmt.Errorf("patching node taints failed: %v", err)
	}

	return nil
}

// CheckTaints Check if the unschedulable taint exists
func CheckTaints(clientset kubernetes.Interface, node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node.kubernetes.io/unschedulable" {
			return true
		}
	}
	return false
}
