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
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	"k8s.io/klog/v2"
)

var _ cloudprovider.CloudProvider = (*cubePathCloudProvider)(nil)

type cubePathCloudProvider struct {
	manager         *cubePathManager
	resourceLimiter *cloudprovider.ResourceLimiter
}

func newCubePathCloudProvider(manager *cubePathManager, rl *cloudprovider.ResourceLimiter) *cubePathCloudProvider {
	return &cubePathCloudProvider{
		manager:         manager,
		resourceLimiter: rl,
	}
}

func (p *cubePathCloudProvider) Name() string {
	return cloudprovider.CubePathProviderName
}

func (p *cubePathCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	p.manager.mu.Lock()
	defer p.manager.mu.Unlock()

	groups := make([]cloudprovider.NodeGroup, 0, len(p.manager.nodePools))
	for _, pool := range p.manager.nodePools {
		groups = append(groups, &cubePathNodeGroup{
			manager:  p.manager,
			nodePool: pool,
		})
	}
	return groups
}

func (p *cubePathCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	pool, _, err := p.manager.nodeGroupForNode(node)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, nil
	}
	return &cubePathNodeGroup{
		manager:  p.manager,
		nodePool: pool,
	}, nil
}

func (p *cubePathCloudProvider) HasInstance(node *apiv1.Node) (bool, error) {
	return true, cloudprovider.ErrNotImplemented
}

func (p *cubePathCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

func (p *cubePathCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (p *cubePathCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string, taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (p *cubePathCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return p.resourceLimiter, nil
}

func (p *cubePathCloudProvider) GPULabel() string {
	return ""
}

func (p *cubePathCloudProvider) GetAvailableGPUTypes() map[string]struct{} {
	return nil
}

func (p *cubePathCloudProvider) GetNodeGpuConfig(node *apiv1.Node) *cloudprovider.GpuConfig {
	return nil
}

func (p *cubePathCloudProvider) Cleanup() error {
	return nil
}

func (p *cubePathCloudProvider) Refresh() error {
	return p.manager.Refresh()
}

func BuildCubePath(_ config.AutoscalingOptions, do cloudprovider.NodeGroupDiscoveryOptions, rl *cloudprovider.ResourceLimiter) cloudprovider.CloudProvider {
	manager, err := newManager()
	if err != nil {
		klog.Fatalf("Failed to create CubePath manager: %v", err)
	}

	if err := manager.Refresh(); err != nil {
		klog.Fatalf("Failed initial refresh of CubePath node pools: %v", err)
	}

	return newCubePathCloudProvider(manager, rl)
}
