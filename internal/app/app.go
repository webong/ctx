package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/webong/ctx/internal/config"
	"github.com/webong/ctx/internal/engine"
	"github.com/webong/ctx/internal/launch"
	"github.com/webong/ctx/internal/platform"
)

const Version = "0.8.0-dev"

type environment struct{}

func (environment) Get(key string) string { return os.Getenv(key) }

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		usage(stdout)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-V" {
		fmt.Fprintf(stdout, "ctx %s\n", Version)
		return 0
	}
	resolver, err := newResolver()
	if err != nil {
		fmt.Fprintf(stderr, "ctx: %v\n", err)
		return 1
	}
	switch args[0] {
	case "resolve":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: resolve needs a key")
			return 2
		}
		resolved, err := resolver.Resolve(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, resolved.Value)
		return 0
	case "status":
		return status(resolver, args[1:], stdout, stderr)
	case "explain":
		return explain(resolver, stdout, stderr)
	case "env":
		return showEnvironment(resolver, stdout, stderr)
	case "adapter":
		return adapterCommand(resolver, args[1:], stdout, stderr)
	case "ls":
		return listContexts(resolver, args[1:], stdout, stderr)
	case "open":
		return openBrowser(resolver, args[1:], stdout, stderr)
	case "doctor":
		return doctor(resolver, stdout, stderr)
	case "run":
		return runCommand(resolver, args[1:], stdout, stderr)
	case "shell":
		return shell(resolver, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "ctx: %s is not migrated to the native core yet\n", args[0])
		return 2
	}
}

func usage(output io.Writer) {
	fmt.Fprintln(output, `ctx — native cross-platform context core (migration preview)
usage:
  ctx status [selector]
  ctx resolve <key>
  ctx explain
  ctx env
  ctx adapter <ls|inspect|install|trust|test|doctor|remove> [arguments...]
  ctx ls <selector>
  ctx open [URL...]
  ctx doctor
  ctx run <docker|podman|nerdctl> [arguments...]
  ctx run -- <command> [arguments...]
  ctx shell [-- <command> [arguments...]]
  ctx version`)
}

func newResolver() (*config.Resolver, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return config.NewResolver(workingDir, homeDir, filepath.Join(configHomePath(), "config.toml"))
}

func configHomePath() string {
	if home := os.Getenv("CTX_HOME"); home != "" {
		return home
	}
	return platform.DefaultConfigHome()
}

func status(resolver *config.Resolver, selectors []string, stdout, stderr io.Writer) int {
	showAll := len(selectors) == 0
	if showAll {
		selectors = []string{"docker", "podman", "nerdctl", "browser", "profile"}
		if installed, err := adapterStore().List(); err == nil {
			for _, candidate := range installed {
				if candidate.Manifest.Kind == "selector" {
					selectors = append(selectors, candidate.Manifest.Name)
				}
			}
		}
	}
	if !showAll && len(selectors) != 1 {
		fmt.Fprintln(stderr, "ctx: status accepts at most one selector")
		return 2
	}
	for _, selector := range selectors {
		if value, source := environmentOverride(selector); value != "" {
			fmt.Fprintf(stdout, "%s: %s (%s)\n", selector, value, source)
			continue
		}
		key := selector
		if installed, err := adapterStore().Load(selector); err == nil && installed.Manifest.Kind == "selector" {
			key = installed.Manifest.SelectorKey
		}
		resolved, err := resolver.Resolve(key)
		if err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		if resolved.Value == "" {
			fmt.Fprintf(stdout, "%s: <not selected>\n", selector)
		} else {
			fmt.Fprintf(stdout, "%s: %s (%s)\n", selector, resolved.Value, resolved.Source)
		}
	}
	return 0
}

func environmentOverride(selector string) (string, string) {
	checks := map[string][]string{
		"docker":  {"DOCKER_CONTEXT", "DOCKER_HOST"},
		"podman":  {"CONTAINER_CONNECTION", "CONTAINER_HOST"},
		"nerdctl": {"CONTAINERD_NAMESPACE", "CONTAINERD_ADDRESS"},
		"browser": {"CTX_BROWSER"},
	}
	for _, key := range checks[selector] {
		if value := os.Getenv(key); value != "" {
			return value, key
		}
	}
	return "", ""
}

