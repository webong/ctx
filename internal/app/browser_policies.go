package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/webong/ctx/internal/config"
)

type browserPolicyEntry struct {
	Location string `json:"location"`
	Level    string `json:"level"`
	Format   string `json:"format"`
	Content  string `json:"content"`
}

type browserPolicyBundle struct {
	Version int                  `json:"version"`
	Source  string               `json:"source"`
	Entries []browserPolicyEntry `json:"entries"`
}

func shareBrowserPolicyCommand(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "export" {
		fmt.Fprintln(stderr, "ctx: share:browser policy requires export")
		return 2
	}
	flags := flag.NewFlagSet("share:browser policy export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	from := flags.String("from", "", "source browser:profile")
	toFile := flags.String("to-file", "", "new file for the policy bundle")
	toStdout := flags.Bool("stdout", false, "write the policy bundle to a pipe")
	if err := flags.Parse(args[1:]); err != nil || len(flags.Args()) != 0 || (*toFile == "") == !*toStdout {
		fmt.Fprintln(stderr, "ctx: policy export needs exactly one of --to-file or --stdout")
		return 2
	}
	source, err := resolveBrowserSource(resolver, *from)
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	if *toStdout {
		if file, ok := stdout.(*os.File); ok {
			info, err := file.Stat()
			if err != nil {
				return reportError(stderr, err)
			}
			if info.Mode()&os.ModeNamedPipe == 0 {
				fmt.Fprintln(stderr, "ctx: --stdout requires a pipe; use --to-file for a protected file")
				return 2
			}
		}
	} else if _, err := os.Lstat(*toFile); err == nil {
		fmt.Fprintln(stderr, "ctx: output file already exists")
		return 1
	} else if !errors.Is(err, os.ErrNotExist) {
		return reportError(stderr, err)
	}
	var bundle browserPolicyBundle
	if err := browserAdapterShare(resolver, source, "policy", "export", browserShareRequest{}, &bundle, stderr); err != nil {
		return reportError(stderr, err)
	}
	if bundle.Version != 1 {
		return reportError(stderr, errors.New("adapter returned an unsupported policy bundle version"))
	}
	bundle.Source = source.Adapter.Manifest.Name + ":" + source.Profile
	if *toStdout {
		if err := json.NewEncoder(stdout).Encode(bundle); err != nil {
			return reportError(stderr, err)
		}
		return 0
	}
	if err := writePrivateJSON(*toFile, bundle); err != nil {
		return reportError(stderr, err)
	}
	fmt.Fprintf(stdout, "exported %d policy sources into %s (mode 0600)\n", len(bundle.Entries), *toFile)
	return 0
}

func exportNativeBrowserPolicies(provider string) (browserPolicyBundle, error) {
	bundle := browserPolicyBundle{Version: 1, Entries: []browserPolicyEntry{}}
	var roots []struct{ Path, Level string }
	switch runtime.GOOS {
	case "linux":
		switch provider {
		case "chrome":
			roots = []struct{ Path, Level string }{{"/etc/opt/chrome/policies/managed", "managed"}, {"/etc/opt/chrome/policies/recommended", "recommended"}}
		case "chromium":
			roots = []struct{ Path, Level string }{{"/etc/chromium/policies/managed", "managed"}, {"/etc/chromium/policies/recommended", "recommended"}, {"/etc/chromium-browser/policies/managed", "managed"}, {"/etc/chromium-browser/policies/recommended", "recommended"}}
		case "firefox":
			for _, path := range []string{"/etc/firefox/policies/policies.json", "/usr/lib/firefox/distribution/policies.json", "/usr/lib64/firefox/distribution/policies.json"} {
				if err := appendPolicyFile(&bundle, path, "managed", "json"); err != nil {
					return bundle, err
				}
			}
		default:
			return bundle, fmt.Errorf("%s policy export is unavailable on Linux", provider)
		}
	case "darwin":
		domain := map[string]string{"chrome": "com.google.Chrome", "chromium": "org.chromium.Chromium", "firefox": "org.mozilla.firefox", "safari": "com.apple.Safari"}[provider]
		if domain == "" {
			return bundle, fmt.Errorf("%s policy export is unavailable on macOS", provider)
		}
		for _, path := range []string{filepath.Join("/Library/Managed Preferences", domain+".plist"), filepath.Join("/Library/Managed Preferences", os.Getenv("USER"), domain+".plist")} {
			if err := appendPolicyFile(&bundle, path, "managed", "plist"); err != nil {
				return bundle, err
			}
		}
		if provider == "firefox" {
			for _, path := range []string{"/Applications/Firefox.app/Contents/Resources/distribution/policies.json", filepath.Join(os.Getenv("HOME"), "Applications/Firefox.app/Contents/Resources/distribution/policies.json")} {
				if err := appendPolicyFile(&bundle, path, "managed", "json"); err != nil {
					return bundle, err
				}
			}
		}
	case "windows":
		key := map[string]string{"chrome": `Software\Policies\Google\Chrome`, "chromium": `Software\Policies\Chromium`, "firefox": `Software\Policies\Mozilla\Firefox`}[provider]
		if key == "" {
			return bundle, fmt.Errorf("%s policy export is unavailable on Windows", provider)
		}
		for _, hive := range []string{"HKLM", "HKCU"} {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			output, err := exec.CommandContext(ctx, "reg", "query", hive+`\`+key, "/s").Output()
			cancel()
			if err == nil && len(output) > 0 {
				bundle.Entries = append(bundle.Entries, browserPolicyEntry{Location: hive + `\` + key, Level: "managed", Format: "reg-query", Content: string(output)})
			}
		}
		return bundle, nil
	default:
		return bundle, fmt.Errorf("%s policy export is unavailable on %s", provider, runtime.GOOS)
	}
	for _, root := range roots {
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
	return bundle, nil
}

func appendPolicyFile(bundle *browserPolicyBundle, path, level, format string) error {
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
	bundle.Entries = append(bundle.Entries, browserPolicyEntry{Location: path, Level: level, Format: format, Content: strings.TrimSpace(string(content))})
	return nil
}

func writePrivateJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(file).Encode(value); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}
