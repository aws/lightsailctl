package cs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/jsonmessage"

	"github.com/aws/lightsailctl/internal"
)

const exampleDigest = "sha256:cafe1234cafe1234cafe1234cafe1234cafe1234cafe1234abce5678cdef9012"

// fakeDockerClient implements dockerClient for testing.
type fakeDockerClient struct {
	// ImageInspectWithRaw
	inspectOS, inspectArch string
	inspectErr             error

	// ImageTag
	tagSource, tagTarget string
	tagErr               error

	// ImageRemove
	removeID  string
	removeErr error

	// ImagePush
	pushImage   string
	pushOptions image.PushOptions
	pushErr     error
	statusError string
	digestInAux bool

	// ServerVersion
	apiVersion string
	versionErr error
}

func (f *fakeDockerClient) ImageInspectWithRaw(_ context.Context, imageID string) (types.ImageInspect, []byte, error) {
	if f.inspectErr != nil {
		return types.ImageInspect{}, nil, f.inspectErr
	}
	return types.ImageInspect{Os: f.inspectOS, Architecture: f.inspectArch}, nil, nil
}

func (f *fakeDockerClient) ImageTag(_ context.Context, source, target string) error {
	f.tagSource, f.tagTarget = source, target
	return f.tagErr
}

func (f *fakeDockerClient) ImageRemove(_ context.Context, imageID string, _ image.RemoveOptions) ([]image.DeleteResponse, error) {
	f.removeID = imageID
	if f.removeErr != nil {
		return nil, f.removeErr
	}
	return []image.DeleteResponse{}, nil
}

func (f *fakeDockerClient) ImagePush(_ context.Context, img string, options image.PushOptions) (io.ReadCloser, error) {
	f.pushImage = img
	f.pushOptions = options
	if f.pushErr != nil {
		return nil, f.pushErr
	}

	badStatus := ""
	if f.statusError != "" {
		badStatus = fmt.Sprintf(`{"errorDetail":{"message":"%s"}}\n`, f.statusError)
	}

	digestStatus := fmt.Sprintf(`{"status":"whatever ... digest: %s ..."}`, exampleDigest)
	if f.digestInAux {
		digestStatus = fmt.Sprintf(`{"aux":{"digest":"%s"}}`, exampleDigest)
	}

	return io.NopCloser(strings.NewReader(strings.TrimSpace(fmt.Sprintf(`
{"status":"Waiting","progressDetail":{},"id":"d3a003bc9307"}
{"status":"Waiting","progressDetail":{},"id":"4f4fb700ef54"}
{"status":"Waiting","progressDetail":{},"id":"4056d2e51b69"}
{"status":"Layer already exists","progressDetail":{},"id":"d3a003bc9307"}
{"status":"Layer already exists","progressDetail":{},"id":"4f4fb700ef54"}
{"status":"Layer already exists","progressDetail":{},"id":"4056d2e51b69"}
%s%s
`, badStatus, digestStatus)))), nil
}

func (f *fakeDockerClient) ServerVersion(_ context.Context) (types.Version, error) {
	if f.versionErr != nil {
		return types.Version{}, f.versionErr
	}
	v := "1.45"
	if f.apiVersion != "" {
		v = f.apiVersion
	}
	return types.Version{APIVersion: v}, nil
}

func TestDockerEngineTag(t *testing.T) {
	ctx := context.Background()
	fc := &fakeDockerClient{}
	e := &DockerEngine{client: fc}

	err := e.TagImage(ctx, "httpd:latest", "example.com/httpd:latest")
	internal.AssertError(t, "", err)
	internal.Assert(t, "source", "httpd:latest", fc.tagSource)
	internal.Assert(t, "target", "example.com/httpd:latest", fc.tagTarget)

	fc.tagErr = io.EOF
	err = e.TagImage(ctx, "mcp:latest", "example.com/mcp:latest")
	internal.AssertError(t, "EOF", err)
	internal.Assert(t, "source", "mcp:latest", fc.tagSource)
	internal.Assert(t, "target", "example.com/mcp:latest", fc.tagTarget)
}

func TestDockerEngineUntag(t *testing.T) {
	ctx := context.Background()
	fc := &fakeDockerClient{}
	e := &DockerEngine{client: fc}

	err := e.UntagImage(ctx, "go:v1.25")
	internal.AssertError(t, "", err)
	internal.Assert(t, "imageID", "go:v1.25", fc.removeID)

	fc.removeErr = io.EOF
	err = e.UntagImage(ctx, "go:v1.23")
	internal.AssertError(t, "EOF", err)
	internal.Assert(t, "imageID", "go:v1.23", fc.removeID)
}

