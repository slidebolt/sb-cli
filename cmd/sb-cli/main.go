package main

import (
	"os"

	"github.com/slidebolt/sb-cli/app"
)

func main() {
	runner, err := app.DefaultRunnerFromEnv()
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr, runner))
}
