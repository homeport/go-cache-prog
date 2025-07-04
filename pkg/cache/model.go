// Copyright © 2025 The Homeport Team
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

package cache

import (
	"io"
	"time"
)

type Provider interface {
	KnownCommands() []string

	Get(actionId string) (objectId string, diskpath string, err error)
	Put(actionId string, objectId string, body io.Reader) (diskpath string, err error)
	Close() error
}

// TBD https://pkg.go.dev/cmd/go/internal/cache#ProgRequest
type progRequest struct {
	ID      int64
	Command string

	ActionID []byte    `json:",omitempty"`
	OutputID []byte    `json:"OutputID,omitempty"`
	Body     io.Reader `json:"-"`
	BodySize int64     `json:",omitempty"`
}

// TBD https://pkg.go.dev/cmd/go/internal/cache#ProgResponse
type progResponse struct {
	ID  int64
	Err string `json:",omitempty"`

	KnownCommands []string `json:",omitempty"`

	Miss     bool       `json:",omitempty"`
	OutputID []byte     `json:",omitempty"`
	Size     int64      `json:",omitempty"`
	Time     *time.Time `json:",omitempty"`

	DiskPath string `json:",omitempty"`
}
