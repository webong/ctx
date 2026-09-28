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

const APIVersion = "1"

var validName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var reservedNames = map[string]bool{
	"docker": true, "podman": true, "nerdctl": true, "browser": true,
	"profile": true, "shell": true, "env": true, "image": true,
	"volume": true, "build": true, "adapter": true,
}

type Manifest struct {
	APIVersion   string
	Name         string
	Kind         string
	Executable   string
	Description  string
	Capabilities []string
	SelectorKey  string
	ExtraKeys    []string
	Commands     []string
	FirstParty   bool
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
		APIVersion:   values["api_version"],
		Name:         values["name"],
		Kind:         values["kind"],
		Executable:   values["executable"],
		Description:  values["description"],
		Capabilities: splitList(values["capabilities"]),
		SelectorKey:  values["selector_key"],
		ExtraKeys:    splitList(values["extra_keys"]),
		Commands:     splitList(values["commands"]),
		FirstParty:   values["first_party"] == "true",
	}
	if manifest.Kind == "" {
		manifest.Kind = "selector"
	}
	if manifest.SelectorKey == "" {
		if manifest.Kind == "browser" {
			manifest.SelectorKey = "browser"
		} else {
			manifest.SelectorKey = manifest.Name
		}
	}
	if len(manifest.Commands) == 0 && manifest.Kind == "selector" {
		manifest.Commands = []string{manifest.Name}
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
	return contains(a.Manifest.Capabilities, capability)
}

func (a *Adapter) HasCommand(command string) bool {
	return contains(a.Manifest.Commands, command)
}

func (a *Adapter) ConfigKeys() []string {
	return append([]string{a.Manifest.SelectorKey}, a.Manifest.ExtraKeys...)
}

func (a *Adapter) ExecutablePath() string {
	return filepath.Join(a.Directory, a.Manifest.Executable)
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
	if manifest.APIVersion != APIVersion {
		return fmt.Errorf("adapter %s uses unsupported API %s (expected %s)", manifest.Name, manifest.APIVersion, APIVersion)
	}
	if !validName.MatchString(manifest.Name) || reservedNames[manifest.Name] {
		return fmt.Errorf("invalid or reserved adapter name %s", manifest.Name)
	}
	if manifest.Kind != "selector" && manifest.Kind != "browser" {
		return fmt.Errorf("adapter %s has invalid kind %s", manifest.Name, manifest.Kind)
	}
	if !validName.MatchString(manifest.SelectorKey) {
		return fmt.Errorf("adapter %s has invalid selector key %s", manifest.Name, manifest.SelectorKey)
	}
	for _, key := range append(append([]string{}, manifest.ExtraKeys...), manifest.Commands...) {
		if !validName.MatchString(key) {
			return fmt.Errorf("adapter %s has invalid key or command %s", manifest.Name, key)
		}
	}
	if manifest.Executable == "" || filepath.Base(manifest.Executable) != manifest.Executable || strings.HasPrefix(manifest.Executable, ".") {
		return fmt.Errorf("adapter %s has an invalid executable name", manifest.Name)
	}
	executable := filepath.Join(directory, manifest.Executable)
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("adapter executable is missing: %s", executable)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("adapter executable is not executable: %s", executable)
	}
	if !contains(manifest.Capabilities, "validate") || !contains(manifest.Capabilities, "doctor") || (!contains(manifest.Capabilities, "run") && !contains(manifest.Capabilities, "open")) {
		return fmt.Errorf("adapter %s must provide validate, doctor, and run or open", manifest.Name)
	}
	for _, capability := range manifest.Capabilities {
		switch capability {
		case "list", "configure", "validate", "run", "doctor", "open":
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
