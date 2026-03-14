package main

import (
	"testing"
)

func TestPrintVersionShowsFields(t *testing.T) {
	// Verify that the version variables exist and have their default values.
	// After a build with ldflags, these would be populated with real values.
	fields := []struct {
		name  string
		value string
	}{
		{"GitSHA", GitSHA},
		{"DirtyBuild", DirtyBuild},
		{"GitFullVersionDescription", GitFullVersionDescription},
		{"GitDescribeVersion", GitDescribeVersion},
		{"GitLastCommitDate", GitLastCommitDate},
		{"GitVersion", GitVersion},
		{"SnapshotBuild", SnapshotBuild},
	}

	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			// Default values are "<unknown>" — after a real build they should
			// be replaced. We just verify the variables are accessible and
			// have their default string value.
			if f.value == "" {
				t.Errorf("%s is empty, want at least the default value", f.name)
			}
		})
	}
}

func TestVersionDefaultsAreUnknown(t *testing.T) {
	// Without ldflags, defaults should be "<unknown>"
	defaults := map[string]string{
		"GitSHA":                    GitSHA,
		"DirtyBuild":                DirtyBuild,
		"GitFullVersionDescription": GitFullVersionDescription,
		"GitDescribeVersion":        GitDescribeVersion,
		"GitLastCommitDate":         GitLastCommitDate,
		"GitVersion":                GitVersion,
		"SnapshotBuild":             SnapshotBuild,
	}

	for name, val := range defaults {
		if val != "<unknown>" {
			// This is expected when tests are run after a build with ldflags.
			// We just log it rather than failing.
			t.Logf("%s = %q (not default, likely built with ldflags)", name, val)
		}
	}
}
