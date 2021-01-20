// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/aws/lightsailctl/internal/plugin"
)

func setArgs(args []string) {
	os.Args = args
}

func setPluginMain(f func(string, []string)) {
	pluginMain = f
}

func TestMainCallsPluginMain(t *testing.T) {
	defer setArgs(os.Args)
	defer setPluginMain(plugin.Main)

	var gotProgname []string
	var gotArgs [][]string
	pluginMain = func(progname string, args []string) {
		gotProgname = append(gotProgname, progname)
		gotArgs = append(gotArgs, args)
	}

	os.Args = []string{"program", "-plugin", "--foo", "55"}
	main()
	os.Args = []string{"program", "--plugin", "--bar", "42"}
	main()

	if want := []string{"program -plugin", "program --plugin"}; !reflect.DeepEqual(gotProgname, want) {
		t.Errorf("got: %v", gotProgname)
		t.Logf("want: %v", want)
	}

	if want := [][]string{{"--foo", "55"}, {"--bar", "42"}}; !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("got: %v", gotArgs)
		t.Logf("want: %v", want)
	}
}
