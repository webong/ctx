package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/webong/ctx/internal/config"
	"github.com/webong/ctx/internal/engine"
	"github.com/webong/ctx/internal/launch"
)

type endpoint struct {
	Engine string
	Name   string
}

func buildImage(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	engineName := "docker"
	if len(args) > 0 && (args[0] == "docker" || args[0] == "podman" || args[0] == "nerdctl") {
		engineName, args = args[0], args[1:]
	}
	cacheRef := ""
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		if args[0] == "--cache-ref" {
			if len(args) < 2 {
				fmt.Fprintln(stderr, "ctx: --cache-ref needs a registry reference")
				return 2
			}
			cacheRef, args = args[1], args[2:]
			continue
		}
		if strings.HasPrefix(args[0], "--cache-ref=") {
			cacheRef, args = strings.TrimPrefix(args[0], "--cache-ref="), args[1:]
			continue
		}
		break
	}
	if cacheRef == "" {
		fmt.Fprintln(stderr, "ctx: build requires --cache-ref <registry-ref>")
		return 2
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: build needs build arguments")
		return 2
	}
	var buildArgs []string
	switch engineName {
	case "docker":
		buildArgs = []string{"buildx", "build", "--cache-from", "type=registry,ref=" + cacheRef, "--cache-to", "type=registry,ref=" + cacheRef + ",mode=max"}
	case "podman":
		buildArgs = []string{"build", "--layers", "--cache-from", cacheRef, "--cache-to", cacheRef}
	case "nerdctl":
		buildArgs = []string{"build", "--cache-from", "type=registry,ref=" + cacheRef, "--cache-to", "type=registry,ref=" + cacheRef + ",mode=max"}
	}
	buildArgs = append(buildArgs, args...)
	return runSelectedEngine(resolver, engineName, buildArgs, os.Stdin, stdout, stderr)
}

func imageCommand(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: image requires sync or copy")
		return 2
	}
	switch args[0] {
	case "sync":
		return imageSync(args[1:], stdout, stderr)
	case "copy":
		return imageCopy(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "ctx: image requires sync or copy")
		return 2
	}
}

func imageSync(args []string, stdout, stderr io.Writer) int {
	tarMode := len(args) > 0 && args[0] == "--tar"
	if tarMode {
		args = args[1:]
	}
	if len(args) < 3 {
		fmt.Fprintln(stderr, "ctx: image sync needs source context, target context, and image names")
		return 2
	}
	source, target, images := args[0], args[1], args[2:]
	if code := validateEngineSelection("docker", source, stderr); code != 0 {
		return code
	}
	if code := validateEngineSelection("docker", target, stderr); code != 0 {
		return code
	}
	if tarMode {
		archive, err := os.CreateTemp("", "ctx-image-*.tar")
		if err != nil {
			return reportError(stderr, err)
		}
		path := archive.Name()
		archive.Close()
		defer os.Remove(path)
		if code := runForcedEngine("docker", source, append([]string{"image", "save", "-o", path}, images...), os.Stdin, stdout, stderr); code != 0 {
			return code
		}
		return runForcedEngine("docker", target, []string{"image", "load", "-i", path}, os.Stdin, stdout, stderr)
	}
	for _, image := range images {
		if code := runForcedEngine("docker", source, []string{"image", "push", image}, os.Stdin, stdout, stderr); code != 0 {
			return code
		}
		if code := runForcedEngine("docker", target, []string{"image", "pull", image}, os.Stdin, stdout, stderr); code != 0 {
			return code
		}
	}
	return 0
}

func imageCopy(args []string, stdout, stderr io.Writer) int {
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
	if code := validateEndpoint(source, stderr); code != 0 {
		return code
	}
	if code := validateEndpoint(target, stderr); code != 0 {
		return code
	}
	for _, image := range args[2:] {
		archive, err := os.CreateTemp("", "ctx-image-*.tar")
		if err != nil {
			return reportError(stderr, err)
		}
		path := archive.Name()
		archive.Close()
		code := endpointImageSave(source, path, image, stdout, stderr)
		if code == 0 {
			code = endpointImageLoad(target, path, stdout, stderr)
		}
		os.Remove(path)
		if code != 0 {
			return code
		}
	}
	return 0
}

func volumeCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ctx: volume requires export, import, or copy")
		return 2
	}
	switch args[0] {
	case "export":
		if len(args) != 3 {
			fmt.Fprintln(stderr, "ctx: volume export needs a source context and volume name")
			return 2
		}
		fmt.Fprintln(stderr, "ctx: export is only crash-consistent; stop or quiesce databases first")
		if code := validateEngineSelection("docker", args[1], stderr); code != 0 {
			return code
		}
		return runForcedEngine("docker", args[1], volumeExportArgs(args[2]), os.Stdin, stdout, stderr)
	case "import":
		if len(args) != 3 {
			fmt.Fprintln(stderr, "ctx: volume import needs a target context and new volume name")
			return 2
		}
		target, volume := endpoint{Engine: "docker", Name: args[1]}, args[2]
		if code := validateEndpoint(target, stderr); code != 0 {
			return code
		}
		if endpointVolumeExists(target, volume) {
			fmt.Fprintf(stderr, "ctx: target volume %s already exists; refusing to merge data\n", volume)
			return 1
		}
		if code := runEndpoint(target, []string{"volume", "create", volume}, os.Stdin, stdout, stderr); code != 0 {
			return code
		}
		return runEndpoint(target, volumeImportArgs(volume), os.Stdin, stdout, stderr)
	case "copy":
		if len(args) < 4 || len(args) > 5 {
			fmt.Fprintln(stderr, "ctx: volume copy needs source endpoint, target endpoint, source volume, and optional target volume")
			return 2
		}
		return volumeCopy(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "ctx: volume requires export, import, or copy")
		return 2
	}
}

func volumeCopy(args []string, stdout, stderr io.Writer) int {
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
	if code := validateEndpoint(source, stderr); code != 0 {
		return code
	}
	if code := validateEndpoint(target, stderr); code != 0 {
		return code
	}
	fmt.Fprintln(stderr, "ctx: copy is only crash-consistent; stop or quiesce databases first")
	if endpointVolumeExists(target, targetVolume) {
		fmt.Fprintf(stderr, "ctx: target volume %s already exists; refusing to merge data\n", targetVolume)
		return 1
	}
	archive, err := os.CreateTemp("", "ctx-volume-*.tar")
	if err != nil {
		return reportError(stderr, err)
	}
	path := archive.Name()
	defer os.Remove(path)
	if code := runEndpoint(source, volumeExportArgs(sourceVolume), os.Stdin, archive, stderr); code != 0 {
		archive.Close()
		return code
	}
	if err := archive.Close(); err != nil {
		return reportError(stderr, err)
	}
	if code := runEndpoint(target, []string{"volume", "create", targetVolume}, os.Stdin, stdout, stderr); code != 0 {
		return code
	}
	input, err := os.Open(path)
	if err != nil {
		return reportError(stderr, err)
	}
	defer input.Close()
	return runEndpoint(target, volumeImportArgs(targetVolume), input, stdout, stderr)
}

func parseEndpoint(value string) (endpoint, error) {
	engineName, name, ok := strings.Cut(value, ":")
	if !ok || name == "" || (engineName != "docker" && engineName != "podman" && engineName != "nerdctl" && engineName != "apple") {
		return endpoint{}, fmt.Errorf("endpoint must be docker:<context>, podman:<connection>, nerdctl:<namespace>, or apple:local")
	}
	return endpoint{Engine: engineName, Name: name}, nil
}

func validateEndpoint(value endpoint, stderr io.Writer) int {
	if value.Engine == "apple" {
		if value.Name != "local" {
			fmt.Fprintf(stderr, "ctx: unknown apple endpoint %s\n", value.Name)
			return 1
		}
		real, err := launch.FindReal("container")
		if err != nil {
			return reportErrorCode(stderr, err, 127)
		}
		command := exec.Command(real, "system", "version")
		command.Stdout, command.Stderr = io.Discard, io.Discard
		if err := command.Run(); err != nil {
			fmt.Fprintln(stderr, "ctx: Apple Container is unavailable")
			return 1
		}
		return 0
	}
	return validateEngineSelection(value.Engine, value.Name, stderr)
}

