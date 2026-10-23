package main

import (
	"os"

	"github.com/jicheng1014/maolv-cli/internal/cli"
)

var version = "dev"

func main() { os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version)) }
