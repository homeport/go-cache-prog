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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/homeport/go-cache-prog/pkg/cache"
	"github.com/homeport/go-cache-prog/pkg/provider/cos"
	"github.com/spf13/cobra"
)

type cosCmdOpts struct {
	config cos.Config
}

var cosCmdSettings cosCmdOpts

var cosCmd = &cobra.Command{
	Use:           "cos",
	Short:         "Use IBM Cloud Object Storage as cache backend",
	Long:          `Use IBM Cloud Object Storage as cache backend`,
	SilenceUsage:  true,
	SilenceErrors: true,

	// PersistentPreRunE runs after flag parsing so env-var values only fill in
	// fields that were not explicitly set via a CLI flag. Precedence is:
	//   GO_CACHE_PROG_COS_CONFIG JSON  <  individual GO_CACHE_PROG_COS_* vars  <  CLI flags
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load JSON blob first so individual vars and flags can override it.
		mapOsEnvToConfig("GO_CACHE_PROG_COS_CONFIG", &cosCmdSettings.config)

		// Individual env vars override JSON values, but only when the
		// corresponding flag was not explicitly provided on the command line.
		if !cmd.Flags().Changed("endpoint") {
			mapOsEnvToVarIfSet("GO_CACHE_PROG_COS_ENDPOINT", &cosCmdSettings.config.Cos.Endpoint)
		}
		if !cmd.Flags().Changed("region") {
			mapOsEnvToVarIfSet("GO_CACHE_PROG_COS_REGION", &cosCmdSettings.config.Cos.Region)
		}
		if !cmd.Flags().Changed("bucket") {
			mapOsEnvToVarIfSet("GO_CACHE_PROG_COS_BUCKET", &cosCmdSettings.config.Cos.Bucket)
		}
		if !cmd.Flags().Changed("access-key-id") {
			mapOsEnvToVarIfSet("GO_CACHE_PROG_COS_ACCESSKEYID", &cosCmdSettings.config.Cos.AccessKeyID)
		}
		if !cmd.Flags().Changed("secret-access-key") {
			mapOsEnvToVarIfSet("GO_CACHE_PROG_COS_SECRETACCESSKEY", &cosCmdSettings.config.Cos.SecretAccessKey)
		}
		if !cmd.Flags().Changed("cache-dir") {
			mapOsEnvToVarIfSet("GO_CACHE_PROG_COS_CACHEDIR", &cosCmdSettings.config.CacheDir)
		}

		return nil
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := cos.NewProvider(cosCmdSettings.config)
		if err != nil {
			return err
		}

		handler := cache.New(os.Stdin, os.Stdout, provider).WithConcurrentWorkers(rootCmdSettings.workers)

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
	rootCmd.AddCommand(cosCmd)
	cosCmd.Flags().SortFlags = false

	cosCmd.PersistentFlags().StringVar(&cosCmdSettings.config.CacheDir, "cache-dir", filepath.Join(os.TempDir(), "go-cache"), "location of the local cache directory")

	cosCmd.PersistentFlags().StringVar(&cosCmdSettings.config.Cos.Endpoint, "endpoint", "", "specify URL endpoint of the COS instance")
	cosCmd.PersistentFlags().StringVar(&cosCmdSettings.config.Cos.Region, "region", "", "specify region of the COS instance")
	cosCmd.PersistentFlags().StringVar(&cosCmdSettings.config.Cos.AccessKeyID, "access-key-id", "", "specify access key id of the COS instance")
	cosCmd.PersistentFlags().StringVar(&cosCmdSettings.config.Cos.SecretAccessKey, "secret-access-key", "", "specify secret access key of the COS instance")
	cosCmd.PersistentFlags().StringVar(&cosCmdSettings.config.Cos.Bucket, "bucket", "", "specify bucket to be used")
}

func mapOsEnvToVarIfSet(key string, target *string) {
	val, found := os.LookupEnv(key)
	if !found {
		return
	}

	*target = val
}

func mapOsEnvToConfig(key string, target *cos.Config) {
	val, found := os.LookupEnv(key)
	if !found {
		return
	}

	if err := json.Unmarshal([]byte(val), target); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse configuration from environment variable %q: %v", key, err)
	}
}
