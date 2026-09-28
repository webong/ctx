package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	adapterpkg "github.com/webong/ctx/internal/adapter"
	"github.com/webong/ctx/internal/config"
)

type endpoint struct {
	Provider *adapterpkg.Adapter
	Name     string
}

func buildImage(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	providerName := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		if candidate, err := containerProvider(args[0]); err == nil {
			providerName, args = candidate.Manifest.Name, args[1:]
		}
	}
	cacheRef := ""
	for len(args) > 0 {
		switch {
		case args[0] == "--":
			args = args[1:]
			goto parsed
		case args[0] == "--cache-ref":
			if len(args) < 2 {
				fmt.Fprintln(stderr, "ctx: --cache-ref needs a registry reference")
				return 2
			}
			cacheRef, args = args[1], args[2:]
		case strings.HasPrefix(args[0], "--cache-ref="):
			cacheRef, args = strings.TrimPrefix(args[0], "--cache-ref="), args[1:]
		default:
			goto parsed
		}
	}

parsed:
	if cacheRef == "" {
		fmt.Fprintln(stderr, "ctx: build requires --cache-ref <registry-ref>")
		return 2
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: build needs build arguments")
		return 2
	}
	provider, err := containerProvider(providerName)
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	selection, err := adapterSelection(resolver, provider)
	if err != nil {
		return reportError(stderr, err)
	}
	operationArgs := append([]string{"--cache-ref", cacheRef, "--"}, args...)
	return invokeAdapter(resolver, provider, "build", selection, operationArgs, "", stdout, stderr)
}

func imageCommand(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: share:container image requires sync or copy")
		return 2
	}
	switch args[0] {
	case "sync":
		return imageSync(resolver, args[1:], stdout, stderr)
	case "copy":
		return imageCopy(resolver, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "ctx: share:container image requires sync or copy")
		return 2
	}
}

func imageSync(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	tarMode := len(args) > 0 && args[0] == "--tar"
	if tarMode {
		args = args[1:]
	}
	if len(args) < 3 {
		fmt.Fprintln(stderr, "ctx: image sync needs source context, target context, and image names")
		return 2
	}
	source, err := parseEndpointWithDefault(args[0])
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	target, err := parseEndpointWithDefault(args[1])
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	images := args[2:]
	if code := validateEndpoint(resolver, source, stderr); code != 0 {
		return code
	}
	if code := validateEndpoint(resolver, target, stderr); code != 0 {
		return code
	}
	if tarMode {
		archive, err := os.CreateTemp("", "ctx-image-*.tar")
		if err != nil {
			return reportError(stderr, err)
		}
		path := archive.Name()
		if err := archive.Close(); err != nil {
			_ = os.Remove(path)
			return reportError(stderr, err)
		}
		defer os.Remove(path)
		if code := invokeEndpoint(resolver, source, "image_save", append([]string{path}, images...), os.Stdin, stdout, stderr); code != 0 {
			return code
		}
		return invokeEndpoint(resolver, target, "image_load", []string{path}, os.Stdin, stdout, stderr)
	}
	for _, image := range images {
		if code := invokeEndpoint(resolver, source, "image_push", []string{image}, os.Stdin, stdout, stderr); code != 0 {
			return code
		}
		if code := invokeEndpoint(resolver, target, "image_pull", []string{image}, os.Stdin, stdout, stderr); code != 0 {
			return code
		}
	}
	return 0
}

func imageCopy(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "ctx: image copy needs source endpoint, target endpoint, and image names")
		return 2
	}
	source, err := parseEndpoint(args[0])
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	target, err := parseEndpoint(args[1])
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	if code := validateEndpoint(resolver, source, stderr); code != 0 {
		return code
	}
	if code := validateEndpoint(resolver, target, stderr); code != 0 {
		return code
	}
	for _, image := range args[2:] {
		archive, err := os.CreateTemp("", "ctx-image-*.tar")
		if err != nil {
			return reportError(stderr, err)
		}
		path := archive.Name()
		if err := archive.Close(); err != nil {
			_ = os.Remove(path)
			return reportError(stderr, err)
		}
		code := invokeEndpoint(resolver, source, "image_save", []string{path, image}, os.Stdin, stdout, stderr)
		if code == 0 {
			code = invokeEndpoint(resolver, target, "image_load", []string{path}, os.Stdin, stdout, stderr)
		}
		_ = os.Remove(path)
		if code != 0 {
			return code
		}
	}
	return 0
}

func volumeCommand(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: share:container volume requires export, import, or copy")
		return 2
	}
	switch args[0] {
	case "export":
		if len(args) != 3 {
			fmt.Fprintln(stderr, "ctx: volume export needs a source context and volume name")
			return 2
		}
		fmt.Fprintln(stderr, "ctx: export is only crash-consistent; stop or quiesce databases first")
		source, err := parseEndpointWithDefault(args[1])
		if err != nil {
			return reportErrorCode(stderr, err, 2)
		}
		if code := validateEndpoint(resolver, source, stderr); code != 0 {
			return code
		}
		return invokeEndpoint(resolver, source, "volume_export", []string{args[2]}, os.Stdin, stdout, stderr)
	case "import":
		if len(args) != 3 {
			fmt.Fprintln(stderr, "ctx: volume import needs a target context and new volume name")
			return 2
		}
		target, err := parseEndpointWithDefault(args[1])
		if err != nil {
			return reportErrorCode(stderr, err, 2)
		}
		if code := validateEndpoint(resolver, target, stderr); code != 0 {
			return code
		}
		return importVolume(resolver, target, args[2], os.Stdin, stdout, stderr)
	case "copy":
		if len(args) < 4 || len(args) > 5 {
			fmt.Fprintln(stderr, "ctx: volume copy needs source endpoint, target endpoint, source volume, and optional target volume")
			return 2
		}
		return volumeCopy(resolver, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "ctx: share:container volume requires export, import, or copy")
		return 2
	}
}