func explain(resolver *config.Resolver, stdout, stderr io.Writer) int {
	for _, item := range []struct{ label, key string }{
		{"profile", "profile"}, {"docker", "docker"}, {"podman", "podman"},
		{"nerdctl", "nerdctl"}, {"browser", "browser"}, {"shell-path", "shell_path"},
	} {
		resolved, err := resolver.Resolve(item.key)
		if err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		if resolved.Value == "" {
			fmt.Fprintf(stdout, "%-16s <native default>\n", item.label)
		} else {
			fmt.Fprintf(stdout, "%-16s %s (%s)\n", item.label, resolved.Value, resolved.Source)
		}
	}
	return 0
}

func profileEnvironment(resolver *config.Resolver) (map[string]string, error) {
	result := map[string]string{}
	_, profile, err := resolver.ActiveProfile()
	if err != nil {
		return result, err
	}
	if profile != nil {
		for key, value := range profile.Env {
			if os.Getenv(key) == "" {
				result[key] = value
			}
		}
	}
	resolvedPath, err := resolver.Resolve("shell_path")
	if err != nil {
		return result, err
	}
	if resolvedPath.Value != "" {
		result["PATH"] = resolvedPath.Value + string(os.PathListSeparator) + os.Getenv("PATH")
	}
	return result, nil
}

func showEnvironment(resolver *config.Resolver, stdout, stderr io.Writer) int {
	values, err := profileEnvironment(resolver)
	if err != nil {
		fmt.Fprintf(stderr, "ctx: %v\n", err)
		return 1
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(stdout, "%s=%s\n", key, values[key])
	}
	return 0
}

func runCommand(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: run needs a tool or -- followed by a command")
		return 2
	}
	if args[0] == "--" {
		if len(args) == 1 {
			fmt.Fprintln(stderr, "ctx: run -- needs a command")
			return 2
		}
		return execute(resolver, args[1], args[2:], stdout, stderr)
	}
	engineName := args[0]
	if engineName != "docker" && engineName != "podman" && engineName != "nerdctl" {
		return runAdapterTool(resolver, engineName, args[1:], stdout, stderr)
	}
	real, err := launch.FindReal(engineName)
	if err != nil {
		fmt.Fprintf(stderr, "ctx: %v\n", err)
		return 127
	}
	resolved, err := resolver.Resolve(engineName)
	if err != nil {
		fmt.Fprintf(stderr, "ctx: %v\n", err)
		return 1
	}
	engineArgs := engine.Arguments(engineName, resolved.Value, args[1:], environment{})
	return execute(resolver, real, engineArgs, stdout, stderr)
}

func shell(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		if len(args) == 1 {
			fmt.Fprintln(stderr, "ctx: shell -- needs a command")
			return 2
		}
		return execute(resolver, args[1], args[2:], stdout, stderr)
	}
	if len(args) != 0 {
		fmt.Fprintln(stderr, "ctx: shell only accepts -- followed by a command")
		return 2
	}
	return execute(resolver, platform.DefaultShell(), nil, stdout, stderr)
}

func execute(resolver *config.Resolver, name string, args []string, stdout, stderr io.Writer) int {
	command := exec.Command(name, args...)
	command.Stdin = os.Stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = os.Environ()
	values, err := profileEnvironment(resolver)
	if err != nil {
		fmt.Fprintf(stderr, "ctx: %v\n", err)
		return 1
	}
	for key, value := range values {
		command.Env = setEnvironment(command.Env, key, value)
	}
	if err := command.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		fmt.Fprintf(stderr, "ctx: %v\n", err)
		return 1
	}
	return 0
}

func setEnvironment(values []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	for index, entry := range values {
		if strings.HasPrefix(strings.ToUpper(entry), prefix) {
			values[index] = key + "=" + value
			return values
		}
	}
	return append(values, key+"="+value)
}
