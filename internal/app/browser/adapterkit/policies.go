package adapterkit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/webong/ctx/internal/app/browser/share"
)

type PolicyFile struct{ Path, Level, Format string }
type PolicyRoot struct{ Path, Level string }
type PolicySources struct {
	Files       []PolicyFile
	Roots       []PolicyRoot
	RegistryKey string
}

// ExportPolicies reads only the locations declared by the calling adapter.
func ExportPolicies(sources PolicySources) (share.PolicyBundle, error) {
	bundle := share.PolicyBundle{Version: share.Version, Entries: []share.PolicyEntry{}}
	for _, file := range sources.Files {
		if err := appendPolicyFile(&bundle, file.Path, file.Level, file.Format); err != nil {
			return bundle, err
		}
	}
	for _, root := range sources.Roots {
		files, err := filepath.Glob(filepath.Join(root.Path, "*.json"))
		if err != nil {
			return bundle, err
		}
		for _, path := range files {
			if err := appendPolicyFile(&bundle, path, root.Level, "json"); err != nil {
				return bundle, err
			}
		}
	}
	if sources.RegistryKey != "" {
		for _, hive := range []string{"HKLM", "HKCU"} {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			output, err := exec.CommandContext(ctx, "reg", "query", hive+`\`+sources.RegistryKey, "/s").Output()
			cancel()
			if err == nil && len(output) > 0 {
				bundle.Entries = append(bundle.Entries, share.PolicyEntry{Location: hive + `\` + sources.RegistryKey, Level: "managed", Format: "reg-query", Content: string(output)})
			}
		}
	}
	return bundle, nil
}

func ManagedPreferenceFiles(domain string) []PolicyFile {
	return []PolicyFile{
		{Path: filepath.Join("/Library/Managed Preferences", domain+".plist"), Level: "managed", Format: "plist"},
		{Path: filepath.Join("/Library/Managed Preferences", os.Getenv("USER"), domain+".plist"), Level: "managed", Format: "plist"},
	}
}

func appendPolicyFile(bundle *share.PolicyBundle, path, level, format string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return fmt.Errorf("policy file %s is not a regular file under 2 MiB", path)
	}
	var content []byte
	if format == "plist" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		content, err = exec.CommandContext(ctx, "plutil", "-convert", "xml1", "-o", "-", path).Output()
		cancel()
	} else {
		content, err = os.ReadFile(path)
	}
	if err != nil {
		return err
	}
	if format == "json" && !json.Valid(content) {
		return fmt.Errorf("policy file %s is invalid JSON", path)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return fmt.Errorf("policy file %s contains binary data", path)
	}
	bundle.Entries = append(bundle.Entries, share.PolicyEntry{Location: path, Level: level, Format: format, Content: strings.TrimSpace(string(content))})
	return nil
}
