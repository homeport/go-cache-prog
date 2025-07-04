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

package cmd

import (
	"os"

	"github.com/homeport/go-cache-prog/pkg/cache"
	"github.com/homeport/go-cache-prog/pkg/provider/local"
	"github.com/spf13/cobra"
)

type localCmdOpts struct {
	cacheDir string
}

var localCmdSettings localCmdOpts

var localCmd = &cobra.Command{
	Use:           "local",
	Short:         "Use local directory as cache backend",
	Long:          `Use local directory as cache backend`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Hidden:        true,

	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := local.NewProvider(localCmdSettings.cacheDir)
		if err != nil {
			return err
		}

		handler := cache.New(os.Stdin, os.Stdout, provider).
			WithConcurrentWorkers(rootCmdSettings.workers)

		if rootCmdSettings.logfile != "" {
			file, err := os.OpenFile(rootCmdSettings.logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				return err
			}
			defer func() { _ = file.Close() }()

			handler.WithLogOutput(file)
		}

		return handler.Run(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(localCmd)

	localCmd.Flags().SortFlags = false
	localCmd.Flags().StringVar(&localCmdSettings.cacheDir, "cache-dir", "/tmp/go-cache", "location of the local cache directory")
}
