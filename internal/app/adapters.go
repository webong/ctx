package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	adapterpkg "github.com/webong/ctx/internal/adapter"
	"github.com/webong/ctx/internal/config"
	"github.com/webong/ctx/internal/launch"
)

func adapterStore() *adapterpkg.Store {
	home := os.Getenv("CTX_ADAPTER_HOME")
	if home == "" {
		home = filepath.Join(configHomePath(), "adapters")
	}
	return adapterpkg.NewStore(home)
}

func adapterCommand(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: adapter requires ls, available, add, refresh, inspect, install, trust, test, doctor, or remove")
		return 2
	}
	store := adapterStore()
	switch args[0] {
	case "ls", "list":
		if len(args) > 2 {
			fmt.Fprintln(stderr, "ctx: adapter ls accepts an optional kind or computer")
			return 2
		}
		kind := ""
		if len(args) == 2 {
			kind = args[1]
			if kind != "selector" && kind != "browser" && kind != "container" && kind != "computer" {
				fmt.Fprintf(stderr, "ctx: unknown adapter kind %s\n", kind)
				return 2
			}
		}
		installed, err := store.List()
		if err != nil {
			return reportError(stderr, err)
		}
		for _, candidate := range installed {
			if kind == "computer" && !candidate.IsComputerEndpoint() {
				continue
			}
			if kind != "" && kind != "computer" && candidate.Manifest.Kind != kind {
				continue
			}
			state := "untrusted"
			if trusted, err := store.IsTrusted(candidate); err == nil && trusted {
				state = "trusted"
			}
			fmt.Fprintf(stdout, "%-16s %-10s %s\n", candidate.Manifest.Name, state, candidate.Manifest.Description)
		}
		return 0
	case "inspect":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: adapter inspect needs a name")
			return 2
		}
		candidate, err := store.Load(args[1])
		if err != nil {
			return reportError(stderr, err)
		}
		state := "untrusted"
		if trusted, err := store.IsTrusted(candidate); err == nil && trusted {
			state = "trusted"
		}
		fmt.Fprintf(stdout, "name:         %s\n", candidate.Manifest.Name)
		fmt.Fprintf(stdout, "api:          %s\n", candidate.Manifest.APIVersion)
		kind := candidate.Manifest.Kind
		if kind == "" && candidate.IsComputerEndpoint() {
			kind = "computer endpoint"
		}
		fmt.Fprintf(stdout, "kind:         %s\n", kind)
		fmt.Fprintf(stdout, "state:        %s\n", state)
		fmt.Fprintf(stdout, "selector:     %s\n", candidate.Manifest.SelectorKey)
		fmt.Fprintf(stdout, "commands:     %s\n", strings.Join(candidate.Manifest.Commands, ","))
		if len(candidate.Manifest.ComputerCommands) > 0 {
			fmt.Fprintf(stdout, "computer commands: %s\n", strings.Join(candidate.Manifest.ComputerCommands, ","))
		}
		if len(candidate.Manifest.ComputerCapabilities) > 0 {
			fmt.Fprintf(stdout, "computer capabilities: %s\n", strings.Join(candidate.Manifest.ComputerCapabilities, ","))
		}
		fmt.Fprintf(stdout, "capabilities: %s\n", strings.Join(candidate.Manifest.Capabilities, ","))
		if len(candidate.Manifest.ShareSpaces) > 0 {
			fmt.Fprintf(stdout, "share spaces: %s\n", strings.Join(candidate.Manifest.ShareSpaces, ","))
		}
		if len(candidate.Manifest.OverrideEnv) > 0 {
			fmt.Fprintf(stdout, "override env: %s\n", strings.Join(candidate.Manifest.OverrideEnv, ","))
		}
		if candidate.Manifest.Kind == "container" {
			fmt.Fprintf(stdout, "default:      %t\n", candidate.Manifest.DefaultProvider)
		}
		fmt.Fprintf(stdout, "executable:   %s\n", candidate.ExecutablePath())
		fmt.Fprintf(stdout, "description:  %s\n", candidate.Manifest.Description)
		return 0
	case "available":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "ctx: adapter available takes no arguments")
			return 2
		}
		available, err := catalogStore().List()
		if err != nil {
			return reportError(stderr, err)
		}
		installed := installedAdapterNames()
		for _, candidate := range available {
			state := "available"
			if installed[candidate.Manifest.Name] {
				state = "installed"
			}
			fmt.Fprintf(stdout, "%-16s %-10s %-10s %s\n", candidate.Manifest.Name, candidate.Manifest.Kind, state, candidate.Manifest.Description)
		}
		return 0
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "ctx: adapter add needs one or more catalog names")
			return 2
		}
		for _, name := range args[1:] {
			if _, err := addCatalogAdapter(name); err != nil {
				return reportError(stderr, err)
			}
			fmt.Fprintf(stdout, "installed adapter %s\n", name)
		}
		return 0
	case "refresh":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "ctx: adapter refresh takes no arguments")
			return 2
		}
		if err := refreshCatalogAdapters(stdout); err != nil {
			return reportError(stderr, err)
		}
		return 0
	case "install":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: adapter install needs a source directory")
			return 2
		}
		installed, err := store.Install(args[1])
		if err != nil {
			return reportError(stderr, err)
		}
		fmt.Fprintf(stdout, "installed adapter %s in %s (untrusted)\n", installed.Manifest.Name, installed.Directory)
		fmt.Fprintf(stdout, "review it, then run: ctx adapter trust %s\n", installed.Manifest.Name)
		return 0
	case "trust":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: adapter trust needs a name")
			return 2
		}
		candidate, err := store.Load(args[1])
		if err != nil {
			return reportError(stderr, err)
		}
		if err := checkShimConflicts(candidate); err != nil {
			return reportError(stderr, err)
		}
		if err := store.Trust(candidate); err != nil {
			return reportError(stderr, err)
		}
		if err := installAdapterShims(candidate); err != nil {
			return reportError(stderr, err)
		}
		fmt.Fprintf(stdout, "trusted adapter %s\n", candidate.Manifest.Name)
		return 0
	case "test":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: adapter test needs an adapter directory")
			return 2
		}
		candidate, err := adapterpkg.LoadDirectory(args[1])
		if err != nil {
			return reportError(stderr, err)
		}
		invocation, err := candidate.Command(adapterpkg.Invocation{Operation: "doctor", Project: currentDirectory()})
		if err != nil {
			return reportError(stderr, err)
		}
		if code := runPrepared(invocation, stdout, stderr); code != 0 {
			return code
		}
		fmt.Fprintf(stdout, "adapter %s satisfies ctx adapter API v%s\n", candidate.Manifest.Name, adapterpkg.APIVersion)
		return 0
	case "doctor":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: adapter doctor needs a name")
			return 2
		}
		candidate, err := store.Load(args[1])
		if err != nil {
			return reportError(stderr, err)
		}
		selection, err := adapterSelection(resolver, candidate)
		if err != nil {
			return reportError(stderr, err)
		}
		return invokeAdapter(resolver, candidate, "doctor", selection, nil, "", stdout, stderr)
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: adapter remove needs a name")
			return 2
		}
		candidate, err := store.Load(args[1])
		if err != nil {
			return reportError(stderr, err)
		}
		if err := removeAdapterShims(candidate); err != nil {
			return reportError(stderr, err)
		}
		if err := store.Remove(args[1]); err != nil {
			return reportError(stderr, err)
		}
		fmt.Fprintf(stdout, "removed adapter %s\n", args[1])
		return 0
	default:
		fmt.Fprintln(stderr, "ctx: adapter requires ls, available, add, refresh, inspect, install, trust, test, doctor, or remove")
		return 2
	}
}

