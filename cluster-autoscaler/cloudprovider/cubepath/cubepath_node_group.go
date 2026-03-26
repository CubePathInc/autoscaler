/*
Copyright 2024 The Kubernetes Authors.

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

package cubepath

import (
	"fmt"
	"strconv"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/klog/v2"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

type cubePathNodeGroup struct {
	manager  *cubePathManager
	nodePool *NodePool
}

func (n *cubePathNodeGroup) MaxSize() int {
	return n.nodePool.MaxSize
}

func (n *cubePathNodeGroup) MinSize() int {
	return n.nodePool.MinSize
}

func (n *cubePathNodeGroup) TargetSize() (int, error) {
	return n.nodePool.Count, nil
}

func (n *cubePathNodeGroup) IncreaseSize(delta int) error {
	if delta <= 0 {
		return fmt.Errorf("delta must be positive, have: %d", delta)
	}

	targetSize := n.nodePool.Count + delta
	if targetSize > n.nodePool.MaxSize {
		return fmt.Errorf("size increase too large: current %d desired %d max %d", n.nodePool.Count, targetSize, n.nodePool.MaxSize)
	}

	klog.V(4).Infof("Scaling node pool %s to %d", n.nodePool.UUID, targetSize)
	err := n.manager.client.ScaleNodePool(n.nodePool.UUID, targetSize)
	if err != nil {
		return fmt.Errorf("failed to scale node pool %s: %v", n.nodePool.UUID, err)
	}

	n.nodePool.Count = targetSize
	return nil
}

func (n *cubePathNodeGroup) AtomicIncreaseSize(delta int) error {
	return cloudprovider.ErrNotImplemented
}

func (n *cubePathNodeGroup) DeleteNodes(nodes []*apiv1.Node) error {
	for _, node := range nodes {
		nodeID := nodeIDFromProvider(node)
		if nodeID == "" {
			return fmt.Errorf("cannot determine node ID for %s", node.Name)
		}

		vpsID, err := strconv.Atoi(nodeID)
		if err != nil {
			return fmt.Errorf("invalid VPS ID %q for node %s: %v", nodeID, node.Name, err)
		}

		klog.V(4).Infof("Deleting node %s (VPS %d) from pool %s", node.Name, vpsID, n.nodePool.UUID)
		err = n.manager.client.DeleteNode(n.nodePool.UUID, vpsID)
		if err != nil {
			return fmt.Errorf("failed to delete node %s: %v", nodeID, err)
		}

		n.nodePool.Count--
	}
	return nil
}

func (n *cubePathNodeGroup) ForceDeleteNodes(nodes []*apiv1.Node) error {
	return cloudprovider.ErrNotImplemented
}

func (n *cubePathNodeGroup) DecreaseTargetSize(delta int) error {
	targetSize := n.nodePool.Count + delta
	if targetSize < n.nodePool.MinSize {
		return fmt.Errorf("size decrease too small: current %d desired %d min %d", n.nodePool.Count, targetSize, n.nodePool.MinSize)
	}
	n.nodePool.Count = targetSize
	return nil
}

func (n *cubePathNodeGroup) Id() string {
	return n.nodePool.UUID
}

func (n *cubePathNodeGroup) Debug() string {
	return fmt.Sprintf("node pool %s (min:%d max:%d count:%d)", n.nodePool.UUID, n.nodePool.MinSize, n.nodePool.MaxSize, n.nodePool.Count)
}

func (n *cubePathNodeGroup) Nodes() ([]cloudprovider.Instance, error) {
	instances := make([]cloudprovider.Instance, 0, len(n.nodePool.Nodes))
	for _, node := range n.nodePool.Nodes {
		instances = append(instances, cloudprovider.Instance{
			Id:     providerIDPrefix + node.ID,
			Status: toInstanceStatus(node.Status),
		})
	}
	return instances, nil
}

func (n *cubePathNodeGroup) TemplateNodeInfo() (*schedulerframework.NodeInfo, error) {
	node := apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("cubepath-%s-template", n.nodePool.UUID),
			Labels: map[string]string{
				apiv1.LabelInstanceType: n.nodePool.PlanSlug,
			},
		},
		Status: apiv1.NodeStatus{
			Capacity: apiv1.ResourceList{
				apiv1.ResourcePods:             *resource.NewQuantity(110, resource.DecimalSI),
				apiv1.ResourceCPU:              *resource.NewQuantity(int64(n.nodePool.PlanVCPUs), resource.DecimalSI),
				apiv1.ResourceMemory:           *resource.NewQuantity(int64(n.nodePool.PlanRAM)*1024*1024, resource.DecimalSI),
				apiv1.ResourceEphemeralStorage: *resource.NewQuantity(int64(n.nodePool.PlanDisk)*1024*1024*1024, resource.DecimalSI),
			},
			Conditions: cloudprovider.BuildReadyConditions(),
		},
	}
	node.Status.Allocatable = node.Status.Capacity

	nodeInfo := schedulerframework.NewNodeInfo(cloudprovider.BuildKubeProxy(n.nodePool.UUID))
	nodeInfo.SetNode(&node)

	return nodeInfo, nil
}

func (n *cubePathNodeGroup) Exist() bool {
	return true
}

func (n *cubePathNodeGroup) Create() (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (n *cubePathNodeGroup) Delete() error {
	return cloudprovider.ErrNotImplemented
}

func (n *cubePathNodeGroup) Autoprovisioned() bool {
	return false
}

func (n *cubePathNodeGroup) GetOptions(defaults config.NodeGroupAutoscalingOptions) (*config.NodeGroupAutoscalingOptions, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func toInstanceStatus(status string) *cloudprovider.InstanceStatus {
	st := &cloudprovider.InstanceStatus{}
	switch status {
	case "running":
		st.State = cloudprovider.InstanceRunning
	case "creating", "provisioning":
		st.State = cloudprovider.InstanceCreating
	case "deleting":
		st.State = cloudprovider.InstanceDeleting
	default:
		st.State = cloudprovider.InstanceRunning
	}
	return st
}

func nodeIDFromProvider(node *apiv1.Node) string {
	if node.Spec.ProviderID != "" {
		return strings.TrimPrefix(node.Spec.ProviderID, providerIDPrefix)
	}
	return ""
}