func volumeCopy(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	source, err := parseEndpoint(args[0])
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	target, err := parseEndpoint(args[1])
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	sourceVolume := args[2]
	targetVolume := sourceVolume
	if len(args) == 4 {
		targetVolume = args[3]
	}
	if code := validateEndpoint(resolver, source, stderr); code != 0 {
		return code
	}
	if code := validateEndpoint(resolver, target, stderr); code != 0 {
		return code
	}
	fmt.Fprintln(stderr, "ctx: copy is only crash-consistent; stop or quiesce databases first")
	exists, code := endpointVolumeExists(resolver, target, targetVolume)
	if code != 0 {
		return code
	}
	if exists {
		fmt.Fprintf(stderr, "ctx: target volume %s already exists; refusing to merge data\n", targetVolume)
		return 1
	}
	archive, err := os.CreateTemp("", "ctx-volume-*.tar")
	if err != nil {
		return reportError(stderr, err)
	}
	path := archive.Name()
	defer os.Remove(path)
	if code := invokeEndpoint(resolver, source, "volume_export", []string{sourceVolume}, os.Stdin, archive, stderr); code != 0 {
		_ = archive.Close()
		return code
	}
	if err := archive.Close(); err != nil {
		return reportError(stderr, err)
	}
	input, err := os.Open(path)
	if err != nil {
		return reportError(stderr, err)
	}
	defer input.Close()
	return createAndImportVolume(resolver, target, targetVolume, input, stdout, stderr)
}

func importVolume(resolver *config.Resolver, target endpoint, volume string, input io.Reader, stdout, stderr io.Writer) int {
	exists, code := endpointVolumeExists(resolver, target, volume)
	if code != 0 {
		return code
	}
	if exists {
		fmt.Fprintf(stderr, "ctx: target volume %s already exists; refusing to merge data\n", volume)
		return 1
	}
	return createAndImportVolume(resolver, target, volume, input, stdout, stderr)
}

func createAndImportVolume(resolver *config.Resolver, target endpoint, volume string, input io.Reader, stdout, stderr io.Writer) int {
	if code := invokeEndpoint(resolver, target, "volume_create", []string{volume}, os.Stdin, stdout, stderr); code != 0 {
		return code
	}
	return invokeEndpoint(resolver, target, "volume_import", []string{volume}, input, stdout, stderr)
}

func parseEndpoint(value string) (endpoint, error) {
	providerName, selection, ok := strings.Cut(value, ":")
	if !ok || providerName == "" || selection == "" {
		return endpoint{}, fmt.Errorf("endpoint must be <container-provider>:<context>")
	}
	provider, err := containerProvider(providerName)
	if err != nil {
		return endpoint{}, err
	}
	return endpoint{Provider: provider, Name: selection}, nil
}

func parseEndpointWithDefault(value string) (endpoint, error) {
	if strings.Contains(value, ":") {
		return parseEndpoint(value)
	}
	provider, err := containerProvider("")
	if err != nil {
		return endpoint{}, err
	}
	return endpoint{Provider: provider, Name: value}, nil
}

func containerProvider(name string) (*adapterpkg.Adapter, error) {
	if name == "" {
		store := adapterStore()
		installed, err := store.List()
		if err != nil {
			return nil, err
		}
		var defaultProvider *adapterpkg.Adapter
		for _, candidate := range installed {
			if candidate.Manifest.Kind != "container" || !candidate.Manifest.DefaultProvider {
				continue
			}
			trusted, trustErr := store.IsTrusted(candidate)
			if trustErr != nil || !trusted {
				continue
			}
			if defaultProvider != nil {
				return nil, fmt.Errorf("multiple default container providers: %s and %s", defaultProvider.Manifest.Name, candidate.Manifest.Name)
			}
			defaultProvider = candidate
		}
		if defaultProvider == nil {
			return nil, fmt.Errorf("no trusted default container provider is installed; use <provider>:<context>")
		}
		return defaultProvider, nil
	}
	provider, err := adapterStore().Load(name)
	if err != nil {
		return nil, fmt.Errorf("unknown container provider %s: %w", name, err)
	}
	if provider.Manifest.Kind != "container" {
		return nil, fmt.Errorf("adapter %s is not a container provider", name)
	}
	return provider, nil
}

func validateEndpoint(resolver *config.Resolver, value endpoint, stderr io.Writer) int {
	return invokeAdapter(resolver, value.Provider, "validate", value.Name, nil, "", io.Discard, stderr)
}

func invokeEndpoint(resolver *config.Resolver, value endpoint, operation string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if !value.Provider.HasCapability(operation) {
		fmt.Fprintf(stderr, "ctx: container provider %s does not support %s\n", value.Provider.Manifest.Name, operation)
		return 2
	}
	return invokeAdapterIO(resolver, value.Provider, operation, value.Name, args, "", stdin, stdout, stderr)
}

func endpointVolumeExists(resolver *config.Resolver, value endpoint, volume string) (bool, int) {
	if !value.Provider.HasCapability("volume_exists") {
		return false, 2
	}
	code := invokeAdapterIO(resolver, value.Provider, "volume_exists", value.Name, []string{volume}, "", os.Stdin, io.Discard, io.Discard)
	if code == 0 {
		return true, 0
	}
	if code == 1 {
		return false, 0
	}
	return false, code
}
