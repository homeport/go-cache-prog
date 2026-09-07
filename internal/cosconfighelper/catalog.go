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
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	globalCatalogBase = "https://globalcatalog.cloud.ibm.com/api/v1"

	// COSPlanLiteFallbackID is the well-known plan ID for COS Lite used as a
	// fallback when the catalog API is unreachable.
	COSPlanLiteFallbackID = "744bfc56-d12c-4866-88d0-352466a1a1c5"
)

// COSCatalogEntry holds the catalog-derived data we need for provisioning.
type COSCatalogEntry struct {
	ID    string
	Plans []COSPlan
}

// FetchCOSCatalogEntry fetches the plans for the COS service entry directly
// using the well-known service ID. Falls back gracefully when the catalog is
// unreachable.
func (h *COSConfigHelper) FetchCOSCatalogEntry() (COSCatalogEntry, error) {
	// The COS service ID is stable and well-known — fetch its children (plans)
	// directly rather than searching, which avoids parsing a nested group structure.
	url := fmt.Sprintf("%s/%s/*", globalCatalogBase, COSServiceResourceID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fallbackCatalogEntry(), nil //nolint:nilerr
	}

	resp, err := h.doRequest(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return fallbackCatalogEntry(), nil
	}
	defer func() { _ = resp.Body.Close() }()

	type childEntry struct {
		Name string `json:"name"`
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	type response struct {
		Resources []childEntry `json:"resources"`
	}

	var result response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fallbackCatalogEntry(), nil //nolint:nilerr
	}

	var plans []COSPlan
	for _, child := range result.Resources {
		if child.Kind == "plan" {
			plans = append(plans, COSPlan{Name: child.Name, ID: child.ID})
		}
	}

	if len(plans) == 0 {
		return fallbackCatalogEntry(), nil
	}

	// The entry ID for deployment lookups is the COS service ID itself.
	return COSCatalogEntry{ID: COSServiceResourceID, Plans: plans}, nil
}

func fallbackCatalogEntry() COSCatalogEntry {
	return COSCatalogEntry{
		ID: COSServiceResourceID,
		Plans: []COSPlan{
			{Name: "Lite", ID: COSPlanLiteFallbackID},
		},
	}
}
