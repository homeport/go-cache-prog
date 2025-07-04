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

package local

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/homeport/go-cache-prog/pkg/cache"
)

type provider struct {
	cacheDir string
}

var _ cache.Provider = &provider{}

func NewProvider(cacheDir string) (*provider, error) {
	cacheDir = filepath.Clean(cacheDir)

	for _, name := range []string{"action", "object"} {
		if err := os.MkdirAll(filepath.Join(cacheDir, name), os.FileMode(0755)); err != nil {
			return nil, err
		}
	}

	return &provider{cacheDir: cacheDir}, nil
}

func (p *provider) actionPath(actionId string) string {
	return filepath.Join(
		p.cacheDir,
		"action",
		actionId,
	)
}

func (p *provider) objPath(objectId string) string {
	return filepath.Join(
		p.cacheDir,
		"object",
		objectId,
	)
}

func (p *provider) KnownCommands() []string {
	return []string{"get", "put", "close"}
}

func (p *provider) Get(actionId string) (string, string, error) {
	data, err := os.ReadFile(p.actionPath(actionId))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return notFound()

	case err != nil:
		return "", "", err
	}

	var parts = strings.SplitN(string(data), ":", 2)
	if len(parts) != 2 {
		// TODO: delete invalid action entry
		return notFound()
	}

	objectId := parts[0]

	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		// TODO: delete invalid action entry
		return notFound()
	}

	diskpath, err := filepath.Abs(p.objPath(objectId))
	if err != nil {
		// TODO: delete invalid action entry
		return notFound()
	}

	fi, err := os.Stat(diskpath)
	if err != nil {
		// TODO: delete invalid action entry
		return notFound()
	}

	if fi.Size() != size {
		// TODO: delete invalid action entry
		return notFound()
	}

	return objectId, diskpath, nil
}

func (p *provider) Put(actionId string, objectId string, body io.Reader) (string, error) {
	diskpath, err := filepath.Abs(p.objPath(objectId))
	if err != nil {
		return "", err
	}

	// TODO Add check whether file already exists?

	file, err := os.Create(diskpath) // #nosec G304 - provider takes care of filepath clean call
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	size, err := io.Copy(file, body)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(p.actionPath(actionId), fmt.Appendf(nil, "%s:%d", objectId, size), os.FileMode(0644)); err != nil {
		return "", err
	}

	return diskpath, nil
}

func (p *provider) Close() error {
	return nil
}

func notFound() (string, string, error) {
	return "", "", nil
}
