// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/term"
)

// DockerEngine defines a subset of client-side
// operations against local Docker Engine, relevant to lightsailctl.
type DockerEngine struct {
	c *client.Client
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
	return &DockerEngine{c: dc}, nil
}

func (e *DockerEngine) TagImage(ctx context.Context, source, target string) error {
	return e.c.ImageTag(ctx, source, target)
}

func (e *DockerEngine) UntagImage(ctx context.Context, image string) error {
	_, err := e.c.ImageRemove(ctx, image, types.ImageRemoveOptions{})
	return err
}

func (e *DockerEngine) PushImage(ctx context.Context, remoteImage RemoteImage) (digest string, err error) {
	authBytes, err := json.Marshal(remoteImage.AuthConfig)
	if err != nil {
		return "", err
	}
	pushRes, err := e.c.ImagePush(ctx, remoteImage.Ref(), types.ImagePushOptions{
		RegistryAuth: base64.URLEncoding.EncodeToString(authBytes),
	})
	if err != nil {
		return "", err
	}
	defer pushRes.Close()

	termFd, isTerm := term.GetFdInfo(os.Stderr)
	if err = jsonmessage.DisplayJSONMessagesStream(
		// Skip statuses that have irrelevant details such as repo address.
		skipStatuses(pushRes, remoteImage.ServerAddress, remoteImage.Tag),
		os.Stderr, termFd, isTerm,
		extractDigest(&digest)); err != nil {
		return "", err
	}
	if digest == "" {
		return "", errors.New("image push response does not contain the image digest")
	}
	return digest, nil
}

func skipStatuses(input io.Reader, s ...string) io.Reader {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		dec := json.NewDecoder(input)
		enc := json.NewEncoder(w)
	InputLoop:
		for {
			m := jsonmessage.JSONMessage{}
			if err := dec.Decode(&m); err != nil {
				if err != io.EOF {
					log.Printf("skipStatuses: %v", err)
				}
				break
			}
			for _, skip := range s {
				if strings.Contains(m.Status, skip) {
					continue InputLoop
				}
			}
			if err := enc.Encode(m); err != nil {
				log.Printf("skipStatuses: %v", err)
			}
		}
	}()
	return r
}

func extractDigest(p *string) func(jsonmessage.JSONMessage) {
	return func(m jsonmessage.JSONMessage) {
		aux := struct{ Digest string }{}
		if err := json.Unmarshal(*m.Aux, &aux); err != nil {
			log.Printf("extractDigest: %v", err)
			return
		}
		*p = aux.Digest
	}
}
