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

package cos

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/homeport/go-cache-prog/pkg/cache"
	"github.com/homeport/go-cache-prog/pkg/provider/local"
)

const DefaultAuthEndpoint = "https://iam.cloud.ibm.com/identity/token"
const DefaultMinUploadSize = 1024

const objectIdKey = "objectid"
const sizeKey = "size"

type provider struct {
	config Config
	client *s3.S3

	localProvider cache.Provider
}

type Config struct {
	Cos           Cos
	CacheDir      string
	MinUploadSize int64
}

type Cos struct {
	AuthEndpoint string
	ApiKey       string

	Endpoint           string
	ResourceInstanceId string
	Bucket             string
}

var _ cache.Provider = &provider{}

func (p *provider) actionKey(actionId string) string {
	return "action/" + actionId
}

func (p *provider) KnownCommands() []string {
	return []string{"get", "put", "close"}
}

func NewProvider(config Config) (*provider, error) {
	if config.CacheDir == "" {
		return nil, fmt.Errorf("cache directory cannot be empty")
	}

	if config.MinUploadSize <= 0 {
		config.MinUploadSize = DefaultMinUploadSize
	}

	localProvider, err := local.NewProvider(config.CacheDir)
	if err != nil {
		return nil, err
	}

	session, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	client := s3.New(
		session,
		aws.NewConfig().
			WithEndpoint(config.Cos.Endpoint).
			WithCredentials(ibmiam.NewStaticCredentials(
				aws.NewConfig(),
				config.Cos.AuthEndpoint,
				config.Cos.ApiKey,
				config.Cos.ResourceInstanceId,
			)).
			WithLowerCaseHeaderMaps(true).
			WithS3ForcePathStyle(true).
			WithHTTPClient(&http.Client{
				Timeout: 5 * time.Second,
			}).
			WithMaxRetries(2))

	listBucketResp, err := client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	var bucketFound bool
	for _, bucket := range listBucketResp.Buckets {
		if config.Cos.Bucket == *bucket.Name {
			bucketFound = true
			break
		}
	}

	if !bucketFound {
		return nil, fmt.Errorf("failed to find bucket %q in COS", config.Cos.Bucket)
	}

	return &provider{
		client:        client,
		config:        config,
		localProvider: localProvider,
	}, nil
}

func lookUpObjectId(metadata map[string]*string) (string, bool) {
	val, found := metadata[objectIdKey]
	if !found || val == nil {
		return "", false
	}

	return *val, true
}

func lookUpSize(metadata map[string]*string) (int64, bool) {
	val, found := metadata[sizeKey]
	if !found || val == nil {
		return -1, false
	}

	size, err := strconv.ParseInt(*val, 10, 64)
	if err != nil {
		return -1, false
	}

	return size, true
}

func (p *provider) Get(actionId string) (string, string, error) {
	objectId, diskpath, err := p.localProvider.Get(actionId)
	if err != nil {
		return failure(err)
	}

	if objectId != "" && diskpath != "" {
		return objectId, diskpath, nil
	}

	// --- --- ---

	var cacheEntry = &s3.GetObjectInput{
		Bucket: &p.config.Cos.Bucket,
		Key:    ptr(p.actionKey(actionId)),
	}

	res, err := p.client.GetObject(cacheEntry)
	if err != nil {
		return notFound()
	}

	objectId, found := lookUpObjectId(res.Metadata)
	if !found {
		// TODO: delete invalid action entry
		return notFound()
	}

	size, found := lookUpSize(res.Metadata)
	if !found {
		// TODO: delete invalid action entry
		return notFound()
	}

	diskpath, err = p.localProvider.Put(actionId, objectId, res.Body)
	if err != nil {
		return notFound()
	}

	fi, err := os.Stat(diskpath)
	if err != nil {
		return failure(err)
	}

	if fi.Size() != size {
		// TODO: delete invalid action entry
		return notFound()
	}

	return objectId, diskpath, nil
}

func (p *provider) Put(actionId string, objectId string, body io.Reader) (string, error) {
	diskpath, err := p.localProvider.Put(actionId, objectId, body)
	if err != nil {
		return "", err
	}

	// --- --- ---

	fi, err := os.Stat(diskpath)
	if err != nil {
		return "", err
	}

	size := fi.Size()

	if size < p.config.MinUploadSize {
		return diskpath, nil
	}

	file, err := os.Open(diskpath) // #nosec G304 - provider takes care of filepath clean call
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	_, err = p.client.PutObject(&s3.PutObjectInput{
		Bucket: &p.config.Cos.Bucket,
		Key:    ptr(p.actionKey(actionId)),

		Metadata: map[string]*string{
			objectIdKey: &objectId,
			sizeKey:     ptr(strconv.FormatInt(size, 10)),
		},

		Body:          file,
		ContentLength: &size,
	})

	return diskpath, err
}

func (p *provider) Close() error {
	// TODO Implement more close stuff?

	if err := p.localProvider.Close(); err != nil {
		return err
	}

	p.client.Config.HTTPClient.CloseIdleConnections()
	return nil
}

func notFound() (string, string, error) {
	return "", "", nil
}

func failure(err error) (string, string, error) {
	return "", "", err
}

func ptr[T any](t T) *T { return &t }
