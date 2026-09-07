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
	"fmt"
	"net/http"
	"time"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
)

// KnownRegions is the set of IBM COS regions presented to the user when
// creating a new bucket. Regional entries come first, followed by single data
// center locations. Source: cloud.ibm.com/docs/cloud-object-storage (endpoints).
var KnownRegions = []string{
	// Regional
	"us-south", "us-east",
	"eu-de", "eu-gb", "eu-es",
	"jp-tok", "jp-osa",
	"au-syd", "ca-tor", "ca-mon", "br-sao",
	"in-che", "in-mum",
	// Single data center
	"che01", "mon01", "par01", "ams03", "sjc04", "sng01",
}

// LocationConstraint returns the IBM COS LocationConstraint value for a given
// region, which is always "<region>-standard" for new buckets.
// IBM COS does not accept bare region codes (e.g. "us-east") as location
// constraints — the storage-class suffix is required.
func LocationConstraint(region string) string {
	return region + "-standard"
}

// RegionalS3Client returns an S3 client for the given region using HMAC
// credentials. The endpoint and signing region are both derived from region,
// which IBM COS requires to match for SigV4 HMAC authentication.
func (h *COSConfigHelper) RegionalS3Client(accessKeyID, secretAccessKey, region string) (*s3.S3, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("https://s3.%s.cloud-object-storage.appdomain.cloud", region)

	return s3.New(sess, aws.NewConfig().
		WithEndpoint(endpoint).
		WithRegion(region).
		WithCredentials(credentials.NewStaticCredentialsFromCreds(credentials.Value{
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
		})).
		WithLowerCaseHeaderMaps(true).
		WithS3ForcePathStyle(true).
		WithHTTPClient(&http.Client{
			Timeout:   15 * time.Second,
			Transport: &verboseTransport{transport: http.DefaultTransport, verbose: h.verbose},
		}),
	), nil
}

// ListBuckets returns the bucket names visible from the given region. IBM COS
// SigV4 HMAC signing requires the credential-scope region in the Authorization
// header to match the endpoint region, and ListBuckets is scoped to that
// region — buckets homed elsewhere will not appear.
func (h *COSConfigHelper) ListBuckets(accessKeyID, secretAccessKey, region string) ([]string, error) {
	client, err := h.RegionalS3Client(accessKeyID, secretAccessKey, region)
	if err != nil {
		return nil, err
	}
	out, err := client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("could not list buckets: %w", err)
	}
	names := make([]string, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		if b.Name != nil {
			names = append(names, *b.Name)
		}
	}
	return names, nil
}

// CreateBucket creates a new COS bucket in the given region using HMAC
// credentials. The IBM COS SDK requires both the region and the location
// constraint to be set to the same value.
func (h *COSConfigHelper) CreateBucket(accessKeyID, secretAccessKey, bucket, region string) error {
	client, err := h.RegionalS3Client(accessKeyID, secretAccessKey, region)
	if err != nil {
		return err
	}
	constraint := LocationConstraint(region)
	_, err = client.CreateBucket(&s3.CreateBucketInput{
		Bucket: &bucket,
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: &constraint,
		},
	})
	return err
}
