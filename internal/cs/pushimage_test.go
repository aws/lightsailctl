// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cs

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/lightsail"
	"github.com/docker/docker/api/types"
)

func TestGenerateUniqueTag(t *testing.T) {
	defer func() {
		testNow, testRngReader = nil, nil
	}()
	testNow = func() time.Time { return time.Unix(0, 1593224653252075123) }
	testRngReader = strings.NewReader("abcdefgh")
	if want, got := "1593224653252075123-c5h66p35cpjmg", generateUniqueTag(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetServiceRegistryAuth(t *testing.T) {
	ctx := context.Background()
	if got, err := getServiceRegistryAuth(ctx, &fakeRegistryLoginCreator{failToCreateLogin: true}); err == nil || got != nil {
		t.Errorf("got err: %v", err)
		t.Errorf("got out: %#v", got)
	}

	want := &types.AuthConfig{
		Username:      "gollum",
		Password:      "precious",
		ServerAddress: "123456789012.dkr.ecr.so-fake-2.amazonaws.com/sr",
	}
	if got, err := getServiceRegistryAuth(ctx, &fakeRegistryLoginCreator{}); err != nil {
		t.Errorf("got err: %v", err)
	} else if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %#v", got)
		t.Logf("want: %#v", want)
	}
}

func TestPushImageErrors(t *testing.T) {
	defer func() {
		testNow, testRngReader = nil, nil
	}()
	testNow = func() time.Time { return time.Unix(1611800397, 0) }
	testRngReader = strings.NewReader("abcdefgh")

	type test struct {
		ls   fakeLightsailImageOperator
		imgo fakeImageOperator
		want string
	}
	ctx := context.Background()
	in := &PushImageInput{Service: "doge", Image: "nginx:latest", Label: "www"}
	for i, test := range []test{
		{
			ls:   fakeLightsailImageOperator{fakeRegistryLoginCreator: fakeRegistryLoginCreator{failToCreateLogin: true}},
			want: "failed: create login",
		},
		{
			ls:   fakeLightsailImageOperator{failToRegister: true},
			want: "failed: register (doge, www, sha256:10b8cc432d56da8b61b070f4c7d2543a9ed17c2b23010b43af434fd40e2ca4aa)",
		},
		{
			imgo: fakeImageOperator{failToTag: true},
			want: `failed: tag "nginx:latest" as "123456789012.dkr.ecr.so-fake-2.amazonaws.com/sr:1611800397000000000-c5h66p35cpjmg"`,
		},
		{
			imgo: fakeImageOperator{failToUntag: true},
			want: "", // Untagging errors are ignored in current implementation.
		},
		{
			imgo: fakeImageOperator{failToPush: true},
			want: `failed: push "123456789012.dkr.ecr.so-fake-2.amazonaws.com/sr:1611800397000000000-c5h66p35cpjmg"`,
		},
	} {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			testRngReader = strings.NewReader("abcdefgh")
			err := PushImage(ctx, in, &test.ls, &test.imgo)
			if err == nil && test.want == "" {
				// succeeded as expected
				return
			}
			if err == nil {
				t.Error("unexpectedly succeeded")
				return
			}
			if err.Error() != test.want {
				t.Errorf("got: %v", err)
				t.Logf("want: %v", test.want)
			}
		})
	}
}

