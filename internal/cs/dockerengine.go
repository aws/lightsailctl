// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/term"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type platformError struct {
	wantPlatform *ocispec.Platform
	cause        error
}

func (pe *platformError) Error() string {
	return fmt.Sprintf("image does not provide %s/%s platform",
		pe.wantPlatform.OS, pe.wantPlatform.Architecture)
}

func (pe *platformError) Unwrap() error { return pe.cause }

// DockerEngine defines a subset of client-side
// operations against local Docker Engine, relevant to lightsailctl.
type DockerEngine struct {
	it interface {
		ImageTag(ctx context.Context, source, target string) error
	}
	ir interface {
		ImageRemove(
			ctx context.Context, imageID string, options image.RemoveOptions,
		) ([]image.DeleteResponse, error)
	}
	ip interface {
		ImagePush(
			ctx context.Context, image string, options image.PushOptions,
		) (io.ReadCloser, error)
	}
}

// RemoteImage combines remote server auth details, address
// and an image tag into a value that has everything that
// one needs to push this image to a remote repo.
type RemoteImage struct {
	registry.AuthConfig
	Tag string
}

func (r *RemoteImage) Ref() string {
	return r.ServerAddress + ":" + r.Tag
}

func NewDockerEngine(ctx context.Context) (*DockerEngine, error) {
	dc, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	dc.NegotiateAPIVersion(ctx)
	return &DockerEngine{it: dc, ir: dc, ip: dc}, nil
}

func (e *DockerEngine) TagImage(ctx context.Context, source, target string) error {
	return e.it.ImageTag(ctx, source, target)
}

func (e *DockerEngine) UntagImage(ctx context.Context, imageID string) error {
	_, err := e.ir.ImageRemove(ctx, imageID, image.RemoveOptions{})
	return err
}

func (e *DockerEngine) PushImage(ctx context.Context, remoteImage RemoteImage) (string, error) {
	authBytes, err := json.Marshal(remoteImage.AuthConfig)
	if err != nil {
		return "", err
	}

	platform := &ocispec.Platform{OS: "linux", Architecture: "amd64"}
	pushOutput, err := e.ip.ImagePush(ctx, remoteImage.Ref(), image.PushOptions{
		RegistryAuth: base64.URLEncoding.EncodeToString(authBytes),
		Platform:     platform,
	})
	if err != nil {
		if platformErrorRE(platform).MatchString(err.Error()) {
			return "", &platformError{wantPlatform: platform, cause: err}
		}
		return "", err
	}
	defer pushOutput.Close()

	var digestFromStatus, digestFromAux string
	termFd, isTerm := term.GetFdInfo(os.Stderr)
	if err = jsonmessage.DisplayJSONMessagesStream(
		scanStatuses(&digestFromStatus, pushOutput,
			remoteImage.ServerAddress, remoteImage.Tag),
		os.Stderr, termFd, isTerm,
		extractDigestFromAux(&digestFromAux),
	); err != nil {
		var je *jsonmessage.JSONError
		if errors.As(err, &je) {
			if platformErrorRE(platform).MatchString(je.Message) {
				return "", &platformError{wantPlatform: platform, cause: err}
			}
		}
		return "", err
	}

	switch {
	case digestFromStatus != "":
		// Got digest from a newer Docker Engine.
		return digestFromStatus, nil
	case digestFromAux != "":
		// Got digest from an older Docker Engine.
		return digestFromAux, nil
	}
	return "", errors.New("image push response does not contain the image digest")
}

// scanStatuses omits statuses that have irrelevant details such as repo
// address, and intercepts the image digest, if any, that newer Docker Engines
// emit.
func scanStatuses(digest *string, input io.Reader, skips ...string) io.Reader {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		dec := json.NewDecoder(input)
		enc := json.NewEncoder(w)
	InputLoop:
		for {
			m := jsonmessage.JSONMessage{}
			if err := dec.Decode(&m); err != nil {
				if !errors.Is(err, io.EOF) && err.Error() != "http: read on closed response body" {
					log.Printf("scanStatuses: %v", err)
				}
				break
			}
			// Newer Docker Engines emit an image digest via message status
			// field.
			if match := digestStatusRE.FindStringSubmatch(m.Status); len(match) == 2 {
				*digest = match[1]
			}
			for _, skip := range skips {
				if strings.Contains(m.Status, skip) {
					continue InputLoop
				}
			}
			if err := enc.Encode(m); err != nil {
				log.Printf("scanStatuses: %v", err)
			}
		}
	}()
	return r
}

// extractDigestFromAux attempts to extract an image digest emitted by older
// Docker Engines.
func extractDigestFromAux(digest *string) func(jsonmessage.JSONMessage) {
	return func(m jsonmessage.JSONMessage) {
		aux := struct{ Digest string }{}
		if err := json.Unmarshal(*m.Aux, &aux); err != nil {
			log.Printf("extractDigest: %v", err)
			return
		}
		*digest = aux.Digest
	}
}

func platformErrorRE(platform *ocispec.Platform) *regexp.Regexp {
	return regexp.MustCompile(`does not (provide|match) the specified platform ` +
		regexp.QuoteMeta(fmt.Sprintf("(%s/%s)", platform.OS, platform.Architecture)))
}

// In newer Docker Engines, the last status message contains the image digest,
// and it looks like this:
// "... digest: sha256:cafe1234cafe1234cafe1234cafe1234cafe1234cafe1234abce5678cdef9012 size: 1819"
var digestStatusRE = regexp.MustCompile(`digest: (sha256:[a-f0-9]{64})`)
