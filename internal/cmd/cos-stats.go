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

package cmd

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/spf13/cobra"
)

var cosStatsCmd = &cobra.Command{
	Use:           "stats",
	Short:         "Display statistics about objects in the COS bucket",
	Long:          `Display statistics about objects in the COS bucket including counts, sizes, and ages`,
	SilenceUsage:  true,
	SilenceErrors: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := session.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}

		client := s3.New(
			sess,
			aws.NewConfig().
				WithEndpoint(cosCmdSettings.config.Cos.Endpoint).
				WithRegion(cosCmdSettings.config.Cos.Region).
				WithCredentials(credentials.NewStaticCredentialsFromCreds(credentials.Value{
					AccessKeyID:     cosCmdSettings.config.Cos.AccessKeyID,
					SecretAccessKey: cosCmdSettings.config.Cos.SecretAccessKey,
				})).
				WithLowerCaseHeaderMaps(true).
				WithS3ForcePathStyle(true).
				WithHTTPClient(&http.Client{
					Timeout: cosCmdSettings.config.Cos.Timeout,
				}).
				WithMaxRetries(cosCmdSettings.config.Cos.MaxRetries),
		)

		var objectCount int

		var min int64 = math.MaxInt64
		var max int64
		var sizes []int64
		var oldest = time.Now()

		var pageFunc = func(listObjectOutput *s3.ListObjectsOutput, _ bool) bool {
			objectCount += len(listObjectOutput.Contents)

			for _, object := range listObjectOutput.Contents {
				if object.LastModified != nil {
					if object.LastModified.Before(oldest) {
						oldest = *object.LastModified
					}
				}

				sizes = append(sizes, *object.Size)

				if min > *object.Size {
					min = *object.Size
				}

				if max < *object.Size {
					max = *object.Size
				}
			}

			return true
		}

		if err := client.ListObjectsPages(&s3.ListObjectsInput{Bucket: &cosCmdSettings.config.Cos.Bucket}, pageFunc); err != nil {
			return err
		}

		fmt.Printf("Oldest object: %v (age: %v)\n", oldest.Format(time.RFC3339), humanReadableDuration(time.Since(oldest)))

		fmt.Printf("Smallest object size: %s\n", humanReadableSize(min))
		fmt.Printf("Average object size: %s\n", humanReadableSize(avg(sizes)))
		fmt.Printf("Median object size: %s\n", humanReadableSize(median(sizes)))
		fmt.Printf("Largest object size: %s\n", humanReadableSize(max))

		fmt.Printf("Total object count: %d\n", objectCount)

		return nil
	},
}

func init() {
	cosCmd.AddCommand(cosStatsCmd)
}

func avg(data []int64) int64 {
	if len(data) == 0 {
		return 0
	}
	return int64(float64(sum(data)) / float64(len(data)))
}

func sum(data []int64) int64 {
	var total int64
	for _, val := range data {
		total += val
	}
	return total
}

func median(data []int64) int64 {
	if len(data) == 0 {
		return 0
	}

	list := make([]int64, len(data))
	copy(list, data)
	slices.Sort(list)

	length := len(list)
	if length%2 == 0 {
		return (list[length/2-1] + list[length/2]) / 2
	}
	return list[length/2]
}

func humanReadableSize(bytes int64) string {
	var mods = []string{"Byte", "KiB", "MiB", "GiB", "TiB"}

	value := float64(bytes)
	i := 0
	for value > 1023.99999 {
		value /= 1024.0
		i++
	}

	return fmt.Sprintf("%.1f %s", value, mods[i])
}

func humanReadableDuration(duration time.Duration) string {
	if duration < time.Second {
		return "less than a second"
	}

	var (
		seconds = int(duration.Seconds())
		minutes = 0
		hours   = 0
		days    = 0
	)

	if seconds >= 60 {
		minutes = seconds / 60
		seconds %= 60

		if minutes >= 60 {
			hours = minutes / 60
			minutes %= 60

			if hours >= 24 {
				days = hours / 24
				hours %= 24
			}
		}
	}

	parts := []string{}

	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}

	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}

	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dmin", minutes))
	}

	if seconds > 0 {
		parts = append(parts, fmt.Sprintf("%dsec", seconds))
	}

	switch len(parts) {
	case 1:
		return parts[0]

	default:
		return strings.Join(parts[:2], " ")
	}
}
