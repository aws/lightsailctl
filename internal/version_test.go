// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal_test

import (
	"testing"

	"github.com/aws/lightsailctl/internal"
)

func TestVersionGlobalIsValid(t *testing.T) {
	if !internal.Version.IsValid() {
		t.Errorf("internal.Version value %q is not a valid semver",
			string(internal.Version))
	}
}