func runAdapterTool(resolver *config.Resolver, tool string, args []string, stdout, stderr io.Writer) int {
	installed, err := adapterStore().List()
	if err != nil {
		return reportError(stderr, err)
	}
	var matched *adapterpkg.Adapter
	for _, candidate := range installed {
		commandMatch := candidate.HasCommand(tool) || candidate.HasComputerCommand(tool)
		if (candidate.Manifest.Kind != "selector" && candidate.Manifest.Kind != "container" && !candidate.IsComputerEndpoint()) || (candidate.Manifest.Name != tool && !commandMatch) {
			continue
		}
		if matched != nil {
			fmt.Fprintf(stderr, "ctx: command %s is claimed by both %s and %s\n", tool, matched.Manifest.Name, candidate.Manifest.Name)
			return 1
		}
		matched = candidate
	}
	if matched == nil {
		fmt.Fprintf(stderr, "ctx: no adapter for %s\n", tool)
		return 2
	}
	selection, err := adapterSelection(resolver, matched)
	if err != nil {
		return reportError(stderr, err)
	}
	if selection == "" && matched.Manifest.Kind != "container" && !matched.IsComputerEndpoint() {
		fmt.Fprintf(stderr, "ctx: no %s selection; run ctx set %s <name>\n", matched.Manifest.Name, matched.Manifest.Name)
		return 1
	}
	return invokeAdapter(resolver, matched, "run", selection, args, tool, stdout, stderr)
}

