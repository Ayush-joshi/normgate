package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"normgate.dev/normgate/internal/contracts"
	v1 "normgate.dev/normgate/internal/contracts/v1"
	"normgate.dev/normgate/internal/policy"
	"normgate.dev/normgate/internal/policy/opa"
	"normgate.dev/normgate/policies"
)

const policyUsage = "Usage: normgate policy init [PATH] | lint PATH | build PATH --out FILE | test PATH | inspect BUNDLE | diff OLD NEW | explain --bundle BUNDLE --event EVENT | activate --bundle BUNDLE --state DIR | status --state DIR | rollback --revision REVISION --state DIR"

func policyRun(args []string, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	usage := func() int { fmt.Fprintln(stderr, policyUsage); return 2 }
	emit := func(value any) int {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(value); err != nil {
			return fail(policy.Failure("output_failed"))
		}
		return 0
	}
	if len(args) == 0 {
		return usage()
	}
	ctx := context.Background()
	engine := opa.New()
	defer engine.Close()
	switch args[0] {
	case "init":
		if len(args) > 2 {
			return usage()
		}
		dir := "policy"
		if len(args) == 2 {
			dir = args[1]
		}
		if _, err := os.Lstat(dir); err == nil {
			return fail(policy.Failure("path_exists"))
		}
		if err := os.Mkdir(dir, 0700); err != nil {
			return fail(policy.Failure("bundle_write"))
		}
		err := fs.WalkDir(policies.Baseline, "baseline", func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			target := filepath.Join(dir, strings.TrimPrefix(name, "baseline/"))
			if name == "baseline" {
				return nil
			}
			if d.IsDir() {
				return os.Mkdir(target, 0700)
			}
			raw, err := policies.Baseline.ReadFile(name)
			if err != nil {
				return err
			}
			return os.WriteFile(target, raw, 0600)
		})
		if err != nil {
			return fail(policy.Failure("bundle_write"))
		}
		fmt.Fprintln(stdout, "Policy initialized.")
		return 0
	case "lint", "test", "inspect", "build":
		if len(args) < 2 {
			return usage()
		}
		out := ""
		if args[0] == "build" {
			flags := flag.NewFlagSet("policy build", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			flags.StringVar(&out, "out", "", "archive output")
			if flags.Parse(args[2:]) != nil || flags.NArg() != 0 || out == "" {
				return usage()
			}
		} else if len(args) != 2 {
			return usage()
		}
		bundle, err := policy.Load(args[1])
		if err != nil {
			return fail(err)
		}
		_, report, err := policy.Prepare(bundle)
		if err != nil {
			return fail(err)
		}
		if args[0] == "inspect" {
			return emit(report)
		}
		revision, err := engine.Compile(ctx, bundle)
		if err != nil {
			return fail(err)
		}
		if args[0] == "lint" {
			return emit(report)
		}
		tests, err := policy.RunTests(ctx, engine, revision, bundle)
		if err != nil {
			emit(tests)
			return fail(err)
		}
		if args[0] == "test" {
			return emit(tests)
		}
		archive, err := policy.Build(bundle)
		if err != nil {
			return fail(err)
		}
		// Create exclusively so a typo cannot truncate an existing artifact/source file.
		file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fail(policy.Failure("bundle_write"))
		}
		_, err = file.Write(archive)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return fail(policy.Failure("bundle_write"))
		}
		return emit(report)
	case "diff":
		if len(args) != 3 {
			return usage()
		}
		a, err := policy.Load(args[1])
		if err != nil {
			return fail(err)
		}
		b, err := policy.Load(args[2])
		if err != nil {
			return fail(err)
		}
		return emit(policy.Diff(a, b))
	case "explain":
		flags := flag.NewFlagSet("policy explain", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		bundlePath := flags.String("bundle", "", "policy")
		eventPath := flags.String("event", "", "event")
		if flags.Parse(args[1:]) != nil || flags.NArg() != 0 || *bundlePath == "" || *eventPath == "" {
			return usage()
		}
		bundle, err := policy.Load(*bundlePath)
		if err != nil {
			return fail(err)
		}
		revision, err := engine.Compile(ctx, bundle)
		if err != nil {
			return fail(err)
		}
		file, err := os.Open(*eventPath)
		if err != nil {
			return fail(policy.Failure("event_read"))
		}
		defer file.Close()
		raw, err := policy.ReadEvent(file)
		if err != nil {
			return fail(err)
		}
		event, err := contracts.Decode[v1.EventEnvelope]("event-envelope", raw)
		if err != nil {
			return fail(policy.Failure("invalid_event"))
		}
		explanation, err := engine.Explain(ctx, revision, event)
		if err != nil {
			return fail(err)
		}
		return emit(explanation)
	case "activate", "status", "rollback":
		flags := flag.NewFlagSet("policy lifecycle", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		state := flags.String("state", "", "durable state directory")
		bundlePath := flags.String("bundle", "", "archive")
		revision := flags.String("revision", "", "exact revision")
		staleness := flags.Duration("max-staleness", 24*time.Hour, "maximum policy age")
		if flags.Parse(args[1:]) != nil || flags.NArg() != 0 || *state == "" || *staleness <= 0 {
			return usage()
		}
		if args[0] == "activate" && (*bundlePath == "" || *revision != "") {
			return usage()
		}
		if args[0] == "rollback" && (*revision == "" || *bundlePath != "") {
			return usage()
		}
		if args[0] == "status" && (*bundlePath != "" || *revision != "") {
			return usage()
		}
		manager, err := policy.NewManager(engine, *state, *staleness)
		if err != nil {
			return fail(err)
		}
		if args[0] == "activate" {
			absolute, err := filepath.Abs(*bundlePath)
			if err != nil {
				return fail(policy.Failure("invalid_source"))
			}
			err = manager.Refresh(ctx, policy.FileSource{Path: absolute})
			if err != nil {
				return fail(err)
			}
		} else if args[0] == "rollback" {
			if err = manager.Rollback(ctx, policy.Revision(*revision)); err != nil {
				return fail(err)
			}
		}
		return emit(manager.Status())
	default:
		return usage()
	}
}
