package adapter

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const APIVersion = "2.0"

const legacyAPIVersion = "1"
const legacyDecimalAPIVersion = "1.0"

var validName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var validBrowserShareOperation = regexp.MustCompile(`^[a-z]+\.[a-z]+$`)
var validEnvironmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var reservedNames = map[string]bool{
	"browser": true, "container": true, "profile": true, "shell": true, "env": true, "image": true,
	"volume": true, "build": true, "adapter": true, "share": true, "computer": true, "virtualizer": true,
}

type Manifest struct {
	APIVersion           string
	Name                 string
	Runtime              string
	Surfaces             []string
	Executable           string
	ExecutableWindows    string
	Description          string
	Capabilities         []string
	SelectorKey          string
	ExtraKeys            []string
	Commands             []string
	ComputerCommands     []string
	ComputerCapabilities []string
	ShareSpaces          []string
	BrowserShare         []string
	OverrideEnv          []string
	DefaultProvider      bool
}

type Adapter struct {
	Directory string
	Manifest  Manifest
}

type Store struct {
	Home      string
	TrustFile string
}

func NewStore(home string) *Store {
	return &Store{Home: home, TrustFile: filepath.Join(home, "trust")}
}

func LoadDirectory(directory string) (*Adapter, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("adapter directory not found: %s", directory)
	}
	if err := rejectLinks(absolute); err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(absolute, "adapter.toml")
	values, err := parseManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	manifest := Manifest{
		APIVersion:           values["api_version"],
		Name:                 values["name"],
		Runtime:              values["runtime"],
		Surfaces:             splitList(values["surfaces"]),
		Executable:           values["executable"],
		ExecutableWindows:    values["executable_windows"],
		Description:          values["description"],
		Capabilities:         splitList(values["capabilities"]),
		SelectorKey:          values["selector_key"],
		ExtraKeys:            splitList(values["extra_keys"]),
		Commands:             splitList(values["commands"]),
		ComputerCommands:     splitList(values["computer_commands"]),
		ComputerCapabilities: splitList(values["computer_capabilities"]),
		ShareSpaces:          splitList(values["share_spaces"]),
		BrowserShare:         splitList(values["browser_share"]),
		OverrideEnv:          splitList(values["override_env"]),
		DefaultProvider:      values["default_provider"] == "true",
	}
	if manifest.APIVersion == legacyAPIVersion || manifest.APIVersion == legacyDecimalAPIVersion {
		if err := normalizeLegacyManifest(&manifest, values["kind"]); err != nil {
			return nil, err
		}
	} else if values["kind"] != "" {
		return nil, fmt.Errorf("adapter %s uses removed manifest field kind; declare runtime and surfaces", manifest.Name)
	}
	if manifest.SelectorKey == "" && !hasComputerEndpoint(manifest) {
		if manifest.Runtime == "browser" {
			manifest.SelectorKey = "browser"
		} else {
			manifest.SelectorKey = manifest.Name
		}
	}
	if len(manifest.Commands) == 0 && manifest.SelectorKey != "" && manifest.Runtime != "browser" {
		manifest.Commands = []string{manifest.Name}
	}
	if contains(manifest.Capabilities, "share") && len(manifest.ShareSpaces) == 0 {
		manifest.ShareSpaces = []string{manifest.Name}
	}
	if err := validateManifest(manifest, absolute); err != nil {
		return nil, err
	}
	return &Adapter{Directory: absolute, Manifest: manifest}, nil
}

func (s *Store) Load(name string) (*Adapter, error) {
	if !validName.MatchString(name) || reservedNames[name] {
		return nil, fmt.Errorf("invalid adapter name %s", name)
	}
	loaded, err := LoadDirectory(filepath.Join(s.Home, name))
	if err != nil {
		return nil, err
	}
	if loaded.Manifest.Name != name {
		return nil, fmt.Errorf("adapter directory %s contains manifest for %s", name, loaded.Manifest.Name)
	}
	return loaded, nil
}

func (s *Store) List() ([]*Adapter, error) {
	entries, err := os.ReadDir(s.Home)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]*Adapter, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		loaded, err := s.Load(entry.Name())
		if err != nil {
			continue
		}
		result = append(result, loaded)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Manifest.Name < result[j].Manifest.Name })
	return result, nil
}

func (a *Adapter) HasCapability(capability string) bool {
	return contains(a.Manifest.Capabilities, capability) || a.HasComputerCapability(capability)
}

func (a *Adapter) HasBrowserShare(operation string) bool {
	return a.IsRuntime("browser") && contains(a.Manifest.BrowserShare, operation)
}

func (a *Adapter) IsRuntime(runtimeName string) bool {
	return a.Manifest.Runtime == runtimeName
}

