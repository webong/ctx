package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/webong/ctx/internal/config"
)

var virtualizerInstanceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type virtualizerInstance struct {
	Name        string `json:"name"`
	Virtualizer string `json:"virtualizer"`
	Machine     string `json:"machine,omitempty"`
	Provider    string `json:"provider"`
	Selection   string `json:"selection"`
	Address     string `json:"address,omitempty"`
}

type virtualizerRegistry struct {
	Version   int                   `json:"version"`
	Instances []virtualizerInstance `json:"instances"`
}

func virtualizerRegistryPath() string {
	return filepath.Join(configHomePath(), "virtualizers.json")
}

func readVirtualizerRegistry() (virtualizerRegistry, error) {
	registry := virtualizerRegistry{}
	path := virtualizerRegistryPath()
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return virtualizerRegistry{Version: 1}, nil
	}
	if err != nil {
		return registry, err
	}
	if !info.Mode().IsRegular() {
		return registry, fmt.Errorf("virtualizer registry is not a regular file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return registry, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return registry, fmt.Errorf("read virtualizer registry: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return registry, fmt.Errorf("virtualizer registry has trailing data")
	}
	if registry.Version != 1 {
		return registry, fmt.Errorf("unsupported virtualizer registry version %d", registry.Version)
	}
	seen := map[string]bool{}
	for _, instance := range registry.Instances {
		if err := validateVirtualizerInstance(instance); err != nil {
			return registry, err
		}
		if seen[instance.Name] {
			return registry, fmt.Errorf("duplicate virtualizer instance %s", instance.Name)
		}
		seen[instance.Name] = true
	}
	return registry, nil
}

func validateVirtualizerInstance(instance virtualizerInstance) error {
	if !virtualizerInstanceName.MatchString(instance.Name) ||
		(instance.Virtualizer != "" && !virtualizerInstanceName.MatchString(instance.Virtualizer)) ||
		(instance.Machine != "" && !virtualizerInstanceName.MatchString(instance.Machine)) {
		return fmt.Errorf("invalid virtualizer instance name, virtualizer, or machine")
	}
	if instance.Provider == "" || instance.Selection == "" || strings.ContainsAny(instance.Provider+instance.Selection+instance.Address, "\x00\r\n") {
		return fmt.Errorf("virtualizer instance %s needs a provider and selection without control characters", instance.Name)
	}
	return nil
}

