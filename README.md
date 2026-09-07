# Go cache program (`GOCACHEPROG`)

[![License](https://img.shields.io/github/license/homeport/go-cache-prog.svg)](https://github.com/homeport/go-cache-prog/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/homeport/go-cache-prog)](https://goreportcard.com/report/github.com/homeport/go-cache-prog)
[![Go Reference](https://pkg.go.dev/badge/github.com/homeport/go-cache-prog.svg)](https://pkg.go.dev/github.com/homeport/go-cache-prog)
[![Release](https://img.shields.io/github/release/homeport/go-cache-prog.svg)](https://github.com/homeport/go-cache-prog/releases/latest)

## Description

Experimental Go Cache program implementation using IBM Cloud Object Storage (COS) as cache backend.

This is a proof-of-concept tool and subject to change.

## Usage

### Quick setup (recommended)

If you have the [IBM Cloud CLI](https://cloud.ibm.com/docs/cli) installed and are logged in (`ibmcloud login`), the `config-helper` command guides you through an interactive setup and generates a ready-to-use config JSON:

```sh
export GO_CACHE_PROG_COS_CONFIG=$(go-cache-prog cos config-helper)
export GOCACHEPROG="go-cache-prog cos"
```

`config-helper` walks you through selecting a COS instance, HMAC credentials, bucket, endpoint variant, and local cache directory — all with arrow-key navigation. The resulting JSON is printed to stdout and can be stored as a CI/CD secret.

### Manual setup

Log into your IBM Cloud account and create a new COS instance in a region that is close to your location to minimize time of objects spend in transit. Create a bucket in your COS instance to be used as the cache. Generate HMAC credentials for your COS instance. Setup your shell to use `go-cache-prog` by exporting the following environment variables:

```sh
export GO_CACHE_PROG_COS_ENDPOINT=s3.<region>.cloud-object-storage.appdomain.cloud
export GO_CACHE_PROG_COS_REGION=<region>
export GO_CACHE_PROG_COS_BUCKET=<bucket-name>
export GO_CACHE_PROG_COS_ACCESSKEYID=<access-key-id>
export GO_CACHE_PROG_COS_SECRETACCESSKEY=<secret-access-key>

export GOCACHEPROG="go-cache-prog cos"
```

The endpoint, region, bucket, and credentials can alternatively be configured via command-line flags.

### JSON config via `GO_CACHE_PROG_COS_CONFIG`

All COS settings can be supplied as a single JSON string in the `GO_CACHE_PROG_COS_CONFIG` environment variable. This is especially convenient for CI/CD pipelines where storing one secret is easier than five:

```sh
export GO_CACHE_PROG_COS_CONFIG='{
  "cos": {
    "endpoint": "s3.us-south.cloud-object-storage.appdomain.cloud",
    "region": "us-south",
    "bucket": "my-cache-bucket",
    "access_key_id": "abc123",
    "secret_access_key": "supersecret"
  },
  "cache_dir": "/tmp/go-cache"
}'
export GOCACHEPROG="go-cache-prog cos"
```

**Configuration precedence** (highest wins):

1. CLI flags (e.g. `--endpoint`, `--bucket`)
2. Individual environment variables (`GO_CACHE_PROG_COS_ENDPOINT`, `GO_CACHE_PROG_COS_REGION`, `GO_CACHE_PROG_COS_BUCKET`, `GO_CACHE_PROG_COS_ACCESSKEYID`, `GO_CACHE_PROG_COS_SECRETACCESSKEY`, `GO_CACHE_PROG_COS_CACHEDIR`)
3. `GO_CACHE_PROG_COS_CONFIG` JSON blob (loaded first, overridden by the above)

## Installation

### Homebrew

The `homeport/tap` has macOS and GNU/Linux pre-built binaries available:

```bash
brew install homeport/tap/go-cache-prog
```

### Pre-built binaries in GitHub

Prebuilt binaries can be [downloaded from the GitHub Releases section](https://github.com/homeport/go-cache-prog/releases/latest).

### Curl To Shell Convenience Script

There is a convenience script to download the latest release for Linux or macOS if you want to need it simple (you need `curl` and `jq` installed on your machine):

```bash
curl --silent --location https://raw.githubusercontent.com/homeport/go-cache-prog/refs/heads/main/hack/download.sh | bash
```

### Build from Source

You can install `go-cache-prog` from source using `go install`:

```bash
go install github.com/homeport/go-cache-prog/cmd/go-cache-prog@latest
```

## Contributing

We are happy to have other people contributing to the project. If you decide to do that, here's how to:

- get Go (`go-cache-prog` requires Go version 1.23 or greater)
- fork the project
- create a new branch
- make your changes
- open a PR.

Git commit messages should be meaningful and follow the rules nicely written down by [Chris Beams](https://chris.beams.io/posts/git-commit/):
> The seven rules of a great Git commit message
>
> 1. Separate subject from body with a blank line
> 1. Limit the subject line to 50 characters
> 1. Capitalize the subject line
> 1. Do not end the subject line with a period
> 1. Use the imperative mood in the subject line
> 1. Wrap the body at 72 characters
> 1. Use the body to explain what and why vs. how

## License

Licensed under [MIT License](https://github.com/homeport/go-cache-prog/blob/main/LICENSE)
