// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main is where lightsailctl command begins.
package main

import (
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/aws/lightsailctl/internal"
	"github.com/aws/lightsailctl/internal/plugin"
)

func main() {
	log.SetFlags(0)

	pluginPattern := regexp.MustCompile(`^--?plugin$`)
	getverPattern := regexp.MustCompile(`^--?version$`)

	switch {
	case len(os.Args) > 1 && pluginPattern.MatchString(os.Args[1]):
		pluginMain(os.Args[0]+" "+os.Args[1], os.Args[2:])
	case len(os.Args) > 1 && getverPattern.MatchString(os.Args[1]):
		fmt.Println(internal.Version)
	default:
		log.Fatalf("%s can't be used directly, it is meant to be invoked by AWS CLI", os.Args[0])
	}
}

// May be set by tests to something else.
var pluginMain = plugin.Main