func runSelectedEngine(resolver *config.Resolver, engineName string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	real, err := launch.FindReal(engineName)
	if err != nil {
		return reportErrorCode(stderr, err, 127)
	}
	resolved, err := resolver.Resolve(engineName)
	if err != nil {
		return reportError(stderr, err)
	}
	command := exec.Command(real, engine.Arguments(engineName, resolved.Value, args, environment{})...)
	return runPreparedIO(command, stdin, stdout, stderr)
}

func runForcedEngine(engineName, selection string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	real, err := launch.FindReal(engineName)
	if err != nil {
		return reportErrorCode(stderr, err, 127)
	}
	prefix := map[string][]string{
		"docker":  {"--context", selection},
		"podman":  {"--connection", selection},
		"nerdctl": {"--namespace", selection},
	}[engineName]
	command := exec.Command(real, append(prefix, args...)...)
	return runPreparedIO(command, stdin, stdout, stderr)
}

func runEndpoint(value endpoint, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if value.Engine != "apple" {
		return runForcedEngine(value.Engine, value.Name, args, stdin, stdout, stderr)
	}
	real, err := launch.FindReal("container")
	if err != nil {
		return reportErrorCode(stderr, err, 127)
	}
	return runPreparedIO(exec.Command(real, args...), stdin, stdout, stderr)
}

func endpointImageSave(value endpoint, archive, image string, stdout, stderr io.Writer) int {
	args := []string{"image", "save", "-o", archive, image}
	if value.Engine == "nerdctl" {
		args = []string{"save", "-o", archive, image}
	} else if value.Engine == "apple" {
		args = []string{"image", "save", "--output", archive, image}
	}
	return runEndpoint(value, args, os.Stdin, stdout, stderr)
}

func endpointImageLoad(value endpoint, archive string, stdout, stderr io.Writer) int {
	args := []string{"image", "load", "-i", archive}
	if value.Engine == "nerdctl" {
		args = []string{"load", "-i", archive}
	} else if value.Engine == "apple" {
		if !appleImageLoadSafe() {
			fmt.Fprintln(stderr, "ctx: Apple Container image import requires container newer than 1.3.0; update it before loading archives")
			return 1
		}
		args = []string{"image", "load", "--input", archive}
	}
	return runEndpoint(value, args, os.Stdin, stdout, stderr)
}

func appleImageLoadSafe() bool {
	real, err := launch.FindReal("container")
	if err != nil {
		return false
	}
	output, err := exec.Command(real, "--version").Output()
	if err != nil {
		return false
	}
	match := regexp.MustCompile(`version\s+([0-9]+)\.([0-9]+)\.([0-9]+)`).FindStringSubmatch(string(output))
	if len(match) != 4 {
		return false
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	return major > 1 || (major == 1 && (minor > 3 || (minor == 3 && patch > 0)))
}

func endpointVolumeExists(value endpoint, volume string) bool {
	command, err := endpointExecCommand(value, []string{"volume", "inspect", volume})
	if err != nil {
		return false
	}
	command.Stdout, command.Stderr = io.Discard, io.Discard
	return command.Run() == nil
}

func endpointExecCommand(value endpoint, args []string) (*exec.Cmd, error) {
	tool := value.Engine
	if tool == "apple" {
		tool = "container"
	}
	real, err := launch.FindReal(tool)
	if err != nil {
		return nil, err
	}
	if value.Engine == "apple" {
		return exec.Command(real, args...), nil
	}
	prefix := map[string][]string{
		"docker":  {"--context", value.Name},
		"podman":  {"--connection", value.Name},
		"nerdctl": {"--namespace", value.Name},
	}[value.Engine]
	return exec.Command(real, append(prefix, args...)...), nil
}

func volumeExportArgs(volume string) []string {
	image := os.Getenv("CTX_VOLUME_IMAGE")
	if image == "" {
		image = "alpine:3.21"
	}
	return []string{"run", "--rm", "-v", volume + ":/volume:ro", image, "tar", "-C", "/volume", "-cf", "-", "."}
}

func volumeImportArgs(volume string) []string {
	image := os.Getenv("CTX_VOLUME_IMAGE")
	if image == "" {
		image = "alpine:3.21"
	}
	return []string{"run", "--rm", "-i", "-v", volume + ":/volume", image, "tar", "-C", "/volume", "-xf", "-"}
}