func listContexts(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "ctx: ls needs one selector")
		return 2
	}
	selector := args[0]
	switch selector {
	case "browser":
		return listBrowsers(resolver, stdout, stderr)
	case "container":
		return listContainers(resolver, stdout, stderr)
	}
	candidate, err := adapterStore().Load(selector)
	if err != nil || (candidate.Manifest.Kind != "selector" && candidate.Manifest.Kind != "container") {
		fmt.Fprintf(stderr, "ctx: unknown selector %s\n", selector)
		return 2
	}
	return invokeAdapter(resolver, candidate, "list", "", nil, "", stdout, stderr)
}

func listBrowsers(resolver *config.Resolver, stdout, stderr io.Writer) int {
	return listProviderFamily(resolver, "browser", true, stdout, stderr)
}

func listContainers(resolver *config.Resolver, stdout, stderr io.Writer) int {
	return listProviderFamily(resolver, "container", true, stdout, stderr)
}

func listProviderFamily(resolver *config.Resolver, kind string, prefix bool, stdout, stderr io.Writer) int {
	installed, err := adapterStore().List()
	if err != nil {
		return reportError(stderr, err)
	}
	found := false
	var failures bytes.Buffer
	for _, candidate := range installed {
		if candidate.Manifest.Kind != kind {
			continue
		}
		var output, adapterError bytes.Buffer
		code := invokeAdapter(resolver, candidate, "list", "", nil, "", &output, &adapterError)
		if code != 0 {
			if adapterError.Len() > 0 {
				fmt.Fprintf(&failures, "%s: %s", candidate.Manifest.Name, adapterError.String())
				if !strings.HasSuffix(adapterError.String(), "\n") {
					failures.WriteByte('\n')
				}
			}
			continue
		}
		if output.Len() > 0 {
			found = true
			if prefix && kind == "container" {
				for _, selection := range strings.Split(strings.TrimSpace(output.String()), "\n") {
					if selection != "" {
						fmt.Fprintf(stdout, "%s:%s\n", candidate.Manifest.Name, selection)
					}
				}
			} else {
				_, _ = io.Copy(stdout, &output)
			}
		}
	}
	if !found {
		_, _ = io.Copy(stderr, &failures)
		fmt.Fprintf(stderr, "ctx: no supported %s contexts found\n", kind)
		return 1
	}
	return 0
}

func openBrowser(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	resolved, err := resolver.Resolve("browser")
	if err != nil {
		return reportError(stderr, err)
	}
	choice := os.Getenv("CTX_BROWSER")
	if choice == "" {
		choice = resolved.Value
	}
	provider, profile, ok := strings.Cut(choice, ":")
	if !ok || provider == "" || profile == "" {
		fmt.Fprintln(stderr, "ctx: no valid browser selected; use a value from ctx ls browser")
		return 1
	}
	candidate, err := adapterStore().Load(provider)
	if err != nil || candidate.Manifest.Kind != "browser" {
		fmt.Fprintf(stderr, "ctx: invalid browser provider %s\n", provider)
		return 1
	}
	code := invokeAdapter(resolver, candidate, "open", profile, args, "", stdout, stderr)
	if code == 0 {
		recordWebContext(provider, profile, args, stderr)
	}
	return code
}

func doctor(resolver *config.Resolver, stdout, stderr io.Writer) int {
	failures := 0
	checked := 0
	if name, profile, err := resolver.ActiveProfile(); err != nil {
		fmt.Fprintf(stdout, "fail profile: %v\n", err)
		failures++
	} else if name != "" {
		checked++
		if profile == nil {
			fmt.Fprintf(stdout, "fail profile %s does not exist\n", name)
			failures++
		} else {
			fmt.Fprintf(stdout, "ok   profile %s\n", name)
		}
	}

	if resolved, err := resolver.Resolve("browser"); err == nil && resolved.Value != "" {
		checked++
		provider, profile, ok := strings.Cut(resolved.Value, ":")
		candidate, loadErr := adapterStore().Load(provider)
		if !ok || loadErr != nil || candidate.Manifest.Kind != "browser" || invokeAdapter(resolver, candidate, "doctor", profile, nil, "", io.Discard, io.Discard) != 0 {
			fmt.Fprintf(stdout, "fail browser %s is unavailable\n", resolved.Value)
			failures++
		} else {
			fmt.Fprintf(stdout, "ok   browser %s\n", resolved.Value)
		}
	}

	installed, err := adapterStore().List()
	if err != nil {
		return reportError(stderr, err)
	}
	for _, candidate := range installed {
		if candidate.Manifest.Kind != "selector" && candidate.Manifest.Kind != "container" && !candidate.IsComputerEndpoint() {
			continue
		}
		selection, err := adapterSelection(resolver, candidate)
		if err != nil || (selection == "" && !candidate.IsComputerEndpoint()) {
			continue
		}
		checked++
		label := candidate.Manifest.Kind
		if candidate.IsComputerEndpoint() {
			label = "computer"
		} else if label == "selector" {
			label = "adapter"
		}
		if invokeAdapter(resolver, candidate, "doctor", selection, nil, "", io.Discard, io.Discard) != 0 {
			fmt.Fprintf(stdout, "fail %s %s %s is unavailable\n", label, candidate.Manifest.Name, selection)
			failures++
		} else {
			fmt.Fprintf(stdout, "ok   %s %s %s\n", label, candidate.Manifest.Name, selection)
		}
	}
	if checked == 0 {
		fmt.Fprintln(stdout, "ok   no ctx-managed selections")
	}
	if failures > 0 {
		return 1
	}
	return 0
}

