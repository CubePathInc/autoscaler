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

	"k8s.io/klog/v2"
)

type NodePool struct {
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	Plan         Plan     `json:"plan"`
	DesiredNodes int      `json:"desired_nodes"`
	MinNodes     int      `json:"min_nodes"`
	MaxNodes     int      `json:"max_nodes"`
	AutoScale    bool     `json:"auto_scale"`
	Nodes        []Worker `json:"nodes"`
}

type Plan struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	RAM          int     `json:"ram"`
	CPU          int     `json:"cpu"`
	Storage      int     `json:"storage"`
	PricePerHour float64 `json:"price_per_hour"`
}

type Worker struct {
	VPSID     int    `json:"vps_id"`
	VPSName   string `json:"vps_name"`
	VPSStatus string `json:"vps_status"`
	K8sStatus string `json:"k8s_status"`
	NodeName  string `json:"node_name"`
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

	if resp.StatusCode == 409 {
		klog.V(2).Infof("API returned 409 (busy), will retry next cycle: %s", string(respBody))
		return nil, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *apiClient) ListNodePools() ([]NodePool, error) {
	path := fmt.Sprintf("/kubernetes/%s/node-pools/", c.clusterID)
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

func (c *apiClient) ScaleNodePool(poolUUID string, desiredNodes int) error {
	klog.V(4).Infof("Scaling pool %s to %d nodes", poolUUID, desiredNodes)
	path := fmt.Sprintf("/kubernetes/%s/node-pools/%s", c.clusterID, poolUUID)
	_, err := c.do(http.MethodPatch, path, map[string]int{"desired_nodes": desiredNodes})
	return err
}

func (c *apiClient) DeleteNode(poolUUID string, vpsID int) error {
	path := fmt.Sprintf("/kubernetes/%s/node-pools/%s/nodes/%d", c.clusterID, poolUUID, vpsID)
	_, err := c.do(http.MethodDelete, path, nil)
	return err
}
