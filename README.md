# Go cache program (`GOCACHEPROG`)

[![License](https://img.shields.io/github/license/homeport/go-cache-prog.svg)](https://github.com/homeport/go-cache-prog/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/homeport/go-cache-prog)](https://goreportcard.com/report/github.com/homeport/go-cache-prog)
[![Go Reference](https://pkg.go.dev/badge/github.com/homeport/go-cache-prog.svg)](https://pkg.go.dev/github.com/homeport/go-cache-prog)
[![Release](https://img.shields.io/github/release/homeport/go-cache-prog.svg)](https://github.com/homeport/go-cache-prog/releases/latest)

## Description

Experimental Go Cache program implementation using IBM Cloud Object Storage (COS) as cache backend.

This is a proof-of-concept tool and subject to change.

## Usage

Log into your IBM Cloud account and create a new COS instance in a region that is close to your location to minimize time of objects spend in transit. Create a bucket in your COS instance to be used as the cache. Setup your shell to use `go-cache-prog` by exporting the following environment variables:

```sh
export GO_CACHE_PROG_COS_APIKEY=<apikey-that-has-permission-to-access-cos>
export GO_CACHE_PROG_COS_ENDPOINT=s3.<region>.cloud-object-storage.appdomain.cloud
export GO_CACHE_PROG_COS_RESOURCEINSTANCEID=<crn-of-cos>
export GO_CACHE_PROG_COS_BUCKET=<bucket-name>

export GOCACHEPROG="go-cache-prog cos"
```

The endpoint, resource instance ID and bucket can alternatively be configured via command-line flags, too.

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
