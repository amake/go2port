package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersionPrefix(t *testing.T) {
	test_cases := map[string][]string{
		"v1.5.0":    {"v", "1.5.0"},
		"v1.0":      {"v", "1.0"},
		"2.5.0":     {"", "2.5.0"},
		"0.5-alpha": {"", "0.5-alpha"},
		"v1.0-pre":  {"v", "1.0-pre"},
	}

	for test_case, expected := range test_cases {

		expected_prefix := expected[0]
		expected_version := expected[1]

		prefix, version := parseVersionPrefix(test_case)

		assert.Equal(t, prefix, expected_prefix)
		assert.Equal(t, version, expected_version)
	}
}