func TestDockerEnginePush(t *testing.T) {
	ctx := context.Background()
	remoteImage := RemoteImage{
		AuthConfig: registry.AuthConfig{
			Username: "user", Password: "42", ServerAddress: "example.com/httpd",
		},
		Tag: "v2.0.0",
	}

	for _, test := range []struct {
		name    string
		fc      fakeDockerClient
		wantErr string
	}{
		{
			name: "pushed ok",
		},
		{
			name: "pushed ok, digest in aux",
			fc:   fakeDockerClient{digestInAux: true},
		},
		{
			name:    "pushed with error",
			fc:      fakeDockerClient{pushErr: io.EOF},
			wantErr: "EOF",
		},
		{
			name:    "pushed with platform mismatch in status",
			fc:      fakeDockerClient{statusError: "... does not provide the specified platform (linux/amd64)"},
			wantErr: "image does not provide linux/amd64 platform",
		},
		{
			name:    "pushed with platform mismatch in error",
			fc:      fakeDockerClient{pushErr: errors.New("... does not match the specified platform (linux/amd64)")},
			wantErr: "image does not provide linux/amd64 platform",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fc := &test.fc
			e := &DockerEngine{client: fc}
			digest, err := e.PushImage(ctx, remoteImage)
			internal.AssertError(t, test.wantErr, err)
			if test.wantErr != "" {
				return
			}
			internal.Assert(t, "digest", exampleDigest, digest)
			internal.Assert(t, "image", "example.com/httpd:v2.0.0", fc.pushImage)
			expectedOptions := image.PushOptions{
				RegistryAuth: "eyJ1c2VybmFtZSI6InVzZXIiLCJwYXNzd29yZCI6IjQyIiwic2VydmVyYWRkcmVzcyI6ImV4YW1wbGUuY29tL2h0dHBkIn0=",
			}
			internal.Assert(t, "options", expectedOptions, fc.pushOptions)
		})
	}
}

func TestCheckPlatform(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name       string
		apiVersion string
		os         string
		arch       string
		fail       error
		wantErr    string
	}{
		{name: "old daemon, linux/amd64 ok", apiVersion: "1.45", os: "linux", arch: "amd64"},
		{name: "old daemon, linux/arm64 rejected", apiVersion: "1.45", os: "linux", arch: "arm64", wantErr: "image does not provide linux/amd64 platform"},
		{name: "old daemon, inspect error", apiVersion: "1.45", fail: io.EOF, wantErr: `inspect image "test:latest": EOF`},
		{name: "new daemon, arm64 allowed (push will select amd64 from manifest)", apiVersion: "1.46", os: "linux", arch: "arm64"},
		{name: "new daemon, skips inspect", apiVersion: "1.47", fail: io.EOF},
	} {
		t.Run(test.name, func(t *testing.T) {
			fc := &fakeDockerClient{apiVersion: test.apiVersion, inspectOS: test.os, inspectArch: test.arch, inspectErr: test.fail}
			e := &DockerEngine{client: fc}
			err := e.CheckPlatform(ctx, "test:latest")
			internal.AssertError(t, test.wantErr, err)
		})
	}
}

// Tests that digests from older Docker Engines are handled.
func TestExtractDigestFromAux(t *testing.T) {
	digest := ""
	badAux := json.RawMessage("42")
	extractDigestFromAux(&digest)(jsonmessage.JSONMessage{Aux: &badAux})
	if digest != "" {
		t.Errorf("unexpected got: %q", digest)
	}
	goodAux := json.RawMessage(`{"digest": "` + exampleDigest + `"}`)
	extractDigestFromAux(&digest)(jsonmessage.JSONMessage{Aux: &goodAux})
	if digest != exampleDigest {
		t.Errorf("got: %q", digest)
		t.Logf("want: %q", exampleDigest)
	}
}

func Example_scanStatuses() {
	digest := ""
	r := scanStatuses(
		&digest,
		strings.NewReader(fmt.Sprintf(`
		{"status": "keep me"}
		{"status": "xyz skip1 abc"}
		{"status": "also keep me!"}
		{"status": "\tskip2"}
		{"status": "... digest: %s ..."}`, exampleDigest)),
		"skip2", "skip1",
	)
	if _, err := io.Copy(os.Stdout, r); err != nil {
		fmt.Println(err)
		return
	}
	// Output:
	// {"status":"keep me"}
	// {"status":"also keep me!"}
	// {"status":"... digest: sha256:cafe1234cafe1234cafe1234cafe1234cafe1234cafe1234abce5678cdef9012 ..."}
}