func adapterSelection(resolver *config.Resolver, candidate *adapterpkg.Adapter) (string, error) {
	if candidate.Manifest.Kind == "" {
		return "", nil
	}
	resolved, err := resolver.Resolve(candidate.Manifest.SelectorKey)
	if err != nil {
		return "", err
	}
	if candidate.Manifest.Kind != "browser" || resolved.Value == "" {
		return resolved.Value, nil
	}
	provider, profile, ok := strings.Cut(resolved.Value, ":")
	if !ok || provider != candidate.Manifest.Name {
		return "", nil
	}
	return profile, nil
}

func invokeAdapter(resolver *config.Resolver, candidate *adapterpkg.Adapter, operation, selection string, args []string, requestedCommand string, stdout, stderr io.Writer) int {
	return invokeAdapterIO(resolver, candidate, operation, selection, args, requestedCommand, os.Stdin, stdout, stderr)
}

func invokeAdapterIO(resolver *config.Resolver, candidate *adapterpkg.Adapter, operation, selection string, args []string, requestedCommand string, stdin io.Reader, stdout, stderr io.Writer) int {
	store := adapterStore()
	if err := store.AssertTrusted(candidate); err != nil {
		return reportError(stderr, err)
	}
	values := map[string]string{}
	for _, key := range candidate.ConfigKeys() {
		resolved, err := resolver.Resolve(key)
		if err != nil {
			return reportError(stderr, err)
		}
		values[key] = resolved.Value
	}
	profile, _, _ := resolver.ActiveProfile()
	realCommand := ""
	if candidate.Manifest.Kind == "container" || (candidate.IsComputerEndpoint() && (operation == "run" || operation == "doctor" || operation == "plugin" || requestedCommand != "")) {
		commandName := ""
		if candidate.HasCommand(requestedCommand) || candidate.HasComputerCommand(requestedCommand) {
			commandName = requestedCommand
		} else if len(candidate.Manifest.Commands) > 0 {
			commandName = candidate.Manifest.Commands[0]
		} else if len(candidate.Manifest.ComputerCommands) > 0 {
			commandName = candidate.Manifest.ComputerCommands[0]
		}
		if commandName == "" {
			return reportErrorCode(stderr, fmt.Errorf("adapter %s does not declare a native command", candidate.Manifest.Name), 127)
		}
		var err error
		realCommand, err = launch.FindReal(commandName)
		if err != nil {
			return reportErrorCode(stderr, err, 127)
		}
	}
	command, err := candidate.Command(adapterpkg.Invocation{
		Operation: operation, Selection: selection, Arguments: args, Values: values,
		Profile: profile, Project: currentDirectory(), Command: requestedCommand, RealCommand: realCommand,
	})
	if err != nil {
		return reportError(stderr, err)
	}
	profileValues, err := profileEnvironment(resolver)
	if err != nil {
		return reportError(stderr, err)
	}
	for key, value := range profileValues {
		command.Env = setEnvironment(command.Env, key, value)
	}
	return runPreparedIO(command, stdin, stdout, stderr)
}

func runPrepared(command *exec.Cmd, stdout, stderr io.Writer) int {
	return runPreparedIO(command, os.Stdin, stdout, stderr)
}

func runPreparedIO(command *exec.Cmd, stdin io.Reader, stdout, stderr io.Writer) int {
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		fmt.Fprintf(stderr, "ctx: %v\n", err)
		return 1
	}
	return 0
}

func currentDirectory() string {
	directory, _ := os.Getwd()
	return directory
}

func reportError(stderr io.Writer, err error) int { return reportErrorCode(stderr, err, 1) }

func reportErrorCode(stderr io.Writer, err error, code int) int {
	fmt.Fprintf(stderr, "ctx: %v\n", err)
	return code
}
