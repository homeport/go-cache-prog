// Copyright © 2026 The Homeport Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cosconfighelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// COSServiceResourceID is the well-known IBM Cloud resource type ID for COS.
	COSServiceResourceID = "dff97f5c-bc5e-4455-b470-411c3edbe49c"

	resourceControllerBase = "https://resource-controller.cloud.ibm.com"
)

// COSInstance is a minimal representation of an IBM Cloud COS service instance.
type COSInstance struct {
	Name string
	GUID string
	CRN  string
}

// ResourceGroup is a minimal representation of an IBM Cloud resource group.
type ResourceGroup struct {
	Name string
	ID   string
}

// COSPlan is a COS service plan (e.g. Lite, Standard).
type COSPlan struct {
	// Name is the display name, e.g. "Lite".
	Name string
	// ID is the resource plan UUID.
	ID string
}

// HMACCredentials holds the HMAC key pair extracted from a resource key.
type HMACCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

// ResourceKey represents a COS service credential that carries HMAC keys.
type ResourceKey struct {
	Name        string
	GUID        string
	Credentials HMACCredentials
}

// ListCOSInstances returns all COS service instances visible to the IAM token.
func (h *COSConfigHelper) ListCOSInstances() ([]COSInstance, error) {
	iamToken := h.IBMCfg.IAMToken

	type resourceInstance struct {
		Name string `json:"name"`
		GUID string `json:"guid"`
		CRN  string `json:"crn"`
	}
	type response struct {
		Resources []resourceInstance `json:"resources"`
	}

	url := resourceControllerBase + "/v2/resource_instances?resource_id=" + COSServiceResourceID + "&limit=100"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", iamToken)

	resp, err := h.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("resource controller request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("resource controller returned %d: %s", resp.StatusCode, body)
	}

	var page response
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("cannot decode resource instances: %w", err)
	}

	var instances []COSInstance
	for _, r := range page.Resources {
		instances = append(instances, COSInstance(r))
	}

	return instances, nil
}

// ListResourceGroups returns the resource groups for the given account.
func (h *COSConfigHelper) ListResourceGroups() ([]ResourceGroup, error) {
	iamToken := h.IBMCfg.IAMToken
	accountGUID := h.IBMCfg.Account.GUID

	type item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type response struct {
		Resources []item `json:"resources"`
	}

	url := fmt.Sprintf("%s/v2/resource_groups?account_id=%s", resourceControllerBase, accountGUID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", iamToken)

	resp, err := h.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("resource groups request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("resource controller returned %d: %s", resp.StatusCode, body)
	}

	var result response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("cannot decode resource groups: %w", err)
	}

	groups := make([]ResourceGroup, len(result.Resources))
	for i, r := range result.Resources {
		groups[i] = ResourceGroup{Name: r.Name, ID: r.ID}
	}
	return groups, nil
}

