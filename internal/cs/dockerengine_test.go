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

	"github.com/aws/lightsailctl/internal"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/jsonmessage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const exampleDigest = "sha256:cafe1234cafe1234cafe1234cafe1234cafe1234cafe1234abce5678cdef9012"

type fakeImageTagger struct {
	gotSource, gotTarget string
	fail                 error
}

func (f *fakeImageTagger) ImageTag(_ context.Context, source, target string) error {
	f.gotSource, f.gotTarget = source, target
	return f.fail
}

type fakeImageRemover struct {
	gotImageID string
	fail       error
}

func (f *fakeImageRemover) ImageRemove(
	_ context.Context, imageID string, _ image.RemoveOptions,
) ([]image.DeleteResponse, error) {
	f.gotImageID = imageID
	if f.fail != nil {
		return nil, f.fail
	}
	return []image.DeleteResponse{}, nil
}

type fakeImagePusher struct {
	gotImage    string
	gotOptions  image.PushOptions
	fail        error
	statusError string
	digestInAux bool
}

func (f *fakeImagePusher) ImagePush(
	_ context.Context, image string, options image.PushOptions,
) (io.ReadCloser, error) {
	f.gotImage = image
	f.gotOptions = options

	if f.fail != nil {
		return nil, f.fail
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

func TestDockerEngine(t *testing.T) {
	ctx := context.Background()

	tagger := &fakeImageTagger{}

	e := &DockerEngine{it: tagger}

	err := e.TagImage(ctx, "httpd:latest", "example.com/httpd:latest")
	internal.AssertError(t, "", err)
	internal.Assert(t, "source", "httpd:latest", tagger.gotSource)
	internal.Assert(t, "target", "example.com/httpd:latest", tagger.gotTarget)

	tagger.fail = io.EOF
	err = e.TagImage(ctx, "mcp:latest", "example.com/mcp:latest")
	internal.AssertError(t, "EOF", err)
	internal.Assert(t, "source", "mcp:latest", tagger.gotSource)
	internal.Assert(t, "target", "example.com/mcp:latest", tagger.gotTarget)

	remover := &fakeImageRemover{}
	e = &DockerEngine{ir: remover}
	err = e.UntagImage(ctx, "go:v1.25")
	internal.AssertError(t, "", err)
	internal.Assert(t, "imageID", "go:v1.25", remover.gotImageID)

	remover.fail = io.EOF
	err = e.UntagImage(ctx, "go:v1.23")
	internal.AssertError(t, "EOF", err)
	internal.Assert(t, "imageID", "go:v1.23", remover.gotImageID)

	for _, test := range []struct {
		name    string
		pusher  fakeImagePusher
		wantErr string
	}{
		{
			name: "pushed ok",
		},
		{
			name:   "pushed ok, digest in aux",
			pusher: fakeImagePusher{digestInAux: true},
		},
		{
			name:    "pushed with error",
			pusher:  fakeImagePusher{fail: io.EOF},
			wantErr: "EOF",
		},
		{
			name:    "pushed with platform mismatch in status",
			pusher:  fakeImagePusher{statusError: "... does not provide the specified platform (linux/amd64)"},
			wantErr: "image does not provide linux/amd64 platform",
		},
		{
			name:    "pushed with platform mismatch in error",
			pusher:  fakeImagePusher{fail: errors.New("... does not match the specified platform (linux/amd64)")},
			wantErr: "image does not provide linux/amd64 platform",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := &DockerEngine{ip: &test.pusher}
			digest, err := e.PushImage(ctx, RemoteImage{
				AuthConfig: registry.AuthConfig{
					Username: "user", Password: "42", ServerAddress: "example.com/httpd",
				},
				Tag: "v2.0.0",
			})
			internal.AssertError(t, test.wantErr, err)
			if test.wantErr != "" {
				return
			}
			internal.Assert(t, "digest", exampleDigest, digest)
			internal.Assert(t, "image", "example.com/httpd:v2.0.0", test.pusher.gotImage)
			internal.Assert(t, "options", image.PushOptions{
				// This is just base64 encoding of the auth config, provided above.
				RegistryAuth: "eyJ1c2VybmFtZSI6InVzZXIiLCJwYXNzd29yZCI6IjQyIiwic2VydmVyYWRkcmVzcyI6ImV4YW1wbGUuY29tL2h0dHBkIn0=",
				Platform:     &ocispec.Platform{OS: "linux", Architecture: "amd64"},
			}, test.pusher.gotOptions)
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
