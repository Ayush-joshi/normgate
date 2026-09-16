// Package cli implements configuration and deterministic policy development commands.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"normgate.dev/normgate/internal/config"
	"normgate.dev/normgate/internal/normalization"
	"normgate.dev/normgate/internal/policy"
)

const Version = policy.CoreVersion + "-dev"
const usage = "Usage: normgate version | normgate config validate --file PATH [--listen HOST:PORT] [--timeout-ms N]"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "policy" {
		return policyRun(args[1:], stdout, stderr)
	}
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(stdout, Version)
		return 0
	}
	if len(args) < 2 || args[0] != "config" || args[1] != "validate" {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("file", "", "configuration file")
	listen := flags.String("listen", "", "listen address override")
	timeout := flags.Int64("timeout-ms", 0, "request timeout override")
	if err := flags.Parse(args[2:]); err != nil || *path == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	file, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(stderr, "Cannot read configuration.")
		return 1
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, normalization.MaxBytes+1))
	if err != nil {
		fmt.Fprintln(stderr, "Cannot read configuration.")
		return 1
	}
	overrides := make(map[string]any)
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "listen":
			overrides["listen_address"] = *listen
		case "timeout-ms":
			overrides["request_timeout_ms"] = *timeout
		}
	})
	if _, err := config.Load(data, overrides); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "Configuration valid.")
	return 0
}
