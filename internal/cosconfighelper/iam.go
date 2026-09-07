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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// IBMCloudLocalConfig holds the fields we need from ~/.bluemix/config.json.
type IBMCloudLocalConfig struct {
	IAMToken    string `json:"IAMToken"`
	AccountName string `json:"-"` // populated from Account.Name below
	Account     struct {
		Name string `json:"Name"`
		GUID string `json:"GUID"`
	} `json:"Account"`
}

// ReadIBMCloudConfig reads ~/.bluemix/config.json (or $IBMCLOUD_HOME/config.json)
// and returns the parsed config including the IAM bearer token and account name.
func ReadIBMCloudConfig() (*IBMCloudLocalConfig, error) {
	home := os.Getenv("IBMCLOUD_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		home = filepath.Join(userHome, ".bluemix")
	}

	data, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		return nil, fmt.Errorf("cannot read IBM Cloud config (%s): %w", filepath.Join(home, "config.json"), err)
	}

	var cfg IBMCloudLocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse IBM Cloud config: %w", err)
	}

	cfg.AccountName = cfg.Account.Name

	return &cfg, nil
}

// IAMTokenExpiry decodes the exp claim from an IAM JWT without verifying its
// signature. The token string may include a "Bearer " prefix.
func IAMTokenExpiry(token string) (time.Time, error) {
	token = strings.TrimPrefix(token, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("not a valid JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot decode JWT payload: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("cannot parse JWT claims: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("JWT has no exp claim")
	}
	return time.Unix(claims.Exp, 0), nil
}
