package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ripemd160"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
)

func main() {
	app := cli.NewApp()
	app.Name = "go2port"
	app.Usage = "Generate a MacPorts portfile from a Go project"
	app.Commands = []cli.Command{
		{
			Name:   "get",
			Usage:  "Generate a MacPorts portfile and output it to stdout",
			Action: generate,
		},
		{
			Name:   "update",
			Usage:  "Overwrite an existing MacPorts portfile",
			Action: update,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

var debug = false

var portfile = `# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4

PortSystem          1.0
PortGroup           golang 1.0

go.setup            {{.PackageId}} {{.Version}}
categories
maintainers
license

description

long_description

checksums           ${distname}${extract.suffix} \
                        rmd160  {{.Rmd160}} \
                        sha256  {{.Sha256}} \
                        size    {{.Size}}

{{.GoVendors}}

{{.DepChecksums}}
`

func generate(c *cli.Context) error {
	if c.NArg() != 2 {
		return cli.NewExitError("Please specify a package and version (tag or SHA1)", 1)
	}
	for i := 0; i < c.NArg(); i = i + 2 {
		pkgstr := c.Args().Get(i)
		version := c.Args().Get(i + 1)
		if debug {
			log.Printf("Generating portfile for %s (%s)", pkgstr, version)
		}
		pkg, err := newPackage(pkgstr, version)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		portfile, err := generateOne(pkg)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		fmt.Print(string(portfile))
	}
	return nil
}

func update(c *cli.Context) error {
	if c.NArg() != 2 {
		return cli.NewExitError("Please specify a package and version (tag or SHA1)", 1)
	}
	for i := 0; i < c.NArg(); i = i + 2 {
		pkgstr := c.Args().Get(i)
		version := c.Args().Get(i + 1)
		pkg, err := newPackage(pkgstr, version)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		out, err := exec.Command("port", "file", pkg.Project).Output()
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		outfile := strings.TrimSpace(string(out))
		log.Printf("Updating existing portfile: %s", outfile)
		portfile, err := generateOne(pkg)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		err = ioutil.WriteFile(outfile, portfile, 0755)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	return nil
}

type Package struct {
	Host    string
	Author  string
	Project string
	Id      string
	Version string
}

type Checksums struct {
	Rmd160 string
	Sha256 string
	Size   string
}

// This struct represents the main information we need about a dependency
// package. It is based on the glide.lock YAML definition, but with cajoling
// (tags) is able to work with the Gopkg.lock TOML definition as
// well. Supporting additional formats may require refactoring to funnel various
// format-specific structures into a single generic one.
type Dependency struct {
	Name    string
	Version string `toml:"revision"`
}

type GlideLock struct {
	Imports []Dependency
}

type GopkgLock struct {
	Projects []Dependency
}

func generateOne(pkg Package) ([]byte, error) {
	deps, err := dependencies(pkg)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	tplt := template.Must(template.New("portfile").Parse(portfile))

	csums, err := checksums(pkg)
	if err != nil {
		return nil, err
	}
	depcsums, err := depChecksums(deps)
	if err != nil {
		return nil, err
	}
	tvars := map[string]string{
		"PackageId":    pkg.Id,
		"Version":      pkg.Version,
		"Rmd160":       csums.Rmd160,
		"Sha256":       csums.Sha256,
		"Size":         csums.Size,
		"GoVendors":    goVendors(deps),
		"DepChecksums": depcsums,
	}

	err = tplt.Execute(&buf, tvars)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var verReg = regexp.MustCompile("\\..*$")

func newPackage(pkg string, version string) (Package, error) {
	parts := strings.Split(pkg, "/")
	ret := Package{
		Host:    parts[0],
		Id:      pkg,
		Version: version,
	}
	switch parts[0] {
	case "github.com":
		ret.Author = parts[1]
		ret.Project = parts[2]
	case "golang.org":
		ret.Author = "golang"
		ret.Project = parts[2]
	case "gopkg.in":
		switch len(parts) {
		case 2:
			ret.Project = verReg.ReplaceAllString(parts[1], "")
			ret.Author = "go-" + ret.Project
		case 3:
			ret.Project = verReg.ReplaceAllString(parts[2], "")
			ret.Author = parts[1]
		}
	default:
		return ret, errors.New("Unknown domain: " + parts[0])
	}
	return ret, nil
}

func dependencies(pkg Package) ([]Dependency, error) {
	deps, err := glideDependencies(pkg)
	if err == nil {
		return deps, nil
	}
	deps, err = gopkgDependencies(pkg)
	if err == nil {
		return deps, nil
	}
	return nil, err
}

func rawFileUrl(pkg Package, file string) (string, error) {
	switch pkg.Host {
	case "github.com":
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
			pkg.Author, pkg.Project, pkg.Version, file), nil
	case "bitbucket.org":
		return fmt.Sprintf("https://bitbucket.org/%s/%s/raw/%s/%s",
			pkg.Author, pkg.Project, pkg.Version, file), nil
	default:
		return "", errors.New(fmt.Sprintf("Unsupported domain: %s", pkg.Host))
	}
}

func glideDependencies(pkg Package) ([]Dependency, error) {
	lockUrl, err := rawFileUrl(pkg, "glide.lock")
	if err != nil {
		return nil, err
	}
	res, err := http.Get(lockUrl)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		msg := fmt.Sprintf("glide.lock not available; HTTP status=%d", res.StatusCode)
		return nil, errors.New(msg)
	}
	lockBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	lock := GlideLock{}
	err = yaml.Unmarshal(lockBytes, &lock)
	if err != nil {
		return nil, err
	}
	return lock.Imports, nil
}

func gopkgDependencies(pkg Package) ([]Dependency, error) {
	lockUrl, err := rawFileUrl(pkg, "Gopkg.lock")
	if err != nil {
		return nil, err
	}
	res, err := http.Get(lockUrl)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		msg := fmt.Sprintf("Gopkg.lock not available; HTTP status=%d", res.StatusCode)
		return nil, errors.New(msg)
	}
	lockBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	lock := GopkgLock{}
	err = toml.Unmarshal(lockBytes, &lock)
	if err != nil {
		return nil, err
	}
	return lock.Projects, nil
}

func goVendors(deps []Dependency) string {
	if len(deps) == 0 {
		return ""
	}
	ret := "go.vendors          "
	for i, dep := range deps {
		ret = ret + dep.Name + " " + dep.Version
		if i < len(deps)-1 {
			ret = ret + " \\\n" + strings.Repeat(" ", 20)
		}
	}
	return ret
}

func tarballUrl(pkg Package) (string, error) {
	switch pkg.Host {
	case "github.com":
		return fmt.Sprintf("https://github.com/%s/%s/tarball/%s",
			pkg.Author, pkg.Project, pkg.Version), nil
	case "bitbucket.org":
		return fmt.Sprintf("https://bitbucket.org/%s/%s/get/%s.tar.gz",
			pkg.Author, pkg.Project, pkg.Version), nil
	default:
		return "", errors.New(fmt.Sprintf("Unsupported domain: %s", pkg.Host))
	}
}

func checksums(pkg Package) (Checksums, error) {
	ret := Checksums{
		Rmd160: "0",
		Sha256: "0",
		Size:   "0",
	}
	tarUrl, err := tarballUrl(pkg)
	if err != nil {
		return ret, err
	}
	res, err := http.Get(tarUrl)
	if err != nil {
		return ret, err
	}
	tarball, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return ret, err
	}

	ret.Size = fmt.Sprintf("%d", len(tarball))

	sha := sha256.New()
	sha.Write(tarball)
	ret.Sha256 = fmt.Sprintf("%x", sha.Sum(nil))

	rmd := ripemd160.New()
	rmd.Write(tarball)
	ret.Rmd160 = fmt.Sprintf("%x", rmd.Sum(nil))

	return ret, nil
}

func depChecksums(deps []Dependency) (string, error) {
	if len(deps) == 0 {
		return "", nil
	}
	ret := "checksums-append    "
	for i, dep := range deps {
		pkg, err := newPackage(dep.Name, dep.Version)
		if err != nil {
			return "", err
		}
		csums, err := checksums(pkg)
		if err != nil {
			return "", err
		}
		chk := fmt.Sprintf("${%s.distfile} \\\n", pkg.Project)
		chk = chk + fmt.Sprintf("%srmd160 %s \\\n", strings.Repeat(" ", 24), csums.Rmd160)
		chk = chk + fmt.Sprintf("%ssha256 %s \\\n", strings.Repeat(" ", 24), csums.Sha256)
		chk = chk + fmt.Sprintf("%ssize %s", strings.Repeat(" ", 24), csums.Size)
		if i < len(deps)-1 {
			chk = chk + " \\\n" + strings.Repeat(" ", 20)
		}
		ret = ret + chk
	}
	return ret, nil
}
