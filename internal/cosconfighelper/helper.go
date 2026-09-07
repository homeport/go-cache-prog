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
	"net/http"
	"time"
)

// COSConfigHelper is the single handle used by the config-helper command to
// interact with IBM Cloud. It carries the loaded IBM Cloud session config, an
// HTTP client (shared across all API calls), and a verbose flag that controls
// request/response tracing to stderr.
type COSConfigHelper struct {
	// IBMCfg is the IBM Cloud local session loaded from ~/.bluemix/config.json.
	IBMCfg *IBMCloudLocalConfig

	// client is the shared HTTP client for all non-S3 API calls.
	client *http.Client

	// verbose controls HTTP request/response tracing to stderr.
	verbose bool
}

// New returns a COSConfigHelper with a default HTTP client. Call
// ReadIBMCloudConfig separately and assign IBMCfg before making API calls.
func New(verbose bool) *COSConfigHelper {
	return &COSConfigHelper{
		client:  &http.Client{Timeout: 15 * time.Second},
		verbose: verbose,
	}
}
