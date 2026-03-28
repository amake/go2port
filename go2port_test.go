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
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.16
	golang.org/x/tools/go/packages/packagestest v0.1.1-deprecated
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171
)
`)
	deps, err := readGoMod(goMod)
	if err != nil {
		t.Fatalf("readGoMod failed: %v", err)
	}

	out := goVendors(deps)

	expected := `go.vendors          sigs.k8s.io/structured-merge-diff/v6 \
                        repo    github.com/kubernetes-sigs/structured-merge-diff \
                        lock    v6.3.0 \
                        rmd160  edb2cdd7f5b865e96be7afe07ca55849b95b907b \
                        sha256  3ace9bd2a9cbac2df362d050f09801c42226425e09c0d0efbcb643e645844d14 \
                        size    257515 \
                    google.golang.org/genproto/googleapis/rpc \
                        repo    github.com/googleapis/go-genproto \
                        lock    a57be14db171 \
                        rmd160  53db77ff43de5bf4a76a4542eec698b7f8f74b95 \
                        sha256  8f002c05d8a36cbbe77fe8b648769fe53ad81f4df18e065e9da3128d374159f4 \
                        size    5932507 \
                    golang.org/x/tools/go/packages/packagestest \
                        lock    go/packages/packagestest/v0.1.1-deprecated \
                        rmd160  52166f3619102b9e082f1a153baefc1406910036 \
                        sha256  a77776ff38a217749e085be1548562fbced6d78196fced167967159589380516 \
                        size    8177810 \
                    go.yaml.in/yaml/v2 \
                        repo    github.com/yaml/go-yaml \
                        lock    v2.4.4 \
                        rmd160  34f7b53530e25a9329540afe8496e466cb1bd355 \
                        sha256  2f8f759505d5924915293b25c45cc80691c66ceaf4773c7f8dfbb23316991643 \
                        size    73836 \
                    github.com/mholt/acmez/v3 \
                        lock    v3.1.6 \
                        rmd160  3338ef993c24ea80118ad7171ade9167d67cb34c \
                        sha256  a35db0f698c5585d682a7ac18117d55c2d08680f8caca0612aa718fcd14f0e9c \
                        size    67607 \
                    github.com/charmbracelet/x/ansi \
                        lock    ansi/v0.11.6 \
                        rmd160  00a4a0fd678c64f7e0fc0d70de62594737aed28e \
                        sha256  920fd5e616a1749dbb80b483360db378ee9db96ed120900814ae963a4a259b05 \
                        size    518326 \
                    github.com/blang/semver \
                        lock    v3.5.1 \
                        rmd160  f3746971886e0aa556800bfd543d2f4a89a69767 \
                        sha256  5f5743805f1baf458ddf2dd8f49c553aa1f5c9667feadf357143602489d3587f \
                        size    14842 \
                    github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 \
                        lock    internal/endpoints/v2.7.16 \
                        rmd160  44c4f78b087850028401559761684b9d998b6822 \
                        sha256  93fdae24dd466cd59762a21edf1278d1e4614fdd9f56005ac076c0e84e4a0648 \
                        size    57434177 \
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
