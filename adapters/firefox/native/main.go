package main

import (
	"io"
	"os"

	firefox "github.com/webong/ctx/adapters/firefox/engine"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	return firefox.Run(firefox.Config{
		Name: "firefox", MacProfileRoot: "Firefox", WindowsProfileRoot: "Mozilla/Firefox",
		LinuxProfileRoot: ".mozilla/firefox", LinuxFallbackProfileRoot: "snap/firefox/common/.mozilla/firefox",
	}, args, input, stdout, stderr)
}
