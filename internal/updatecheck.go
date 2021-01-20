// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/lightsail"
)

type ContainerAPIMetadataGetter interface {
	GetContainerAPIMetadataWithContext(
		context.Context,
		*lightsail.GetContainerAPIMetadataInput,
		...request.Option,
	) (*lightsail.GetContainerAPIMetadataOutput, error)
}

func CheckForUpdates(
	ctx context.Context,
	debugLog *log.Logger,
	g ContainerAPIMetadataGetter,
	inUse Semver,
) {
	available, err := getLatestLightsailctlVersion(ctx, g)
	if err != nil {
		debugLog.Print(err.Error())
		return
	}

	if inUse.Less(available) {
		log.Printf(`WARNING: You are using lightsailctl %s, but %s is available.
To download, visit https://lightsail.aws.amazon.com/ls/docs/en_us/articles/amazon-lightsail-install-software`,
			inUse, available)
	}
}

func getLatestLightsailctlVersion(
	ctx context.Context,
	g ContainerAPIMetadataGetter,
) (Semver, error) {
	res, err := g.GetContainerAPIMetadataWithContext(ctx, new(lightsail.GetContainerAPIMetadataInput))
	if err != nil {
		return "", fmt.Errorf("could not get latest lightsailctl version: %v", err)
	}

	var rawSemver string
	for _, md := range res.Metadata {
		if v := aws.StringValue(md["name"]); v == "lightsailctlVersion" {
			rawSemver = aws.StringValue(md["value"])
		}
	}

	if rawSemver == "" {
		return "", fmt.Errorf("latest lightsailctl version was not in GetContainerAPIMetadata response")
	}

	ver := Semver(rawSemver)
	if !ver.IsValid() {
		return "", fmt.Errorf("latest lightsailctl version is not a semver: %q", rawSemver)
	}

	return ver, nil
}
