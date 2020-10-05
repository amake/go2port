package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v2"
)

// Build with -ldflags "-X main.version=$VERSION" to overwrite
var version = "dev"

func main() {

	// Don't prefix log lines with time
	log.SetFlags(0)

	app := cli.NewApp()
	app.Name = "go2port"
	app.Usage = "Generate a MacPorts portfile from a Go project"
	app.Version = version
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug, d",
			Usage:       "print debug information",
			Destination: &debugOn,
		},
	}
	app.Commands = []cli.Command{
		{
			Name:      "get",
			Usage:     "Generate a MacPorts portfile and output it to stdout",
			ArgsUsage: "<package> <version> ...",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "output, o",
					Usage: "output `FILE` (\"-\" for stdout)",
					Value: "-",
				},
			},

			Action: generate,
		},
		{
			Name:      "update",
			Usage:     "Overwrite an existing MacPorts portfile",
			ArgsUsage: "<portname> <version> ...",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "output, o",
					Usage: "output `FILE` (\"-\" for stdout)",
				},
			},
			Action: update,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

var debugOn = false

var portfileTemplate = `# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4

PortSystem          1.0
PortGroup           golang 1.0

go.setup            {{.PackageId}} {{.Version}}
categories
maintainers
license

description

long_description

{{.PackageAlias}}{{.Checksums}}

{{.GoVendors}}

destroot {
    xinstall -m 0755 ${worksrcpath}/${name} ${destroot}${prefix}/bin/
}
`

