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
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/homeport/go-cache-prog/pkg/errgroup"
)

type Handler struct {
	in       io.Reader
	out      io.Writer
	provider Provider

	log *log.Logger

	workers int
}

func New(in io.Reader, out io.Writer, provider Provider) *Handler {
	return &Handler{
		in:       in,
		out:      out,
		provider: provider,
		log:      log.New(io.Discard, "", log.LstdFlags),
		workers:  1,
	}
}

func (h *Handler) WithConcurrentWorkers(workers int) *Handler {
	h.workers = workers
	return h
}

func (h *Handler) WithLogOutput(w io.Writer) *Handler {
	h.log.SetOutput(w)
	return h
}

func (h *Handler) Run(_ context.Context) error {
	reader := bufio.NewReader(h.in)
	decoder := json.NewDecoder(reader)

	writer := bufio.NewWriter(h.out)
	encoder := json.NewEncoder(writer)

	// ---

	var writeMutex sync.Mutex
	var write = func(v any) error {
		writeMutex.Lock()
		defer writeMutex.Unlock()
		if err := encoder.Encode(v); err != nil {
			return err
		}

		return writer.Flush()
	}

	// ---

	if err := write(&progResponse{ID: 0, KnownCommands: h.provider.KnownCommands()}); err != nil {
		return err
	}

	// ---

	// Limit number of workers to configured number
	// plus one for the request producer itself
	g := errgroup.New(h.workers + 1)
	g.Go(func() error {
		for {
			var req progRequest
			err := decoder.Decode(&req)
			switch {
			case errors.Is(err, io.EOF):
				return nil

			case err != nil:
				return fmt.Errorf("failed to decode: %w", err)
			}

			// --- --- ---

			switch req.Command {
			case "get":
				if len(req.ActionID) == 0 {
					return fmt.Errorf("invalid ActionID")
				}

				g.Go(func() error {
					resp, err := h.handleGet(&req)
					if err != nil {
						return err
					}

					if err := write(resp); err != nil {
						return err
					}

					return nil
				})

			case "put":
				if len(req.OutputID) == 0 {
					return fmt.Errorf("invalid OutputID")
				}

				if req.BodySize > 0 {
					var body []byte
					if err := decoder.Decode(&body); err != nil {
						return err
					}

					if req.BodySize != int64(len(body)) {
						return fmt.Errorf("error processing request #%d, size mismatch: request=%d and body=%d", req.ID, req.BodySize, len(body))
					}

					req.Body = bytes.NewReader(body)
				}

				if req.Body == nil {
					req.Body = bytes.NewReader(nil)
				}

				g.Go(func() error {
					resp, err := h.handlePut(&req)
					if err != nil {
						return err
					}

					if err := write(resp); err != nil {
						return err
					}

					return nil
				})

			case "close":
				defer g.Done()
				return h.handleClose(&req)

			default:
				return fmt.Errorf("unsupported command %q", req.Command)
			}
		}
	})

	return g.Wait()
}

func (h *Handler) handleGet(req *progRequest) (*progResponse, error) {
	pid, diskpath, err := h.provider.Get(enc(req.ActionID))
	if err != nil {
		return nil, fmt.Errorf("failed to obtain entry from cache: %w", err)
	}

	if pid == "" && diskpath == "" {
		return cacheMiss(req)
	}

	outputID, err := dec(pid)
	if err != nil {
		return nil, err
	}

	return cacheHit(req, outputID, diskpath)
}

func (h *Handler) handlePut(req *progRequest) (*progResponse, error) {
	path, err := h.provider.Put(enc(req.ActionID), enc(req.OutputID), req.Body)
	if err != nil {
		return nil, err
	}

	return &progResponse{ID: req.ID, DiskPath: path}, nil
}

func (h *Handler) handleClose(_ *progRequest) error {
	return h.provider.Close()
}

func cacheMiss(req *progRequest) (*progResponse, error) {
	return &progResponse{
		ID:   req.ID,
		Miss: true,
	}, nil
}

func cacheHit(req *progRequest, objectId []byte, diskpath string) (*progResponse, error) {
	fi, err := os.Stat(diskpath)
	if err != nil {
		return nil, err
	}

	modTime := fi.ModTime()
	return &progResponse{
		ID:       req.ID,
		OutputID: objectId,
		Size:     fi.Size(),
		Time:     &modTime,
		DiskPath: diskpath,
	}, nil
}

func enc(in []byte) string {
	return hex.EncodeToString(in)
}

func dec(in string) ([]byte, error) {
	return hex.DecodeString(in)
}
