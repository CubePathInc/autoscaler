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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type NodePool struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	MinSize   int    `json:"min_size"`
	MaxSize   int    `json:"max_size"`
	Count     int    `json:"count"`
	PlanSlug  string `json:"plan_slug"`
	PlanVCPUs int    `json:"plan_vcpus"`
	PlanRAM   int    `json:"plan_ram_mb"`
	PlanDisk  int    `json:"plan_disk_gb"`
	Nodes     []Node `json:"nodes"`
}

type Node struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProviderID string `json:"provider_id"`
	Status     string `json:"status"`
}

type apiClient struct {
	baseURL    string
	token      string
	clusterID  string
	httpClient *http.Client
}

func newAPIClient(baseURL, token, clusterID string) *apiClient {
	return &apiClient{
		baseURL:    baseURL,
		token:      token,
		clusterID:  clusterID,
		httpClient: &http.Client{},
	}
}

func (c *apiClient) do(method, path string, body interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *apiClient) ListNodePools() ([]NodePool, error) {
	path := fmt.Sprintf("/k8s/clusters/%s/node-pools", c.clusterID)
	data, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var pools []NodePool
	if err := json.Unmarshal(data, &pools); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node pools: %v", err)
	}
	return pools, nil
}

func (c *apiClient) ScaleNodePool(poolUUID string, count int) error {
	path := fmt.Sprintf("/k8s/clusters/%s/node-pools/%s/scale", c.clusterID, poolUUID)
	_, err := c.do(http.MethodPost, path, map[string]int{"count": count})
	return err
}

func (c *apiClient) DeleteNode(poolUUID, nodeID string) error {
	path := fmt.Sprintf("/k8s/clusters/%s/node-pools/%s/nodes/%s", c.clusterID, poolUUID, nodeID)
	_, err := c.do(http.MethodDelete, path, nil)
	return err
}
