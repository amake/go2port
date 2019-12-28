# go2port: a MacPorts portfile generator for Go projects

go2port is a tool for generating and updating
[MacPorts](https://www.macports.org/) portfiles for projects written in
[Go](https://golang.org/).

## Installation

Install with MacPorts:

```
sudo port install go2port
```

Or with Go:

```
go get github.com/amake/go2port
```

## Usage

Supply a Go package and a version to generate a portfile template. The result is
sent to standard output by default:

```
$ go2port get github.com/amake/go2port 1.0.0
# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4

PortSystem          1.0
PortGroup           golang 1.0

go.setup            github.com/amake/go2port 1.0.0
categories
maintainers
license

description

long_description

checksums           ${distname}${extract.suffix} \
                        rmd160  8a0c94a4e840ede8633b305c467a2afc3184ca14 \
                        sha256  571217785e309f01e528842b246c8085189972696df525acacb1cf17dcd59bd5 \
                        size    4305

go.vendors          github.com/BurntSushi/toml \
                        lock    b26d9c308763d68093482582cea63d69be07a0f0 \
                        rmd160  08c91052763fa884c7d88f6b10a03bfbcdea93e8 \
                        sha256  360c150f4ec9f5450feee0009aba9555b6731ca0bbb2ce612c3b7b9173c0d896 \
                        size    41567 \
                    github.com/urfave/cli \
                        lock    cfb38830724cc34fedffe9a2a29fb54fa9169cd1 \
                        rmd160  b54f7232fbbfda640f7d9411a5dedab3adf6a888 \
                        sha256  94f12754129bce1d3435efd84826a73fc8af70f61f9264c60c1f554d425d503a \
                        size    58405 \
                    golang.org/x/crypto \
                        lock    0e37d006457bf46f9e6692014ba72ef82c33022c \
                        rmd160  dc6590753cf4472777b7a35a8ceacfb9a2316091 \
                        sha256  ab5b09609da7722997b32a55b58703e90815e8a8c28668444df62b00cac93aab \
                        size    1638395 \
                    gopkg.in/yaml.v2 \
                        lock    5420a8b6744d3b0345ab293f6fcba19c978f1183 \
                        rmd160  56eb283b31feac8db4ede3e24768e0f9999913d2 \
                        sha256  34dc73c7798abfa3bb96c46c25002ccc5b92543dc3e008a31e0ae94c2528e52b \
                        size    70231

destroot {
    xinstall -m 755 ${worksrcpath}/${name} ${destroot}${prefix}/bin/
}
```

If the project is hosted on GitHub or Bitbucket, go2port will automatically
calculate the checksums for the main distfile.

If the project uses a supported lockfile format for dependencies (currently
`go.sum`, `glide.lock` or `Gopkg.lock`), go2port will also automatically add
`go.vendors` entries for dependencies.

See the [golang PortGroup
documentation](https://guide.macports.org/#reference.portgroup.golang) for more
information about specifying dependencies.

**Note:** Many projects commit their dependency source e.g. in `vendor`. For
such projects you should not specify `go.vendors`.

### Updating existing ports

go2port can also update existing portfiles:

```
$ go2port update go2port 1.0.1
```

By default this will overwrite an existing portfile (located with `port file
<portname>`) with new checksums and dependency information.

## License

go2port is available under the three-clause BSD license.

## See also

- [MacPorts Guide: golang
  PortGroup](https://guide.macports.org/#reference.portgroup.golang)
- [golang-1.0 PortGroup
code](https://github.com/macports/macports-ports/blob/master/_resources/port1.0/group/golang-1.0.tcl)