func ExamplePushImage() {
	defer func() {
		testNow, testRngReader = nil, nil
	}()
	testNow = func() time.Time { return time.Unix(1611796436, 0) }
	testRngReader = strings.NewReader("abcdefgh")

	ctx := context.Background()
	fls := &fakeLightsailImageOperator{}
	fimgo := &fakeImageOperator{}
	if err := PushImage(ctx, &PushImageInput{Service: "doge", Image: "nginx:latest", Label: "www"}, fls, fimgo); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("docker engine call log:")
	for _, s := range fimgo.log {
		fmt.Println(" ", s)
	}
	fmt.Println("lightsail api call log:")
	for _, s := range fls.log {
		fmt.Println(" ", s)
	}
	// Output:
	// Digest: sha256:10b8cc432d56da8b61b070f4c7d2543a9ed17c2b23010b43af434fd40e2ca4aa
	// Image "nginx:latest" registered.
	// Refer to this image as ":doge.www.12345" in deployments.
	// docker engine call log:
	//   tag "nginx:latest" as "123456789012.dkr.ecr.so-fake-2.amazonaws.com/sr:1611796436000000000-c5h66p35cpjmg"
	//   push "123456789012.dkr.ecr.so-fake-2.amazonaws.com/sr:1611796436000000000-c5h66p35cpjmg"
	//   untag "123456789012.dkr.ecr.so-fake-2.amazonaws.com/sr:1611796436000000000-c5h66p35cpjmg"
	// lightsail api call log:
	//   create login
	//   register (doge, www, sha256:10b8cc432d56da8b61b070f4c7d2543a9ed17c2b23010b43af434fd40e2ca4aa)
}

type fakeRegistryLoginCreator struct {
	failToCreateLogin bool
	log               []string
}

func (f *fakeRegistryLoginCreator) CreateContainerServiceRegistryLoginWithContext(
	context.Context,
	*lightsail.CreateContainerServiceRegistryLoginInput,
	...request.Option,
) (*lightsail.CreateContainerServiceRegistryLoginOutput, error) {
	op := "create login"
	if f.failToCreateLogin {
		return nil, fmt.Errorf("failed: %s", op)
	}
	f.log = append(f.log, op)
	return &lightsail.CreateContainerServiceRegistryLoginOutput{
		RegistryLogin: new(lightsail.ContainerServiceRegistryLogin).
			SetUsername("gollum").
			SetPassword("precious").
			SetRegistry("123456789012.dkr.ecr.so-fake-2.amazonaws.com"),
	}, nil
}

type fakeLightsailImageOperator struct {
	fakeRegistryLoginCreator
	failToRegister bool
}

func (f *fakeLightsailImageOperator) RegisterContainerImageWithContext(
	_ context.Context,
	in *lightsail.RegisterContainerImageInput,
	_ ...request.Option,
) (*lightsail.RegisterContainerImageOutput, error) {
	op := fmt.Sprintf("register (%s, %s, %s)",
		aws.StringValue(in.ServiceName),
		aws.StringValue(in.Label),
		aws.StringValue(in.Digest))
	if f.failToRegister {
		return nil, fmt.Errorf("failed: %s", op)
	}
	f.log = append(f.log, op)
	return &lightsail.RegisterContainerImageOutput{
		ContainerImage: &lightsail.ContainerImage{
			Digest: in.Digest,
			Image:  aws.String(":" + aws.StringValue(in.ServiceName) + "." + aws.StringValue(in.Label) + ".12345"),
		},
	}, nil
}

type fakeImageOperator struct {
	failToTag, failToUntag, failToPush bool
	log                                []string
}

func (f *fakeImageOperator) TagImage(_ context.Context, source, target string) error {
	op := fmt.Sprintf("tag %q as %q", source, target)
	if f.failToTag {
		return fmt.Errorf("failed: %s", op)
	}
	f.log = append(f.log, op)
	return nil
}

func (f *fakeImageOperator) UntagImage(_ context.Context, image string) error {
	op := fmt.Sprintf("untag %q", image)
	if f.failToUntag {
		return fmt.Errorf("failed: %s", op)
	}
	f.log = append(f.log, op)
	return nil
}

func (f *fakeImageOperator) PushImage(_ context.Context, remoteImage RemoteImage) (string, error) {
	op := fmt.Sprintf("push %q", remoteImage.Ref())
	if f.failToPush {
		return "", fmt.Errorf("failed: %s", op)
	}
	f.log = append(f.log, op)
	return "sha256:10b8cc432d56da8b61b070f4c7d2543a9ed17c2b23010b43af434fd40e2ca4aa", nil
}