func (a *Adapter) SupportsSurface(surface string) bool {
	return contains(a.Manifest.Surfaces, surface)
}

func (a *Adapter) IsSelectable() bool {
	return a.Manifest.SelectorKey != ""
}

func (a *Adapter) HasCommand(command string) bool {
	return contains(a.Manifest.Commands, command)
}

func (a *Adapter) HasComputerCommand(command string) bool {
	return contains(a.Manifest.ComputerCommands, command)
}

func (a *Adapter) HasComputerCapability(capability string) bool {
	return contains(a.Manifest.ComputerCapabilities, capability)
}

func (a *Adapter) IsComputerEndpoint() bool {
	return hasComputerEndpoint(a.Manifest)
}

func hasComputerEndpoint(manifest Manifest) bool {
	return len(manifest.ComputerCommands) > 0 || len(manifest.ComputerCapabilities) > 0
}

func normalizeLegacyManifest(manifest *Manifest, kind string) error {
	if kind == "" {
		if hasComputerEndpoint(*manifest) {
			manifest.Runtime = "computer"
			manifest.Surfaces = []string{"shell"}
			return nil
		}
		kind = "selector"
	}
	switch kind {
	case "selector":
		manifest.Runtime = "computer"
		manifest.Surfaces = []string{"shell"}
	case "browser":
		manifest.Runtime = "browser"
		manifest.Surfaces = []string{"web"}
	case "container":
		manifest.Runtime = "virtualizer"
		manifest.Surfaces = []string{"shell"}
	default:
		return fmt.Errorf("adapter %s has invalid legacy kind %s", manifest.Name, kind)
	}
	return nil
}

func (a *Adapter) ConfigKeys() []string {
	keys := append([]string(nil), a.Manifest.ExtraKeys...)
	if a.Manifest.SelectorKey != "" {
		keys = append([]string{a.Manifest.SelectorKey}, keys...)
	}
	return keys
}

func (a *Adapter) ExecutablePath() string {
	return a.ExecutablePathForOS(runtime.GOOS)
}

func (a *Adapter) ExecutablePathForOS(goos string) string {
	executable := a.Manifest.Executable
	if goos == "windows" && a.Manifest.ExecutableWindows != "" {
		executable = a.Manifest.ExecutableWindows
	}
	return filepath.Join(a.Directory, executable)
}

