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
	"fmt"
	"math"
	"net"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/gonvenience/bunt"
	"github.com/gonvenience/term"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var regions = []string{
	"us-south",
	"us-east",
	"eu-gb",
	"eu-de",
	"au-syd",
	"jp-tok",
	"jp-osa",
	"ca-tor",
	"br-sao",
	"eu-es",
	"ca-mon",
}

type cosPingCmdOpts struct {
	public  bool
	private bool
	direct  bool
}

var cosPingCmdSettings cosPingCmdOpts

var cosPingCmd = &cobra.Command{
	Use:           "ping",
	Short:         "Ping all regional endpoints",
	Long:          `Ping all regional endpoints to verify they can be reached.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Hidden:        true,

	RunE: func(cmd *cobra.Command, args []string) error {
		type results struct {
			endpoint     string
			dialDuration time.Duration
			attempts     int
			err          error
		}

		var data []results

		var addMutex sync.Mutex
		var add = func(r results) {
			addMutex.Lock()
			defer addMutex.Unlock()
			data = append(data, r)
		}

		var endpoints []string

		if cosPingCmdSettings.public {
			endpoints = append(endpoints, genEndpoints("s3")...)
		}

		if cosPingCmdSettings.private {
			endpoints = append(endpoints, genEndpoints("s3.private")...)
		}

		if cosPingCmdSettings.direct {
			endpoints = append(endpoints, genEndpoints("s3.direct")...)
		}

		g := &errgroup.Group{}
		g.SetLimit(rootCmdSettings.workers)

		for _, endpoint := range endpoints {
			g.Go(func() error {
				var start time.Time

				var attempts int
				conn, err := retry.DoWithData(
					func() (net.Conn, error) {
						attempts++

						start = time.Now()
						return net.DialTimeout(
							"tcp",
							net.JoinHostPort(endpoint, "80"),
							1*time.Second,
						)
					},

					retry.LastErrorOnly(true),
					retry.Delay(time.Second),
					retry.Attempts(5),
				)

				if err != nil {
					add(results{
						endpoint:     endpoint,
						attempts:     attempts,
						dialDuration: math.MaxInt64,
						err:          err,
					})
					return nil
				}

				if err := conn.Close(); err != nil {
					add(results{
						endpoint:     endpoint,
						attempts:     attempts,
						dialDuration: math.MaxInt64,
						err:          err,
					})
					return nil
				}

				add(results{
					endpoint:     endpoint,
					attempts:     attempts,
					dialDuration: time.Since(start),
				})
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		slices.SortFunc(data, func(a, b results) int {
			return int(a.dialDuration - b.dialDuration)
		})

		table := tablewriter.NewTable(
			os.Stdout,
			tablewriter.WithRenderer(renderer.NewBlueprint(
				tw.Rendition{
					Borders: tw.BorderNone,
					Symbols: tw.NewSymbolCustom("").
						WithColumn(bunt.Sprintf("DimGray{│}")).
						WithRow(bunt.Sprintf("DimGray{─}")).
						WithCenter(bunt.Sprintf("DimGray{┼}")),
				},
			)),
			tablewriter.WithConfig(tablewriter.Config{
				MaxWidth: term.GetTerminalWidth() - 5,
				Row: tw.CellConfig{
					Formatting: tw.CellFormatting{AutoWrap: tw.WrapNormal},
				},
			}),
		)

		_ = table.Append(bold("Duration", "Endpoint", "Note"))
		for _, entry := range data {
			_ = table.Append([]string{
				func() string {
					if entry.err != nil {
						return "n/a"
					}

					return entry.dialDuration.Round(time.Millisecond).String()
				}(),

				entry.endpoint,

				func() string {
					if entry.err != nil {
						return entry.err.Error()
					}

					return fmt.Sprintf("took %s", plural(entry.attempts, "attempt"))
				}(),
			})
		}

		if err := table.Render(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	cosCmd.AddCommand(cosPingCmd)

	cosPingCmd.Flags().SortFlags = false
	cosPingCmd.Flags().BoolVar(&cosPingCmdSettings.public, "public", true, "ping public endpoints")
	cosPingCmd.Flags().BoolVar(&cosPingCmdSettings.private, "private", false, "ping private endpoints")
	cosPingCmd.Flags().BoolVar(&cosPingCmdSettings.direct, "direct", false, "ping direct endpoints")
}

func plural(num int, text string) string {
	if num == 1 {
		return fmt.Sprintf("%d %s", num, text)
	}

	return fmt.Sprintf("%d %ss", num, text)
}

func genEndpoints(prefix string) []string {
	var endpoints []string
	for _, region := range regions {
		endpoints = append(endpoints, prefix+"."+region+"."+"cloud-object-storage.appdomain.cloud")
	}
	return endpoints
}

func bold(list ...string) []string {
	var results []string
	for _, entry := range list {
		results = append(results, bunt.Style(entry, bunt.Bold()))
	}
	return results
}
