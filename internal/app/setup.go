package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	adapterpkg "github.com/webong/ctx/internal/adapter"
)

func catalogStore() *adapterpkg.Store {
	home := os.Getenv("CTX_CATALOG_HOME")
	if home == "" {
		home = filepath.Join(configHomePath(), "catalog", "adapters")
	}
	return adapterpkg.NewStore(home)
}

func setupCommand(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "adapters" {
		args = args[1:]
	}
	mode, selection := "interactive", ""
	for len(args) > 0 {
		switch {
		case args[0] == "--all" && mode == "interactive":
			mode, args = "all", args[1:]
		case args[0] == "--minimal" && mode == "interactive":
			mode, args = "minimal", args[1:]
		case args[0] == "--adapters" && mode == "interactive" && len(args) >= 2:
			mode, selection, args = "selected", args[1], args[2:]
		case strings.HasPrefix(args[0], "--adapters=") && mode == "interactive":
			mode, selection, args = "selected", strings.TrimPrefix(args[0], "--adapters="), args[1:]
		default:
			return setupUsage(stderr)
		}
	}

	if mode == "minimal" {
		fmt.Fprintln(stdout, "No new adapters selected. Existing adapters were preserved.")
		return 0
	}
	available, err := catalogStore().List()
	if err != nil {
		return reportError(stderr, err)
	}
	if len(available) == 0 {
		fmt.Fprintln(stderr, "ctx: adapter catalog is empty; reinstall ctx or set CTX_CATALOG_HOME")
		return 1
	}
	if mode == "interactive" {
		selection, err = promptAdapterSelection(available, input, stdout)
		if err != nil {
			fmt.Fprintln(stderr, "ctx: no interactive input; use --adapters, --all, or --minimal")
			return 2
		}
		switch selection {
		case "", "none":
			fmt.Fprintln(stdout, "No adapters selected.")
			return 0
		case "all":
			mode = "all"
		}
	}

	names := catalogAdapterNames(available)
	if mode != "all" {
		names, err = parseAdapterSelection(selection, available)
		if err != nil {
			return reportErrorCode(stderr, err, 2)
		}
	}
	for _, name := range names {
		if _, err := addCatalogAdapter(name); err != nil {
			return reportError(stderr, err)
		}
		fmt.Fprintf(stdout, "installed adapter %s\n", name)
	}
	return 0
}

func setupUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "ctx: setup accepts adapters, --all, --minimal, or --adapters <name,...>")
	return 2
}

func promptAdapterSelection(available []*adapterpkg.Adapter, input io.Reader, output io.Writer) (string, error) {
	installed := installedAdapterNames()
	fmt.Fprintln(output, "Available adapters:")
	for index, candidate := range available {
		state := ""
		if installed[candidate.Manifest.Name] {
			state = " [installed]"
		}
		fmt.Fprintf(output, "  %2d) %-12s %-9s %s%s\n", index+1, candidate.Manifest.Name, candidate.Manifest.Kind, candidate.Manifest.Description, state)
	}
	fmt.Fprint(output, "Select names or numbers separated by commas (all/none): ")
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func parseAdapterSelection(value string, available []*adapterpkg.Adapter) ([]string, error) {
	byName := make(map[string]bool, len(available))
	for _, candidate := range available {
		byName[candidate.Manifest.Name] = true
	}
	seen := map[string]bool{}
	var names []string
	for _, token := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		name := token
		if index, err := strconv.Atoi(token); err == nil {
			if index < 1 || index > len(available) {
				return nil, fmt.Errorf("adapter number %d is out of range", index)
			}
			name = available[index-1].Manifest.Name
		}
		if !byName[name] {
			return nil, fmt.Errorf("adapter %s is not in the installed catalog", name)
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, errors.New("no adapters selected")
	}
	return names, nil
}

func catalogAdapterNames(available []*adapterpkg.Adapter) []string {
	names := make([]string, 0, len(available))
	for _, candidate := range available {
		names = append(names, candidate.Manifest.Name)
	}
	return names
}

func installedAdapterNames() map[string]bool {
	result := map[string]bool{}
	installed, err := adapterStore().List()
	if err != nil {
		return result
	}
	for _, candidate := range installed {
		result[candidate.Manifest.Name] = true
	}
	return result
}

func addCatalogAdapter(name string) (*adapterpkg.Adapter, error) {
	source, err := catalogStore().Load(name)
	if err != nil {
		return nil, fmt.Errorf("adapter %s is not available in the catalog: %w", name, err)
	}
	if err := checkShimConflicts(source); err != nil {
		return nil, err
	}
	store := adapterStore()
	var installed *adapterpkg.Adapter
	if _, loadErr := store.Load(name); loadErr == nil {
		installed, err = store.Replace(source.Directory)
	} else {
		installed, err = store.Install(source.Directory)
	}
	if err != nil {
		return nil, err
	}
	if err := store.Trust(installed); err != nil {
		return nil, err
	}
	if err := installAdapterShims(installed); err != nil {
		return nil, err
	}
	return installed, nil
}

func refreshCatalogAdapters(stdout io.Writer) error {
	installed, err := adapterStore().List()
	if err != nil {
		return err
	}
	for _, candidate := range installed {
		if _, err := catalogStore().Load(candidate.Manifest.Name); err != nil {
			continue
		}
		if _, err := addCatalogAdapter(candidate.Manifest.Name); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "refreshed adapter %s\n", candidate.Manifest.Name)
	}
	return nil
}