// CreateCOSInstance creates a new COS service instance. For COS the provisioning
// is synchronous so the 201 response already carries state "active"; the function
// falls back to polling (up to 90s) only when that is not the case.
func (h *COSConfigHelper) CreateCOSInstance(name, resourceGroupID, planID string, w io.Writer) (COSInstance, error) {
	iamToken := h.IBMCfg.IAMToken

	type createRequest struct {
		Name           string `json:"name"`
		Target         string `json:"target"`
		ResourceGroup  string `json:"resource_group"`
		ResourcePlanID string `json:"resource_plan_id"`
	}

	// COS is a global service; the provisioning target is always "global".
	// Note: the Resource Controller API uses "resource_group" (not "resource_group_id").
	body, err := json.Marshal(createRequest{
		Name:           name,
		Target:         "global",
		ResourceGroup:  resourceGroupID,
		ResourcePlanID: planID,
	})
	if err != nil {
		return COSInstance{}, err
	}

	req, err := http.NewRequest(http.MethodPost, resourceControllerBase+"/v2/resource_instances", bytes.NewReader(body))
	if err != nil {
		return COSInstance{}, err
	}
	req.Header.Set("Authorization", iamToken)
	req.Header.Set("Content-Type", "application/json")

	// Use a longer timeout for instance creation.
	tmp := *h.client
	tmp.Timeout = 20 * time.Second
	inner := tmp.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}
	tmp.Transport = &verboseTransport{transport: inner, verbose: h.verbose}
	resp, err := tmp.Do(req)
	if err != nil {
		return COSInstance{}, fmt.Errorf("create instance request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return COSInstance{}, fmt.Errorf("resource controller returned %d: %s", resp.StatusCode, respBody)
	}

	var created struct {
		Name          string `json:"name"`
		GUID          string `json:"guid"`
		CRN           string `json:"crn"`
		ID            string `json:"id"`
		State         string `json:"state"`
		LastOperation struct {
			Async bool `json:"async"`
		} `json:"last_operation"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return COSInstance{}, fmt.Errorf("cannot decode create response: %w", err)
	}

	inst := COSInstance{Name: created.Name, GUID: created.GUID, CRN: created.CRN}

	// COS provisioning is synchronous — the instance is active immediately.
	if created.State == "active" || !created.LastOperation.Async {
		return inst, nil
	}

	// Async path: poll until active or 90-second timeout.
	_, _ = fmt.Fprintln(w, "Waiting for instance to become active ...")
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)

		pollReq, err := http.NewRequest(http.MethodGet, resourceControllerBase+"/v2/resource_instances/"+created.ID, nil)
		if err != nil {
			continue
		}
		pollReq.Header.Set("Authorization", iamToken)

		pollResp, err := h.doRequest(pollReq)
		if err != nil {
			continue
		}

		var pollState struct {
			State string `json:"state"`
		}
		_ = json.NewDecoder(pollResp.Body).Decode(&pollState)
		_ = pollResp.Body.Close()

		if pollState.State == "active" {
			return inst, nil
		}
		_, _ = fmt.Fprint(w, ".")
	}

	return COSInstance{}, fmt.Errorf("instance %q did not become active within 90 seconds", name)
}

// ListResourceKeys returns the HMAC resource keys for a given COS instance.
// instanceCRN must be the full CRN of the instance (e.g. crn:v1:bluemix:…::<guid>::).
func (h *COSConfigHelper) ListResourceKeys(instanceCRN string) ([]ResourceKey, error) {
	iamToken := h.IBMCfg.IAMToken

	type credentialsPayload struct {
		CosHMACKeys struct {
			AccessKeyID     string `json:"access_key_id"`
			SecretAccessKey string `json:"secret_access_key"`
		} `json:"cos_hmac_keys"`
	}
	type resourceKeyItem struct {
		Name        string             `json:"name"`
		GUID        string             `json:"guid"`
		SourceCRN   string             `json:"source_crn"`
		Credentials credentialsPayload `json:"credentials"`
	}
	type response struct {
		Resources []resourceKeyItem `json:"resources"`
	}

	url := fmt.Sprintf("%s/v2/resource_keys?source_crn=%s&limit=100",
		resourceControllerBase,
		strings.NewReplacer(":", "%3A", "/", "%2F").Replace(instanceCRN))

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", iamToken)

	resp, err := h.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("resource keys request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("resource controller returned %d: %s", resp.StatusCode, body)
	}

	var result response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("cannot decode resource keys: %w", err)
	}

	var keys []ResourceKey
	for _, item := range result.Resources {
		if !strings.Contains(item.SourceCRN, instanceCRN) {
			continue
		}
		if item.Credentials.CosHMACKeys.AccessKeyID == "" {
			continue
		}
		keys = append(keys, ResourceKey{
			Name: item.Name,
			GUID: item.GUID,
			Credentials: HMACCredentials{
				AccessKeyID:     item.Credentials.CosHMACKeys.AccessKeyID,
				SecretAccessKey: item.Credentials.CosHMACKeys.SecretAccessKey,
			},
		})
	}
	return keys, nil
}

// CreateResourceKey creates a new HMAC service credential for the given COS instance.
// instanceCRN must be the full CRN of the instance.
func (h *COSConfigHelper) CreateResourceKey(instanceCRN, keyName string) (ResourceKey, error) {
	iamToken := h.IBMCfg.IAMToken

	type createRequest struct {
		Name       string         `json:"name"`
		Source     string         `json:"source"`
		Parameters map[string]any `json:"parameters"`
	}
	body, err := json.Marshal(createRequest{
		Name:       keyName,
		Source:     instanceCRN,
		Parameters: map[string]any{"HMAC": true},
	})
	if err != nil {
		return ResourceKey{}, err
	}

	req, err := http.NewRequest(http.MethodPost, resourceControllerBase+"/v2/resource_keys", bytes.NewReader(body))
	if err != nil {
		return ResourceKey{}, err
	}
	req.Header.Set("Authorization", iamToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.doRequest(req)
	if err != nil {
		return ResourceKey{}, fmt.Errorf("create resource key request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return ResourceKey{}, fmt.Errorf("resource controller returned %d: %s", resp.StatusCode, respBody)
	}

	var item struct {
		Name        string `json:"name"`
		GUID        string `json:"guid"`
		Credentials struct {
			CosHMACKeys struct {
				AccessKeyID     string `json:"access_key_id"`
				SecretAccessKey string `json:"secret_access_key"`
			} `json:"cos_hmac_keys"`
		} `json:"credentials"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return ResourceKey{}, fmt.Errorf("cannot decode new resource key: %w", err)
	}

	return ResourceKey{
		Name: item.Name,
		GUID: item.GUID,
		Credentials: HMACCredentials{
			AccessKeyID:     item.Credentials.CosHMACKeys.AccessKeyID,
			SecretAccessKey: item.Credentials.CosHMACKeys.SecretAccessKey,
		},
	}, nil
}
