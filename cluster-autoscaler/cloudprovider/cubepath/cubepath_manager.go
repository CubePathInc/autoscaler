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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	providerIDPrefix = "cubepath://"
	defaultAPIURL    = "https://api.cubepath.com"
)

type cubePathManager struct {
	client    *apiClient
	nodePools []*NodePool
	mu        sync.Mutex
}

func newManager() (*cubePathManager, error) {
	token := os.Getenv("CUBEPATH_API_TOKEN")
	if token == "" {
		return nil, errors.New("CUBEPATH_API_TOKEN is not set")
	}

	clusterUUID := os.Getenv("CUBEPATH_CLUSTER_UUID")
	if clusterUUID == "" {
		return nil, errors.New("CUBEPATH_CLUSTER_UUID is not set")
	}

	apiURL := os.Getenv("CUBEPATH_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	client := newAPIClient(apiURL, token, clusterUUID)

	return &cubePathManager{
		client: client,
	}, nil
}

func (m *cubePathManager) Refresh() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pools, err := m.client.ListNodePools()
	if err != nil {
		return fmt.Errorf("failed to list node pools: %v", err)
	}

	m.nodePools = make([]*NodePool, 0, len(pools))
	for i := range pools {
		if !pools[i].AutoScale {
			klog.V(4).Infof("Skipping node pool %s (%s): auto_scale disabled", pools[i].UUID, pools[i].Name)
			continue
		}
		m.nodePools = append(m.nodePools, &pools[i])
	}

	klog.V(4).Infof("Refreshed %d autoscalable node pools from CubePath API", len(m.nodePools))
	return nil
}

func (m *cubePathManager) nodeGroupForNode(node *apiv1.Node) (*NodePool, *Worker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pool := range m.nodePools {
		for i, w := range pool.Nodes {
			if node.Spec.ProviderID != "" {
				nodeID := strings.TrimPrefix(node.Spec.ProviderID, providerIDPrefix)
				vpsIDStr := strconv.Itoa(w.VPSID)
				if vpsIDStr == nodeID {
					return pool, &pool.Nodes[i], nil
				}
			}
			if w.NodeName == node.Name {
				return pool, &pool.Nodes[i], nil
			}
		}
	}

	return nil, nil, nil
}