func binaryDirectory() string {
	if directory := os.Getenv("CTX_BIN_DIR"); directory != "" {
		return directory
	}
	executable, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(executable)
}

func adapterShimSources(candidate *adapterpkg.Adapter) map[string]string {
	result := map[string]string{}
	commands := append([]string(nil), candidate.Manifest.Commands...)
	commands = append(commands, candidate.Manifest.ComputerCommands...)
	for _, command := range commands {
		filename := command
		if runtime.GOOS == "windows" {
			filename += ".cmd"
		}
		path := filepath.Join(candidate.Directory, filename)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			result[filename] = path
		}
	}
	return result
}

func checkShimConflicts(candidate *adapterpkg.Adapter) error {
	var previous *adapterpkg.Adapter
	if installed, err := adapterStore().Load(candidate.Manifest.Name); err == nil {
		previous = installed
	}
	for filename, source := range adapterShimSources(candidate) {
		target := filepath.Join(binaryDirectory(), filename)
		same, err := sameFileContents(source, target)
		if err == nil && same {
			continue
		}
		if err == nil && previous != nil {
			if oldSource := adapterShimSources(previous)[filename]; oldSource != "" {
				if oldSame, oldErr := sameFileContents(oldSource, target); oldErr == nil && oldSame {
					continue
				}
			}
		}
		if err == nil {
			return fmt.Errorf("%s exists and is not the ctx-managed %s shim", target, candidate.Manifest.Name)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func installAdapterShims(candidate *adapterpkg.Adapter) error {
	if err := os.MkdirAll(binaryDirectory(), 0o755); err != nil {
		return err
	}
	for filename, source := range adapterShimSources(candidate) {
		contents, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o755)
		if runtime.GOOS == "windows" {
			mode = 0o644
		}
		if err := writeManagedFile(filepath.Join(binaryDirectory(), filename), contents, mode); err != nil {
			return err
		}
	}
	return nil
}

func removeAdapterShims(candidate *adapterpkg.Adapter) error {
	for filename, source := range adapterShimSources(candidate) {
		target := filepath.Join(binaryDirectory(), filename)
		same, err := sameFileContents(source, target)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if same {
			if err := os.Remove(target); err != nil {
				return err
			}
		}
	}
	return nil
}

func sameFileContents(left, right string) (bool, error) {
	leftContents, err := os.ReadFile(left)
	if err != nil {
		return false, err
	}
	rightContents, err := os.ReadFile(right)
	if err != nil {
		return false, err
	}
	return bytes.Equal(leftContents, rightContents), nil
}

func writeManagedFile(path string, contents []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ctx-shim-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(temporaryPath, path)
}
