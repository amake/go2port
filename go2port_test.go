package main

import (
	"testing"
)

func TestGoMod(t *testing.T) {
	goMod := []byte(`
module example.com/foo

go 1.20

require (
	cloud.google.com/go/compute/metadata v0.9.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/charmbracelet/x/ansi v0.11.6
	github.com/mholt/acmez/v3 v3.1.6
	go.yaml.in/yaml/v2 v2.4.4
)
`)
	deps, err := readGoMod(goMod)
	if err != nil {
		t.Fatalf("readGoMod failed: %v", err)
	}

	out := goVendors(deps)

	expected := `go.vendors          go.yaml.in/yaml/v2 \
                        repo    github.com/yaml/go-yaml \
                        lock    v2.4.4 \
                        rmd160  34f7b53530e25a9329540afe8496e466cb1bd355 \
                        sha256  2f8f759505d5924915293b25c45cc80691c66ceaf4773c7f8dfbb23316991643 \
                        size    73836 \
                    github.com/mholt/acmez/v3 \
                        repo    github.com/mholt/acmez \
                        lock    v3.1.6 \
                        rmd160  3338ef993c24ea80118ad7171ade9167d67cb34c \
                        sha256  a35db0f698c5585d682a7ac18117d55c2d08680f8caca0612aa718fcd14f0e9c \
                        size    67607 \
                    github.com/charmbracelet/x/ansi \
                        repo    github.com/charmbracelet/x \
                        lock    ansi/v0.11.6 \
                        rmd160  00a4a0fd678c64f7e0fc0d70de62594737aed28e \
                        sha256  920fd5e616a1749dbb80b483360db378ee9db96ed120900814ae963a4a259b05 \
                        size    518326 \
                    github.com/blang/semver \
                        lock    v3.5.1 \
                        rmd160  f3746971886e0aa556800bfd543d2f4a89a69767 \
                        sha256  5f5743805f1baf458ddf2dd8f49c553aa1f5c9667feadf357143602489d3587f \
                        size    14842 \
                    cloud.google.com/go/compute/metadata \
                        repo    github.com/googleapis/google-cloud-go \
                        lock    compute/metadata/v0.9.0 \
                        rmd160  b9b22799973c4c72d9b2efe718a22b47644016fd \
                        sha256  5ee4c220bf5020bf72bfb4ffb8951db575c78c9621c0381b84dd34f68e6e650d \
                        size    38793782`

	if out != expected {
		t.Fatalf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", out, expected)
	}
}
