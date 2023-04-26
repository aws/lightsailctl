// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/lightsail"
)

type fakeContainerAPIMetadataGetter string

func (f fakeContainerAPIMetadataGetter) GetContainerAPIMetadata(
	context.Context,
	*lightsail.GetContainerAPIMetadataInput,
	...func(*lightsail.Options),
) (*lightsail.GetContainerAPIMetadataOutput, error) {
	switch {
	case f == "":
		return &lightsail.GetContainerAPIMetadataOutput{}, nil
	case strings.Contains(string(f), "error"):
		return nil, errors.New(string(f))
	default:
		return &lightsail.GetContainerAPIMetadataOutput{
			Metadata: []map[string]string{
				{
					"name":  "lightsailctlVersion",
					"value": string(f),
				},
			},
		}, nil
	}
}

func ExampleCheckForUpdates() {
	defer func(w io.Writer, flags int, p string) {
		log.SetOutput(w)
		log.SetFlags(flags)
		log.SetPrefix(p)
	}(log.Writer(), log.Flags(), log.Prefix())

	log.SetOutput(os.Stdout)
	log.SetFlags(0)
	log.SetPrefix("[logger] ")
	debugLog := log.New(log.Writer(), log.Prefix(), log.Flags())

	ctx := context.Background()

	CheckForUpdates(ctx, debugLog, fakeContainerAPIMetadataGetter("1.4.33"), "v1.4.33")
	CheckForUpdates(ctx, debugLog, fakeContainerAPIMetadataGetter("very bad error occurred"), "v1.4.33")

	fmt.Println("now we should get warnings")
	CheckForUpdates(ctx, debugLog, fakeContainerAPIMetadataGetter("v1.6.11"), "v1.4.33")
	CheckForUpdates(ctx, debugLog, fakeContainerAPIMetadataGetter("v2.7.3"), "v2.7.3-beta")

	// Output:
	// [logger] could not get latest lightsailctl version: very bad error occurred
	// now we should get warnings
	// [logger] WARNING: You are using lightsailctl v1.4.33, but v1.6.11 is available.
	// To download, visit https://lightsail.aws.amazon.com/ls/docs/en_us/articles/amazon-lightsail-install-software
	// [logger] WARNING: You are using lightsailctl v2.7.3-beta, but v2.7.3 is available.
	// To download, visit https://lightsail.aws.amazon.com/ls/docs/en_us/articles/amazon-lightsail-install-software
}

func TestGetLatestLightsailctlVersion(t *testing.T) {
	ctx := context.Background()

	for i, c := range []struct {
		input   string
		wantVer Semver
		wantErr string
	}{
		{
			input:   "network timeout error",
			wantVer: "",
			wantErr: "could not get latest lightsailctl version: network timeout error",
		},
		{
			input:   "",
			wantVer: "",
			wantErr: "latest lightsailctl version was not in GetContainerAPIMetadata response",
		},
		{
			input:   "bogus",
			wantVer: "",
			wantErr: `latest lightsailctl version is not a semver: "bogus"`,
		},
		{
			input:   "1.4.0-beta",
			wantVer: "1.4.0-beta",
			wantErr: "",
		},
		{
			input:   "1.4.1",
			wantVer: "1.4.1",
			wantErr: "",
		},
	} {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			gotErr := ""
			gotVer, err := getLatestLightsailctlVersion(ctx, fakeContainerAPIMetadataGetter(c.input))
			if err != nil {
				gotErr = err.Error()
			}
			if c.wantErr != gotErr {
				t.Errorf("got error %q, want error %q", gotErr, c.wantErr)
			}
			if c.wantVer != gotVer {
				t.Errorf("got ver %q, want ver %q", gotVer, c.wantVer)
			}
		})
	}
}
