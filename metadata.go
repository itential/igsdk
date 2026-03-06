// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import "fmt"

const (
	// Name is the SDK name.
	Name = "igsdk"

	// Author is the SDK author/maintainer.
	Author = "Itential"
)

// Build represents the Git SHA (short form) that the SDK was compiled from.
// This value is set at build time using linker flags:
//
//	-ldflags "-X github.com/itential/igsdk.Build=<commit-sha>"
var Build string

// Version represents the semantic version of the SDK.
// This value is set at build time using linker flags:
//
//	-ldflags "-X github.com/itential/igsdk.Version=<version>"
var Version = ""

// Info contains SDK metadata including name, version, author, and build information.
// Use GetInfo() to obtain an Info instance populated with current build metadata.
type Info struct {
	// Name is the SDK name (always "igsdk").
	Name string

	// Author is the SDK author/maintainer (always "Itential").
	Author string

	// Version is the semantic version string (e.g., "v1.2.3").
	// Empty string indicates a development build.
	Version string

	// Build is the Git commit SHA (short form) the SDK was built from.
	// Empty string indicates version information is not available.
	Build string
}

// GetInfo returns SDK information including name, version, author, and build details.
// The returned Info is populated with values from the Build and Version package variables,
// which are typically set at compile time via linker flags.
func GetInfo() Info {
	return Info{
		Name:    Name,
		Author:  Author,
		Version: Version,
		Build:   Build,
	}
}

// IsRelease returns true if the SDK is a release build.
// A release build is one where both Version and Build are set (non-empty).
// Returns false for development builds where version information is not available.
func (i Info) IsRelease() bool {
	return i.Version != "" && i.Build != ""
}

// ShortVersion returns just the version string without build information.
// Returns "devel" if version is not set.
func (i Info) ShortVersion() string {
	if i.Version == "" {
		return "devel"
	}
	return i.Version
}

// FullVersion returns a complete version string including build information.
// Format: "igsdk v1.2.3 (abc123)" for release builds.
// Format: "igsdk development" for development builds.
func (i Info) FullVersion() string {
	if i.IsRelease() {
		return fmt.Sprintf("%s %s (%s)", i.Name, i.Version, i.Build)
	}
	return fmt.Sprintf("%s %s", i.Name, i.ShortVersion())
}

// String returns a human-readable representation of the SDK info.
// This is equivalent to calling FullVersion().
func (i Info) String() string {
	return i.FullVersion()
}
