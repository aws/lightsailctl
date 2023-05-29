// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cs implements features related to Lightsail (C)ontainer (S)ervice.
package cs

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lightsail"
	"github.com/docker/docker/api/types/registry"
)

type PushImageInput struct {
	Service string
	Image   string
	Label   string
}

type RegistryLoginCreator interface {
	CreateContainerServiceRegistryLogin(
		context.Context,
		*lightsail.CreateContainerServiceRegistryLoginInput,
		...func(*lightsail.Options),
	) (*lightsail.CreateContainerServiceRegistryLoginOutput, error)
}

type LightsailImageOperator interface {
	RegistryLoginCreator

	RegisterContainerImage(
		context.Context,
		*lightsail.RegisterContainerImageInput,
		...func(*lightsail.Options),
	) (*lightsail.RegisterContainerImageOutput, error)
}

type ImageOperator interface {
	TagImage(ctx context.Context, source, target string) error
	UntagImage(ctx context.Context, image string) error
	PushImage(ctx context.Context, r RemoteImage) (digest string, err error)
}

// PushImage pushes and registers the image to Lightsail service registry.
func PushImage(ctx context.Context, in *PushImageInput, lio LightsailImageOperator, imgo ImageOperator) error {
	authConfig, err := getServiceRegistryAuth(ctx, lio)
	if err != nil {
		return err
	}

	remoteImage := RemoteImage{AuthConfig: *authConfig, Tag: generateUniqueTag()}

	err = imgo.TagImage(ctx, in.Image, remoteImage.Ref())
	if err != nil {
		return err
	}
	defer tryUntagImage(ctx, imgo, remoteImage.Ref())

	digest, err := imgo.PushImage(ctx, remoteImage)
	if err != nil {
		return err
	}

	registered, err := lio.RegisterContainerImage(
		ctx,
		&lightsail.RegisterContainerImageInput{
			ServiceName: &in.Service,
			Label:       &in.Label,
			Digest:      &digest,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("Digest: %s\nImage %q registered.\nRefer to this image as %q in deployments.\n",
		aws.ToString(registered.ContainerImage.Digest),
		in.Image,
		aws.ToString(registered.ContainerImage.Image))

	return nil
}

// getServiceRegistryAuth returns the server address and
// the temporary credentials sufficient to push images to
// Lightsail Containers service repo (aka "sr").
//
// Note that "sr" repo only retains image tags generated
// when RegisterContainerImage API is called with specific image
// digests. The purpose of this repo is to keep images that are
// strictly related to your Lightsail container service deployments.
func getServiceRegistryAuth(ctx context.Context, rlc RegistryLoginCreator) (*registry.AuthConfig, error) {
	out, err := rlc.CreateContainerServiceRegistryLogin(
		ctx,
		new(lightsail.CreateContainerServiceRegistryLoginInput),
	)
	if err != nil {
		return nil, err
	}

	return &registry.AuthConfig{
		Username:      aws.ToString(out.RegistryLogin.Username),
		Password:      aws.ToString(out.RegistryLogin.Password),
		ServerAddress: aws.ToString(out.RegistryLogin.Registry) + "/sr",
	}, nil
}

// tryUntagImage is the same as ImageOperator.UntagImage
// except it doesn't return error and instead logs it.
func tryUntagImage(ctx context.Context, imgo ImageOperator, image string) {
	if err := imgo.UntagImage(ctx, image); err != nil {
		log.Println(err)
	}
}

func generateUniqueTag() string {
	now := time.Now()
	if testNow != nil {
		now = testNow()
	}
	return fmt.Sprintf("%v-%s", now.UnixNano(), randomName13())
}

func randomName13() string {
	r := rand.Reader
	if testRngReader != nil {
		r = testRngReader
	}

	b := make([]byte, 8)
	if _, err := io.ReadFull(r, b); err != nil {
		panic(err)
	}
	return b32.EncodeToString(b)
}

var (
	b32 = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv").WithPadding(base32.NoPadding)

	testNow       func() time.Time
	testRngReader io.Reader
)
