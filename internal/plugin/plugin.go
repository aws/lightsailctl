// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package plugin implements extensions for AWS CLI's lightsail subcommand.
// See: https://github.com/aws/aws-cli/tree/ce7dc9a61b/awscli/customizations/lightsail
package plugin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lightsail"
	"github.com/aws/lightsailctl/internal"
	"github.com/aws/lightsailctl/internal/cs"
	smithyMW "github.com/aws/smithy-go/middleware"
)

func Main(progname string, args []string) {
	input, inputStdin := "", false

	fs := flag.NewFlagSet(progname, flag.ExitOnError)

	const inputFlag = "input"
	fs.StringVar(&input, inputFlag, "", "plugin `payload`")

	const inputStdinFlag = "input-stdin"
	fs.BoolVar(&inputStdin, inputStdinFlag, false, "receive plugin payload on stdin")

	_ = fs.Parse(args)

	if input == "" && !inputStdin {
		fs.Usage()
		log.Fatalf("no plugin input: either %q or %q flag must be specified",
			fs.Lookup(inputFlag).Name,
			fs.Lookup(inputStdinFlag).Name)
	}

	var r io.Reader = strings.NewReader(input)
	if inputStdin {
		r = os.Stdin
	}

	in, err := parseInput(r)
	if err != nil {
		log.Fatalf("invalid plugin input: %v", err)
	}

	// This is a logger used for extra diagnostics, when the debugging mode is on.
	debugLog := log.New(log.Writer(), log.Prefix(), log.Flags())
	if !in.Configuration.Debug {
		debugLog.SetOutput(io.Discard)
	}

	if err := invokeOperation(context.Background(), in, debugLog); err != nil {
		log.Fatal(err)
	}
}

type Input struct {
	InputVersion  string          `json:"inputVersion"`
	Operation     string          `json:"operation"`
	Payload       json.RawMessage `json:"payload"`
	Configuration OperationConfig `json:"configuration"`
}

type OperationConfig struct {
	Debug          bool   `json:"debug,omitempty"`
	Endpoint       string `json:"endpoint,omitempty"`
	Region         string `json:"region,omitempty"`
	Profile        string `json:"profile,omitempty"`
	CABundle       string `json:"caBundle,omitempty"`
	DoNotVerifySSL bool   `json:"doNotVerifySSL,omitempty"`
	// CLIVersion is the version of the calling CLI,
	// for diagnostics and logging purposes.
	CLIVersion string `json:"cliVersion"`
}

func (c *OperationConfig) awsConfig(ctx context.Context) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	opts = append(opts, config.WithAPIOptions([]func(*smithyMW.Stack) error{
		middleware.AddUserAgentKeyValue("lightsailctl", internal.Version.String()),
	}))

	if c.Region != "" {
		opts = append(opts, config.WithRegion(c.Region))
	}

	if c.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(c.Profile))
	}

	if c.Debug {
		opts = append(opts, config.WithClientLogMode(aws.LogSigning|aws.LogRequestWithBody|aws.LogResponseWithBody))
	}

	if c.DoNotVerifySSL {
		opts = append(opts, config.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}))
	}

	if c.CABundle != "" {
		b, err := os.ReadFile(c.CABundle)
		if err != nil {
			return aws.Config{}, fmt.Errorf("read CA bundle file: %w", err)
		}
		opts = append(opts, config.WithCustomCABundle(bytes.NewReader(b)))
	}

	return config.LoadDefaultConfig(ctx, opts...)
}

func parseInput(r io.Reader) (*Input, error) {
	in := new(Input)
	if err := json.NewDecoder(r).Decode(in); err != nil {
		return nil, fmt.Errorf("unable to unmarshal JSON input: %v", err)
	}
	if ver, err := strconv.Atoi(in.InputVersion); err != nil || ver < 0 {
		return nil, fmt.Errorf("invalid inputVersion: it must contain a non-negative number")
	}
	return in, nil
}

func invokeOperation(ctx context.Context, in *Input, debugLog *log.Logger) error {
	switch in.Operation {
	case "PushContainerImage":
		cfg, err := in.Configuration.awsConfig(ctx)
		if err != nil {
			return err
		}

		ls := lightsail.NewFromConfig(cfg, func(o *lightsail.Options) {
			if ep := strings.TrimRight(in.Configuration.Endpoint, "/"); ep != "" {
				o.BaseEndpoint = &ep
			}
		})

		internal.CheckForUpdates(ctx, debugLog, ls, internal.Version)

		r, err := parsePushContainerImagePayload(in.Payload)
		if err != nil {
			return fmt.Errorf("unable to parse the input's payload field: %w", err)
		}

		dc, err := cs.NewDockerEngine(ctx)
		if err != nil {
			return err
		}

		if err := cs.PushImage(ctx, r, ls, dc); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown plugin operation: %q", in.Operation)
	}
	return nil
}

func parsePushContainerImagePayload(data json.RawMessage) (*cs.PushImageInput, error) {
	p := struct {
		Service string `json:"service"`
		Image   string `json:"image"`
		Label   string `json:"label"`
	}{}
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	for _, check := range []struct{ what, input string }{
		{"service name", p.Service},
		{"container image", p.Image},
		{"container label", p.Label},
	} {
		if len(check.input) != 0 {
			continue
		}
		return nil, fmt.Errorf("push container image: %s is not specified", check.what)
	}

	return &cs.PushImageInput{Service: p.Service, Image: p.Image, Label: p.Label}, nil
}