func (a *Adapter) Checksum() (string, error) {
	if err := rejectLinks(a.Directory); err != nil {
		return "", err
	}
	var files []string
	err := filepath.WalkDir(a.Directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			relative, err := filepath.Rel(a.Directory, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	outer := sha256.New()
	for _, relative := range files {
		contents, err := os.ReadFile(filepath.Join(a.Directory, filepath.FromSlash(relative)))
		if err != nil {
			return "", err
		}
		inner := sha256.Sum256(contents)
		fmt.Fprintf(outer, "./%s %s\n", relative, hex.EncodeToString(inner[:]))
	}
	return hex.EncodeToString(outer.Sum(nil)), nil
}

func (s *Store) IsTrusted(a *Adapter) (bool, error) {
	checksum, err := a.Checksum()
	if err != nil {
		return false, err
	}
	trust, err := readTrust(s.TrustFile)
	if err != nil {
		return false, err
	}
	return trust[a.Manifest.Name] == checksum, nil
}

func (s *Store) AssertTrusted(a *Adapter) error {
	trusted, err := s.IsTrusted(a)
	if err != nil {
		return err
	}
	if !trusted {
		return fmt.Errorf("adapter %s is not trusted or changed; run ctx adapter trust %s after reviewing it", a.Manifest.Name, a.Manifest.Name)
	}
	return nil
}

func (s *Store) Trust(a *Adapter) error {
	checksum, err := a.Checksum()
	if err != nil {
		return err
	}
	trust, err := readTrust(s.TrustFile)
	if err != nil {
		return err
	}
	trust[a.Manifest.Name] = checksum
	return writeTrust(s.TrustFile, trust)
}

func (s *Store) RemoveTrust(name string) error {
	trust, err := readTrust(s.TrustFile)
	if err != nil {
		return err
	}
	delete(trust, name)
	return writeTrust(s.TrustFile, trust)
}

func (s *Store) Install(source string) (*Adapter, error) {
	loaded, err := LoadDirectory(source)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.Home, 0o700); err != nil {
		return nil, err
	}
	target := filepath.Join(s.Home, loaded.Manifest.Name)
	if _, err := os.Lstat(target); err == nil {
		return nil, fmt.Errorf("adapter %s is already installed", loaded.Manifest.Name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	staging, err := os.MkdirTemp(s.Home, ".install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)
	if err := copyDirectory(loaded.Directory, staging); err != nil {
		return nil, err
	}
	if err := os.Rename(staging, target); err != nil {
		return nil, err
	}
	return s.Load(loaded.Manifest.Name)
}

// Replace validates and stages an adapter before swapping it into the store.
// If the final rename fails, the previous adapter is restored.
func (s *Store) Replace(source string) (*Adapter, error) {
	loaded, err := LoadDirectory(source)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.Home, 0o700); err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp(s.Home, ".replace-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)
	if err := copyDirectory(loaded.Directory, staging); err != nil {
		return nil, err
	}
	if _, err := LoadDirectory(staging); err != nil {
		return nil, err
	}

	target := filepath.Join(s.Home, loaded.Manifest.Name)
	backupRoot, err := os.MkdirTemp(s.Home, ".backup-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(backupRoot)
	backup := filepath.Join(backupRoot, loaded.Manifest.Name)
	hadTarget := false
	if _, err := os.Lstat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return nil, err
		}
		hadTarget = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.Rename(staging, target); err != nil {
		if hadTarget {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return nil, errors.Join(err, fmt.Errorf("restore previous adapter: %w", restoreErr))
			}
		}
		return nil, err
	}
	replaced, err := s.Load(loaded.Manifest.Name)
	if err == nil {
		return replaced, nil
	}
	if removeErr := os.RemoveAll(target); removeErr != nil {
		return nil, errors.Join(err, fmt.Errorf("remove invalid replacement: %w", removeErr))
	}
	if hadTarget {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return nil, errors.Join(err, fmt.Errorf("restore previous adapter: %w", restoreErr))
		}
	}
	return nil, err
}

func (s *Store) Remove(name string) error {
	loaded, err := s.Load(name)
	if err != nil {
		return err
	}
	if filepath.Clean(loaded.Directory) != filepath.Join(filepath.Clean(s.Home), name) {
		return errors.New("refusing to remove adapter outside adapter home")
	}
	if err := os.RemoveAll(loaded.Directory); err != nil {
		return err
	}
	return s.RemoveTrust(name)
}

func parseManifest(path string) (map[string]string, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("adapter manifest not found: %s", path)
	}
	defer input.Close()
	values := map[string]string{}
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		raw := strings.TrimSpace(parts[1])
		value, err := strconv.Unquote(raw)
		if err != nil {
			continue
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func validateManifest(manifest Manifest, directory string) error {
	if manifest.APIVersion != APIVersion && manifest.APIVersion != legacyAPIVersion && manifest.APIVersion != legacyDecimalAPIVersion {
		return fmt.Errorf("adapter %s uses unsupported API %s (expected %s)", manifest.Name, manifest.APIVersion, APIVersion)
	}
	if !validName.MatchString(manifest.Name) || reservedNames[manifest.Name] {
		return fmt.Errorf("invalid or reserved adapter name %s", manifest.Name)
	}
	if manifest.Runtime != "computer" && manifest.Runtime != "virtualizer" && manifest.Runtime != "browser" {
		return fmt.Errorf("adapter %s has invalid runtime %s", manifest.Name, manifest.Runtime)
	}
	if len(manifest.Surfaces) == 0 {
		return fmt.Errorf("adapter %s must declare at least one surface", manifest.Name)
	}
	seenSurfaces := map[string]bool{}
	for _, surface := range manifest.Surfaces {
		if surface != "shell" && surface != "web" || seenSurfaces[surface] {
			return fmt.Errorf("adapter %s has invalid or duplicate surface %s", manifest.Name, surface)
		}
		seenSurfaces[surface] = true
	}
	if hasComputerEndpoint(manifest) && manifest.Runtime != "computer" {
		return fmt.Errorf("adapter %s computer endpoint requires the computer runtime", manifest.Name)
	}
	if manifest.DefaultProvider && manifest.Runtime != "virtualizer" {
		return fmt.Errorf("adapter %s can only be a default provider for the virtualizer runtime", manifest.Name)
	}
	if !hasComputerEndpoint(manifest) && manifest.SelectorKey == "" {
		return fmt.Errorf("adapter %s must declare a selector key", manifest.Name)
	}
	if manifest.SelectorKey != "" && !validName.MatchString(manifest.SelectorKey) {
		return fmt.Errorf("adapter %s has invalid selector key %s", manifest.Name, manifest.SelectorKey)
	}
	for _, key := range append(append(append([]string{}, manifest.ExtraKeys...), manifest.Commands...), manifest.ComputerCommands...) {
		if !validName.MatchString(key) {
			return fmt.Errorf("adapter %s has invalid key or command %s", manifest.Name, key)
		}
	}
	seenComputerCapabilities := map[string]bool{}
	for _, capability := range manifest.ComputerCapabilities {
		if capability != "hook" && capability != "plugin" || seenComputerCapabilities[capability] {
			return fmt.Errorf("adapter %s has invalid or duplicate computer capability %s", manifest.Name, capability)
		}
		seenComputerCapabilities[capability] = true
	}
	if len(manifest.ShareSpaces) > 0 && !contains(manifest.Capabilities, "share") {
		return fmt.Errorf("adapter %s declares share spaces without the share capability", manifest.Name)
	}
	if len(manifest.BrowserShare) > 0 && (manifest.Runtime != "browser" || !contains(manifest.Capabilities, "share")) {
		return fmt.Errorf("adapter %s browser_share requires a browser runtime with share capability", manifest.Name)
	}
	seenBrowserShare := map[string]bool{}
	for _, operation := range manifest.BrowserShare {
		if !validBrowserShareOperation.MatchString(operation) || seenBrowserShare[operation] {
			return fmt.Errorf("adapter %s has invalid browser share operation %s", manifest.Name, operation)
		}
		seenBrowserShare[operation] = true
	}
	seenShareSpaces := map[string]bool{}
	for _, space := range manifest.ShareSpaces {
		if !validName.MatchString(space) || space == "virtualizer" || space == "container" || (space == "browser" && manifest.Runtime != "browser") || space == "computer" || seenShareSpaces[space] {
			return fmt.Errorf("adapter %s has invalid or duplicate share space %s", manifest.Name, space)
		}
		seenShareSpaces[space] = true
	}
	for _, key := range manifest.OverrideEnv {
		if !validEnvironmentName.MatchString(key) {
			return fmt.Errorf("adapter %s has invalid override environment variable %s", manifest.Name, key)
		}
	}
	for platform, executableName := range map[string]string{"default": manifest.Executable, "windows": manifest.ExecutableWindows} {
		if platform == "windows" && executableName == "" {
			continue
		}
		if executableName == "" || filepath.Base(executableName) != executableName || strings.HasPrefix(executableName, ".") {
			return fmt.Errorf("adapter %s has an invalid %s executable name", manifest.Name, platform)
		}
		executable := filepath.Join(directory, executableName)
		info, err := os.Stat(executable)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("adapter executable is missing: %s", executable)
		}
		if platform == "default" && runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("adapter executable is not executable: %s", executable)
		}
	}
	if !contains(manifest.Capabilities, "validate") || !contains(manifest.Capabilities, "doctor") {
		return fmt.Errorf("adapter %s must provide validate and doctor", manifest.Name)
	}
	if !contains(manifest.Capabilities, "run") && !contains(manifest.Capabilities, "open") {
		return fmt.Errorf("adapter %s must provide run or open", manifest.Name)
	}
	if contains(manifest.Capabilities, "run") && !contains(manifest.Surfaces, "shell") {
		return fmt.Errorf("adapter %s provides run without the shell surface", manifest.Name)
	}
	if contains(manifest.Capabilities, "open") && !contains(manifest.Surfaces, "web") {
		return fmt.Errorf("adapter %s provides open without the web surface", manifest.Name)
	}
	if (len(manifest.Commands) > 0 || len(manifest.ComputerCommands) > 0) && !contains(manifest.Surfaces, "shell") {
		return fmt.Errorf("adapter %s declares commands without the shell surface", manifest.Name)
	}
	if len(manifest.ComputerCommands) > 0 && !contains(manifest.Capabilities, "run") {
		return fmt.Errorf("adapter %s declares computer commands without the run capability", manifest.Name)
	}
	for _, capability := range manifest.Capabilities {
		switch capability {
		case "list", "configure", "validate", "run", "doctor", "open", "build", "share",
			"image_push", "image_pull", "image_save", "image_load",
			"volume_exists", "volume_create", "volume_export", "volume_import":
		default:
			return fmt.Errorf("adapter %s declares unknown capability %s", manifest.Name, capability)
		}
	}
	return nil
}

func rejectLinks(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("adapter packages cannot contain symbolic links: %s", path)
		}
		return nil
	})
}

func copyDirectory(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported adapter package entry: %s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			input.Close()
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		return closeErr
	})
}

func readTrust(path string) (map[string]string, error) {
	result := map[string]string{}
	input, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer input.Close()
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 {
			result[fields[0]] = fields[1]
		}
	}
	return result, scanner.Err()
}

func writeTrust(path string, trust map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".trust-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	names := make([]string, 0, len(trust))
	for name := range trust {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(temporary, "%s %s\n", name, trust[name]); err != nil {
			temporary.Close()
			return err
		}
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceFile(temporaryPath, path)
}

func splitList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
