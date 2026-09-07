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
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
)

// verboseTransport wraps an http.RoundTripper and, when verbose is set, dumps
// each request (Authorization header redacted) and response to stderr.
type verboseTransport struct {
	transport http.RoundTripper
	verbose   bool
}

// RoundTrip implements http.RoundTripper. It logs the request and response
// to stderr when verbose is enabled, then delegates to the inner transport.
func (vt *verboseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if vt.verbose {
		// Read and restore req.Body before cloning: req.Clone shares the same
		// underlying io.Reader, so DumpRequestOut draining the clone also drains
		// the original. We save the bytes here and put fresh readers on both.
		var bodyBytes []byte
		if req.Body != nil && req.Body != http.NoBody {
			bodyBytes, _ = io.ReadAll(req.Body)
			_ = req.Body.Close()
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		clone := req.Clone(req.Context())
		if len(bodyBytes) > 0 {
			clone.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		if clone.Header.Get("Authorization") != "" {
			clone.Header.Set("Authorization", "[REDACTED]")
		}
		dump, _ := httputil.DumpRequestOut(clone, true)
		_, _ = fmt.Fprintf(os.Stderr, ">>> %s\n%s\n", req.URL, dump)
	}
	resp, err := vt.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if vt.verbose {
		dump, _ := httputil.DumpResponse(resp, true)
		_, _ = fmt.Fprintf(os.Stderr, "<<< %d %s\n%s\n", resp.StatusCode, req.URL, dump)
	}
	return resp, nil
}

// doRequest executes req via the helper's HTTP client with verbose
// request/response dumps to stderr when verbose is enabled. The caller must
// close resp.Body.
func (h *COSConfigHelper) doRequest(req *http.Request) (*http.Response, error) {
	inner := h.client.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}
	tmp := *h.client
	tmp.Transport = &verboseTransport{transport: inner, verbose: h.verbose}
	return tmp.Do(req)
}
