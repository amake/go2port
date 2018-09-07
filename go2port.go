package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
)

func main() {
	app := cli.NewApp()
	app.Name = "go2port"
	app.Usage = "Generate a MacPorts portfile from a Go project"
	app.Action = generate

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

var debug = false

var portfile = `# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4

PortSystem          1.0
PortGroup           github 1.0
PortGroup           golang 1.0

github.setup        {{.Author}} {{.Project}} {{.Version}}
name                {{.Project}}
categories
platforms           darwin
maintainers
license

description

long_description

checksums           {{.Project}}-${github.version}.tar.gz \
                        rmd160  {{.Rmd160}} \
                        sha256  {{.Sha256}} \
                        size    {{.Size}}

{{.GoVendors}}

{{.Checksums}}
`

func generate(c *cli.Context) error {
	if c.NArg() != 2 {
		return cli.NewExitError("Please specify a package and version (tag or SHA1)", 1)
	}
	for i := 0; i < c.NArg(); i = i + 2 {
		pkgstr := c.Args().Get(i)
		version := c.Args().Get(i + 1)
		if debug {
			log.Printf("Generating portfile for %q (%q)", pkgstr, version)
		}
		pkg, err := splitPackage(pkgstr)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		err = generateOne(pkg, version)
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
}

type Dependency struct {
	Name    string
	Version string
}

type GlideLock struct {
	Imports []Dependency
}

func generateOne(pkg Package, version string) error {
	deps, err := dependencies(pkg, version)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	tplt := template.Must(template.New("portfile").Parse(portfile))

	csums, err := checksums(deps)
	if err != nil {
		return err
	}
	tvars := map[string]string{
		"Author":    pkg.Author,
		"Project":   pkg.Project,
		"Version":   version,
		"Rmd160":    "0",
		"Sha256":    "0",
		"Size":      "0",
		"GoVendors": goVendors(deps),
		"Checksums": csums,
	}

	err = tplt.Execute(&buf, tvars)
	if err != nil {
		return err
	}
	fmt.Print(buf.String())
	return nil
}

var verReg = regexp.MustCompile("\\..*$")

func splitPackage(pkg string) (Package, error) {
	parts := strings.Split(pkg, "/")
	ret := Package{
		Host: parts[0],
		Id:   pkg,
	}
	switch parts[0] {
	case "github.com":
		ret.Author = parts[1]
		ret.Project = parts[2]
	case "golang.org":
		ret.Author = "golang"
		ret.Project = parts[1]
	case "gopkg.in":
		switch len(parts) {
		case 2:
			ret.Project = verReg.ReplaceAllString(parts[1], "")
			ret.Author = "go-" + ret.Project
		}
	default:
		return ret, errors.New("Unknown domain: " + parts[0])
	}
	return ret, nil
}

func dependencies(pkg Package, version string) ([]Dependency, error) {
	lockUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/glide.lock",
		pkg.Author, pkg.Project, version)
	res, err := http.Get(lockUrl)
	if err != nil {
		return nil, err
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

func goVendors(deps []Dependency) string {
	if len(deps) == 0 {
		return ""
	}
	ret := "go.vendors-append   "
	for i, dep := range deps {
		ret = ret + dep.Name + " " + dep.Version
		if i < len(deps)-1 {
			ret = ret + " \\\n" + strings.Repeat(" ", 20)
		}
	}
	return ret
}

func checksums(deps []Dependency) (string, error) {
	if len(deps) == 0 {
		return "", nil
	}
	ret := "checksums-append    "
	for i, dep := range deps {
		pkg, err := splitPackage(dep.Name)
		if err != nil {
			return "", err
		}
		chk := fmt.Sprintf("%[1]s-%[2]s-${%[2]s.version}.tar.gz \\\n", pkg.Author, pkg.Project)
		chk = chk + strings.Repeat(" ", 24) + "rmd160 0 \\\n"
		chk = chk + strings.Repeat(" ", 24) + "sha256 0 \\\n"
		chk = chk + strings.Repeat(" ", 24) + "size 0"
		if i < len(deps)-1 {
			chk = chk + " \\\n" + strings.Repeat(" ", 20)
		}
		ret = ret + chk
	}
	return ret, nil
}