func writeVirtualizerRegistry(registry virtualizerRegistry) error {
	path := virtualizerRegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("virtualizer registry is not a regular file: %s", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	sort.Slice(registry.Instances, func(i, j int) bool { return registry.Instances[i].Name < registry.Instances[j].Name })
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ctx-virtualizers-")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceVirtualizerRegistry(temporary.Name(), path)
}

func updateVirtualizerRegistry(change func(*virtualizerRegistry) error) error {
	path := virtualizerRegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lockPath := path + ".lock"
	deadline := time.Now().Add(2 * time.Second)
	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(lock, "%d\n", os.Getpid())
			_ = lock.Close()
			defer os.Remove(lockPath)
			registry, err := readVirtualizerRegistry()
			if err != nil {
				return err
			}
			if err := change(&registry); err != nil {
				return err
			}
			return writeVirtualizerRegistry(registry)
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("virtualizer registry is busy")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func virtualizerCommand(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: virtualizer requires add, ls, show, or remove")
		return 2
	}
	registry, err := readVirtualizerRegistry()
	if err != nil {
		return reportError(stderr, err)
	}
	switch args[0] {
	case "ls", "list":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "ctx: virtualizer ls takes no arguments")
			return 2
		}
		inventory, err := resolvedMachineInventory(resolver)
		if err != nil {
			return reportError(stderr, err)
		}
		printDiscoveredVirtualizers(inventory, stdout)
		seen := map[string]bool{}
		for _, instance := range registry.Instances {
			if _, err := inventory.Find(instance.Provider, instance.Selection); err == nil {
				seen[instance.Provider+"\x00"+instance.Selection] = true
			}
		}
		for _, instance := range registry.Instances {
			state := "unavailable"
			if seen[instance.Provider+"\x00"+instance.Selection] {
				state = "discovered"
			}
			fmt.Fprintf(stdout, "@%s\t%s\t%s:%s\t%s", instance.Name, virtualizerDescription(instance), instance.Provider, instance.Selection, state)
			if instance.Address != "" {
				fmt.Fprint(stdout, " (pinned address)")
			}
			fmt.Fprintln(stdout)
		}
		return 0
	case "show":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: virtualizer show needs an instance name")
			return 2
		}
		instance, ok := findVirtualizerInstance(registry, strings.TrimPrefix(args[1], "@"))
		if !ok {
			return reportErrorCode(stderr, fmt.Errorf("unknown virtualizer instance %s", args[1]), 2)
		}
		fmt.Fprintf(stdout, "name:        @%s\n", instance.Name)
		if instance.Virtualizer != "" {
			fmt.Fprintf(stdout, "virtualizer: %s\n", instance.Virtualizer)
		}
		if instance.Machine != "" {
			fmt.Fprintf(stdout, "machine:     %s\n", instance.Machine)
		}
		fmt.Fprintf(stdout, "provider:    %s\nselection:   %s\n", instance.Provider, instance.Selection)
		if instance.Address != "" {
			fmt.Fprintf(stdout, "address:     %s\n", instance.Address)
		}
		return 0
	case "add":
		if len(args) < 2 || !virtualizerInstanceName.MatchString(args[1]) {
			fmt.Fprintln(stderr, "ctx: virtualizer add needs a valid instance name")
			return 2
		}
		instance := virtualizerInstance{Name: args[1]}
		seen := map[string]bool{}
		for index := 2; index < len(args); index += 2 {
			if index+1 >= len(args) || args[index+1] == "" || seen[args[index]] {
				fmt.Fprintln(stderr, "ctx: virtualizer add needs distinct option/value pairs")
				return 2
			}
			seen[args[index]] = true
			switch args[index] {
			case "--virtualizer":
				instance.Virtualizer = args[index+1]
			case "--machine":
				instance.Machine = args[index+1]
			case "--provider":
				instance.Provider = args[index+1]
			case "--selection":
				instance.Selection = args[index+1]
			case "--address":
				instance.Address = args[index+1]
			default:
				fmt.Fprintf(stderr, "ctx: unknown virtualizer option %s\n", args[index])
				return 2
			}
		}
		if err := validateVirtualizerInstance(instance); err != nil {
			return reportErrorCode(stderr, err, 2)
		}
		if _, ok := findVirtualizerInstance(registry, instance.Name); ok {
			return reportErrorCode(stderr, fmt.Errorf("virtualizer instance %s already exists", instance.Name), 2)
		}
		provider, err := virtualizerProvider(instance.Provider)
		if err != nil {
			return reportErrorCode(stderr, err, 2)
		}
		candidate := endpoint{Provider: provider, Name: instance.Selection, Instance: &instance}
		if code := validateEndpoint(resolver, candidate, stderr); code != 0 {
			return code
		}
		if err := updateVirtualizerRegistry(func(current *virtualizerRegistry) error {
			if _, exists := findVirtualizerInstance(*current, instance.Name); exists {
				return fmt.Errorf("virtualizer instance %s already exists", instance.Name)
			}
			current.Instances = append(current.Instances, instance)
			return nil
		}); err != nil {
			return reportError(stderr, err)
		}
		if _, err := scanMachineInventoryInGraph(resolver); err != nil {
			fmt.Fprintf(stderr, "ctx: warning: could not update virtualizer graph: %v\n", err)
		}
		fmt.Fprintf(stdout, "registered @%s (%s via %s:%s)\n", instance.Name, virtualizerDescription(instance), instance.Provider, instance.Selection)
		return 0
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "ctx: virtualizer remove needs an instance name")
			return 2
		}
		name := strings.TrimPrefix(args[1], "@")
		if err := updateVirtualizerRegistry(func(current *virtualizerRegistry) error {
			for index, instance := range current.Instances {
				if instance.Name == name {
					current.Instances = append(current.Instances[:index], current.Instances[index+1:]...)
					return nil
				}
			}
			return fmt.Errorf("unknown virtualizer instance %s", args[1])
		}); err != nil {
			return reportError(stderr, err)
		}
		if _, err := scanMachineInventoryInGraph(resolver); err != nil {
			fmt.Fprintf(stderr, "ctx: warning: could not update virtualizer graph: %v\n", err)
		}
		fmt.Fprintf(stdout, "removed @%s\n", name)
		return 0
	default:
		fmt.Fprintln(stderr, "ctx: virtualizer requires add, ls, show, or remove")
		return 2
	}
}

func findVirtualizerInstance(registry virtualizerRegistry, name string) (virtualizerInstance, bool) {
	for _, instance := range registry.Instances {
		if instance.Name == name {
			return instance, true
		}
	}
	return virtualizerInstance{}, false
}

func virtualizerDescription(instance virtualizerInstance) string {
	if instance.Virtualizer == "" {
		return instance.Provider
	}
	if instance.Machine != "" {
		return instance.Virtualizer + "/" + instance.Machine
	}
	return instance.Virtualizer
}
