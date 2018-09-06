package main

import (
	"fmt"
	"github.com/urfave/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "go2port"
	app.Usage = "Generate a MacPorts portfile from a Go project"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "project",
			Usage: "Project to generate portfile for",
		},
	}
	app.Action = generate

	app.Run(os.Args)
}

func generate(c *cli.Context) error {
	fmt.Println("Getting project:", c.Args().Get(0))
	return nil
}
