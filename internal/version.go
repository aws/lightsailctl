// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"strings"

	"golang.org/x/mod/semver"
)

const Version Semver = "v1.0.7-fix95fix100"

type Semver string

func (v Semver) IsValid() bool {
	return semver.IsValid(v.String())
}

func (v Semver) Less(other Semver) bool {
	return semver.Compare(v.String(), other.String()) < 0
}

func (v Semver) String() string {
	s := string(v)
	if s == "" || strings.HasPrefix(s, "v") {
		return semver.Canonical(s)
	}
	return semver.Canonical("v" + s)
}
