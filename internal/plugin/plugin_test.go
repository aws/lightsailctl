// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/aws/lightsailctl/internal/cs"
)

func TestInputVersion(t *testing.T) {
	var tests = []struct {
		pass  bool
		input string
	}{
		{input: `{"inputVersion": ""}`},
		{input: `{"inputVersion": "v1"}`},
		{input: `{"inputVersion": "bogus"}`},
		{
			pass:  true,
			input: `{"inputVersion": "55"}`,
		},
	}

	for _, test := range tests {
		_, err := parseInput(strings.NewReader(test.input))
		if test.pass {
			if err != nil {
				t.Errorf("%s: %v", test.input, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: unexpected succeeded", test.input)
		}
	}

	f := func(i int) bool {
		input := fmt.Sprintf(`{"inputVersion": "%v"}`, i)
		parsed, err := parseInput(strings.NewReader(input))
		if i < 0 {
			return err != nil
		}
		if err != nil {
			return false
		}
		return parsed.InputVersion == strconv.Itoa(i)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestParseInput(t *testing.T) {
	input := `{
		"inputVersion":  "1",
		"operation":     "Whatever",
		"payload":       42,
		"configuration": {"region": "us-west-2", "cliVersion": "2.0.47"}
	}`

	got, err := parseInput(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	want := &Input{
		InputVersion:  "1",
		Operation:     "Whatever",
		Payload:       []byte{'4', '2'},
		Configuration: OperationConfig{Region: "us-west-2", CLIVersion: "2.0.47"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestParsePushContainerImagePayload(t *testing.T) {
	inputf := `{
		"inputVersion":  "1",
		"operation":     "PushContainerImage",
		"payload":       %s,
		"configuration": {"region": "us-west-2", "cliVersion": "2.0.47"}
	}`

	for i, test := range []struct {
		pass                 bool
		payload, errContains string
		want                 *cs.PushImageInput
	}{
		{
			payload:     `{"service": "dyservicev3", "image": "hello:latest"}`,
			errContains: "container label",
		},
		{
			payload:     `{"service": "dyservicev3", "label": "david16"}`,
			errContains: "container image",
		},
		{
			payload:     `{"image": "hello:latest", "label": "david16"}`,
			errContains: "service name",
		},
		{
			pass:    true,
			payload: `{"service": "dyservicev3", "image": "hello:latest", "label": "david16"}`,
			want:    &cs.PushImageInput{Service: "dyservicev3", Image: "hello:latest", Label: "david16"},
		},
	} {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			in, err := parseInput(strings.NewReader(fmt.Sprintf(inputf, test.payload)))
			if err != nil {
				t.Error(err)
				return
			}

			got, err := parsePushContainerImagePayload(in.Payload)
			if test.pass {
				if err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(got, test.want) {
					t.Errorf("got %#v, want %#v", got, test.want)
				}
				return
			}
			if err == nil {
				t.Error("unexpectedly succeeded")
				return
			}
			if !strings.Contains(err.Error(), test.errContains) {
				t.Errorf("got err: %v, that doesn't contain %q", err, test.errContains)
			}
		})
	}
}