func generate(c *cli.Context) error {
	if c.NArg()%2 != 0 {
		return cli.NewExitError("Please specify a package and version (tag or SHA1)", 1)
	}
	outfile := c.String("output")
	if c.NArg() > 2 && outfile != "-" && outfile != "" {
		log.Println("WARNING: Output file ignored in batch mode")
		outfile = ""
	}
	for i := 0; i < c.NArg(); i = i + 2 {
		pkgstr := c.Args().Get(i)
		version := c.Args().Get(i + 1)
		if debugOn {
			log.Printf("Generating portfile for %s (%s)", pkgstr, version)
		}
		pkg, err := newPackage(pkgstr, version)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		portfile, err := generateOne(pkg, portfileTemplate)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		if outfile == "-" {
			_, err = fmt.Print(string(portfile))
		} else {
			err = ioutil.WriteFile(outfile, portfile, 0755)
		}
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	return nil
}

func update(c *cli.Context) error {
	if c.NArg()%2 != 0 {
		return cli.NewExitError("Please specify a package and version (tag or SHA1)", 1)
	}
	outfile := c.String("output")
	if c.NArg() > 2 && outfile != "-" && outfile != "" {
		log.Println("WARNING: Output file ignored in batch mode")
		outfile = ""
	}
	for i := 0; i < c.NArg(); i = i + 2 {
		portname := c.Args().Get(i)
		version := c.Args().Get(i + 1)
		err := updateOne(portname, version, outfile)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	return nil
}

func getPortfilePath(portname string) (string, error) {
	cmd := exec.Command("port", "file", portname)
	var (
		stdout, stderr bytes.Buffer
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Print(stderr.String())
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func updateOne(portname string, version string, outfile string) error {
	toStdOut := outfile == "-"
	portfilePath, err := getPortfilePath(portname)
	if err != nil {
		return err
	}
	portfileOld, err := ioutil.ReadFile(portfilePath)
	if err != nil {
		return err
	}
	portfileOldStr := string(portfileOld)
	pkgstr, err := packageFromPortfile(portfileOldStr)
	if pkgstr == "" {
		msg := fmt.Sprintf("Could not detect Go package from portfile %s", portfilePath)
		return errors.New(msg)
	}
	pkg, err := newPackage(pkgstr, version)
	if err != nil {
		return err
	}
	tmplate, err := templateFromPortfile(pkg, portfileOldStr)
	if err != nil {
		return err
	}
	if outfile == "" {
		outfile = portfilePath
	}
	if debugOn {
		log.Printf("Generated template from existing portfile:\n%s", tmplate)
	}
	if !toStdOut {
		log.Printf("Updating existing portfile: %s", portfilePath)
	}
	portfileNew, err := generateOne(pkg, tmplate)
	if err != nil {
		return err
	}
	if toStdOut {
		_, err = fmt.Print(string(portfileNew))
	} else {
		err = ioutil.WriteFile(outfile, portfileNew, 0755)
	}
	if err != nil {
		return err
	}
	return nil
}

var setupPkgRegexp = regexp.MustCompile("go.setup\\s+(\\S+)")

func packageFromPortfile(portfile string) (string, error) {
	match := setupPkgRegexp.FindStringSubmatch(portfile)
	if len(match) < 2 {
		return "", errors.New("Could not detect package name in portfile")
	}
	return match[1], nil
}

var checksumsPattern = regexp.MustCompile("checksums(?:.*\\\\\n)*.*")
var goVendorsPattern = regexp.MustCompile("go\\.vendors(?:.*\\\\\n)*.*")

func templateFromPortfile(pkg Package, portfile string) (string, error) {
	setupRegexp := fmt.Sprintf("(?P<before>go.setup\\s+%s\\s+)\\S+(?P<after>.*)", pkg.Id)
	setupPattern, err := regexp.Compile(setupRegexp)
	if err != nil {
		return "", err
	}
	portfile = setupPattern.ReplaceAllString(portfile, "$before{{.Version}}$after")
	portfile = goVendorsPattern.ReplaceAllString(portfile, "{{.GoVendors}}")
	portfile = checksumsPattern.ReplaceAllString(portfile, "{{.Checksums}}")
	return portfile, nil
}

type Package struct {
	Host    string
	Author  string
	Project string
	Id      string
	Alias   string
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

func generateOne(pkg Package, tmplate string) ([]byte, error) {
	deps, err := dependencies(pkg)
	if debugOn && err != nil {
		msg := fmt.Sprintf("Could not retrieve dependencies for package: %s", pkg.Id)
		log.Println(msg)
		log.Println(err)
	}

	var buf bytes.Buffer
	tplt := template.Must(template.New("portfile").Parse(tmplate))

	tvars := map[string]string{
		"PackageId":    pkg.Id,
		"PackageAlias": packageAlias(pkg),
		"Version":      pkg.Version,
		"Checksums":    checksumsStr(pkg, len(deps)),
		"GoVendors":    goVendors(deps),
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
	case "golang.org":
		if len(parts) < 3 {
			return ret, errors.New(fmt.Sprintf("Invalid package ID: %s", pkg))
		}
		// Use GitHub mirror
		ret.Host = "github.com"
		ret.Author = "golang"
		ret.Project = parts[2]
	case "gopkg.in":
		// gopkg.in redirects to GitHub
		ret.Host = "github.com"
		switch len(parts) {
		case 2:
			// Short format: gopkg.in/foo.v1 -> github.com/go-foo/foo
			ret.Project = verReg.ReplaceAllString(parts[1], "")
			ret.Author = "go-" + ret.Project
		case 3:
			// Long format: gopkg.in/foo/bar.v1 -> github.com/foo/bar
			ret.Project = verReg.ReplaceAllString(parts[2], "")
			ret.Author = parts[1]
		default:
			return ret, errors.New(fmt.Sprintf("Invalid package ID: %s", pkg))
		}
	case "bitbucket.org":
		fallthrough
	case "github.com":
		if len(parts) < 3 {
			return ret, errors.New(fmt.Sprintf("Invalid package ID: %s", pkg))
		}
		ret.Author = parts[1]
		ret.Project = parts[2]
	default:
		parts, err := resolvePackage(pkg)
		if err != nil {
			return ret, err
		}
		ret.Id = strings.Join(parts, "/")
		ret.Host = parts[0]
		// TODO: What if there's really more than 3?
		if len(parts) >= 3 {
			ret.Author = parts[1]
			ret.Project = parts[2]
		} else if len(parts) == 2 {
			ret.Project = parts[1]
		} else {
			return ret, errors.New(fmt.Sprintf("Too few parts: %s", parts))
		}
		ret.Alias = pkg
	}
	return ret, nil
}

func resolvePackage(pkg string) ([]string, error) {
	res, err := http.Get("https://" + pkg + "?go-get=1")
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	var parts []string
	var f func(*html.Node) bool
	f = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "meta" {
			isGoImport := false
			content := ""
			for _, a := range n.Attr {
				if a.Key == "name" && a.Val == "go-import" {
					isGoImport = true
				} else if a.Key == "content" {
					content = a.Val
				}
			}
			if !isGoImport {
				return false
			}
			contentFields := strings.Fields(content)
			importPrefix := contentFields[0]
			if !strings.HasPrefix(pkg, importPrefix) {
				return false
			}
			u, err := url.Parse(contentFields[2])
			if err != nil {
				return false
			}
			pathParts := strings.Split(u.Path, "/")
			// Filter empty strings
			n := 0
			for _, part := range pathParts {
				if part != "" {
					pathParts[n] = part
					n++
				}
			}
			pathParts = pathParts[:n]
			if len(pathParts) == 0 {
				return false
			}
			parts = append([]string{u.Host}, pathParts...)
			// Remove ".git" suffix
			parts[len(parts)-1] = strings.TrimSuffix(parts[len(parts)-1], ".git")
			if debugOn {
				log.Printf("Resolved dependency %s to %s", pkg, strings.Join(parts, "/"))
			}
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if f(c) {
				return true
			}
		}
		return false
	}
	if f(doc) {
		return parts, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Invalid package ID: %s", pkg))
	}
}

func dependencies(pkg Package) ([]Dependency, error) {
	deps, err := moduleDependencies(pkg)
	if err == nil {
		return deps, nil
	}
	deps, err = glideDependencies(pkg)
	if err == nil {
		return deps, nil
	}
	deps, err = gopkgDependencies(pkg)
	if err == nil {
		return deps, nil
	}
	deps, err = glockDependencies(pkg)
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

func moduleDependencies(pkg Package) ([]Dependency, error) {
	modUrl, err := rawFileUrl(pkg, "go.sum")
	if debugOn {
		log.Printf("Looking for go.sum at %s", modUrl)
	}
	if err != nil {
		return nil, err
	}
	res, err := http.Get(modUrl)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		msg := fmt.Sprintf("go.sum not available; HTTP status=%d", res.StatusCode)
		return nil, errors.New(msg)
	}
	modBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	lock, err := readGoSum(modUrl, modBytes)
	if err != nil {
		return nil, err
	}
	return lock, nil
}

// emptyGoModHash and readGoSum are adapted from go internal code:
// https://github.com/golang/vgo/blob/9d567625acf4c5e156b9890bf6feb16eb9fa5c51/vendor/cmd/go/internal/modfetch/fetch.go#L193

// emptyGoModHash is the hash of a 1-file tree containing a 0-length go.mod.
// A bug caused us to write these into go.sum files for non-modules.
// We detect and remove them.
const emptyGoModHash = "h1:G7mAYYxgmS0lVkHyy2hEOLQCFB0DlQFTMLWggykrydY="

func readGoSum(file string, data []byte) ([]Dependency, error) {
	var mods = make(map[string]Dependency)
	lineno := 0
	for len(data) > 0 {
		var line []byte
		lineno++
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			line, data = data, nil
		} else {
			line, data = data[:i], data[i+1:]
		}
		f := strings.Fields(string(line))
		if len(f) == 0 {
			// blank line; skip it
			continue
		}
		if len(f) != 3 {
			msg := fmt.Sprintf("go: malformed go.sum:\n%s:%d: wrong number of fields %v", file, lineno, len(f))
			return nil, errors.New(msg)
		}
		if f[2] == emptyGoModHash {
			// Old bug; drop it.
			continue
		}
		if strings.HasSuffix(f[1], "/go.mod") {
			// Skip go.mod entry; see
			// https://golang.org/cmd/go/#hdr-Module_authentication_using_go_sum
			continue
		}
		name := readName(f[0])
		version := readVersion(f[1])
		if debugOn {
			msg := fmt.Sprintf("Found dependency: %s (%s)", name, version)
			log.Println(msg)
		}
		mods[name] = Dependency{Name: name, Version: version}
	}

	var modValues = make([]Dependency, len(mods))
	i := 0
	for _, v := range mods {
		modValues[i] = v
		if debugOn {
			msg := fmt.Sprintf("Using dependency: %s (%s)", v.Name, v.Version)
			log.Println(msg)
		}
		i++
	}
	return modValues, nil
}

var pkgVerReg = regexp.MustCompile("/v\\d+$")

func readName(raw string) string {
	return pkgVerReg.ReplaceAllString(raw, "")
}

func readVersion(raw string) string {
	f := strings.FieldsFunc(raw, func(r rune) bool { return strings.ContainsRune("-+", r) })
	if len(f) == 4 && f[3] == "incompatible" {
		// A pseudo-version with +incompatible
		// https://golang.org/cmd/go/#hdr-Pseudo_versions
		return f[2]
	}
	if len(f) == 3 {
		// A pseudo-version
		return f[2]
	}
	if len(f) == 2 && f[1] == "incompatible" {
		// A normal version with +incompatible
		return f[0]
	}
	// Just use the raw version
	return raw
}

func glideDependencies(pkg Package) ([]Dependency, error) {
	lockUrl, err := rawFileUrl(pkg, "glide.lock")
	if err != nil {
		return nil, err
	}
	if debugOn {
		log.Printf("Looking for glide.lock at %s", lockUrl)
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
	if debugOn {
		log.Printf("Looking for Gopkg.lock at %s", lockUrl)
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

func glockDependencies(pkg Package) ([]Dependency, error) {
	lockUrl, err := rawFileUrl(pkg, "GLOCKFILE")
	if err != nil {
		return nil, err
	}
	if debugOn {
		log.Printf("Looking for GLOCKFILE at %s", lockUrl)
	}
	res, err := http.Get(lockUrl)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		msg := fmt.Sprintf("GLOCKFILE not available; HTTP status=%d", res.StatusCode)
		return nil, errors.New(msg)
	}
	glockBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	return readGlockfile(glockBytes), nil
}

func readGlockfile(data []byte) []Dependency {
	var mods []Dependency
	for len(data) > 0 {
		var line []byte
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			line, data = data, nil
		} else {
			line, data = data[:i], data[i+1:]
		}
		f := strings.Fields(string(line))
		if len(f) != 2 {
			continue
		}
		name := f[0]
		version := f[1]
		mod := Dependency{Name: name, Version: version}
		mods = append(mods, mod)
	}
	return mods
}

func packageAlias(pkg Package) string {
	if pkg.Alias == "" {
		return ""
	}
	return fmt.Sprintf("go.package%s%s\n\n", strings.Repeat(" ", 10), pkg.Alias)
}

func goVendors(deps []Dependency) string {
	if len(deps) == 0 {
		return ""
	}
	ret := "go.vendors          "
	for i, dep := range deps {
		pkg, err := newPackage(dep.Name, dep.Version)
		if pkg.Alias == "" {
			ret = ret + pkg.Id + " \\\n"
		} else {
			ret = ret + pkg.Alias + " \\\n"
			ret = ret + fmt.Sprintf("%srepo    %s \\\n", strings.Repeat(" ", 24), pkg.Id)
		}
		ret = ret + fmt.Sprintf("%slock    %s \\\n", strings.Repeat(" ", 24), dep.Version)
		if debugOn && err != nil {
			msg := fmt.Sprintf("Could not parse package ID: %s", dep.Name)
			log.Println(msg)
			log.Println(err)
		}
		if debugOn {
			log.Printf("Calculating checksums for %s", pkg.Id)
		}
		csums, err := checksums(pkg)
		if debugOn && err != nil {
			msg := fmt.Sprintf("Could not calculate checksums for package: %s", pkg.Id)
			log.Println(msg)
			log.Println(err)
		}
		ret = ret + csums.valueString(24)
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

	// go.googlesource.com appears to serve slightly different tarballs each
	// time you hit the URL, so it's impossible to get a stable checksum. DON'T
	// enable until this issue has been resolved.
	//
	// case "go.googlesource.com":
	// 	return fmt.Sprintf("https://go.googlesource.com/%s/+archive/refs/tags/%s.tar.gz",
	// 		pkg.Project, pkg.Version), nil
	default:
		// Custom domain GitLab repos
		return fmt.Sprintf("https://%s/%s/%s/-/archive/%s/%s-%s.tar.gz",
			pkg.Host, pkg.Author, pkg.Project, pkg.Version, pkg.Project, pkg.Version), nil
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

func (csums *Checksums) valueString(indentSize int) string {
	pad := strings.Repeat(" ", indentSize)
	ret := fmt.Sprintf(`%[1]srmd160  %[2]s \
%[1]ssha256  %[3]s \
%[1]ssize    %[4]s`, pad, csums.Rmd160, csums.Sha256, csums.Size)
	return ret
}

func checksumsStr(pkg Package, depCount int) string {
	csums, err := checksums(pkg)
	if debugOn && err != nil {
		msg := fmt.Sprintf("Could not calculate checksums for package: %s", pkg.Id)
		log.Println(msg)
		log.Println(err)
	}
	if depCount > 0 {
		return "checksums           ${distname}${extract.suffix} \\\n" + csums.valueString(24)
	} else {
		return "checksums           " + strings.TrimSpace(csums.valueString(20))
	}
}
