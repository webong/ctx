package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// doctorManagers inspects host-side toolchains and the most recent Rancher
// Desktop boot. It never changes a manager's configuration or data.
func doctorManagers(registry managerRegistry, name string, stdout, stderr io.Writer) int {
	home, err := os.UserHomeDir()
	if err != nil {
		return reportError(stderr, err)
	}
	return doctorManagersAtHome(registry, name, home, stdout)
}

func doctorManagersAtHome(registry managerRegistry, name, home string, stdout io.Writer) int {
	failures := 0
	checked := 0
	inspectRancher := name == ""
	for _, instance := range registry.Instances {
		if name != "" && instance.Name != name {
			continue
		}
		checked++
		if instance.Virtualizer == "rancher-desktop" {
			inspectRancher = true
		}
		if err := validateManagerToolchain(instance); err != nil {
			fmt.Fprintf(stdout, "fail @%s toolchain: %v\n", instance.Name, err)
			failures++
		} else if instance.Command != "" || instance.PluginDir != "" || len(instance.Plugins) != 0 {
			fmt.Fprintf(stdout, "ok   @%s toolchain configured\n", instance.Name)
		} else {
			fmt.Fprintf(stdout, "ok   @%s uses the PATH command and global plugins\n", instance.Name)
		}
	}
	if inspectRancher && runtime.GOOS == "darwin" {
		if inspectDockerPluginOwnership(home, stdout) {
			checked++
		}
		if inspected, failed := inspectRancherBoot(home, stdout); inspected {
			checked++
			if failed {
				failures++
			}
		}
	}
	if checked == 0 {
		fmt.Fprintln(stdout, "ok   no registered managers or supported host diagnostics")
	}
	if failures != 0 {
		return 1
	}
	return 0
}

func inspectDockerPluginOwnership(home string, stdout io.Writer) bool {
	rancher := filepath.Join(home, ".rd", "bin")
	if info, err := os.Stat(rancher); err != nil || !info.IsDir() {
		return false
	}
	for _, plugin := range []string{"buildx", "compose"} {
		global := filepath.Join(home, ".docker", "cli-plugins", "docker-"+plugin)
		current, err := os.Readlink(global)
		if err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(stdout, "warn Docker %s global plugin is not a symlink; inspect %s\n", plugin, global)
			}
			continue
		}
		expected := filepath.Join(rancher, "docker-"+plugin)
		if current != expected {
			fmt.Fprintf(stdout, "warn Docker %s global plugin points to %s; Rancher Desktop expects %s. Use a manager-specific plugin directory to avoid changing the global link.\n", plugin, current, expected)
		} else {
			fmt.Fprintf(stdout, "ok   Docker %s global plugin points to Rancher Desktop\n", plugin)
		}
	}
	return true
}

func inspectRancherBoot(home string, stdout io.Writer) (bool, bool) {
	directory := filepath.Join(home, "Library", "Application Support", "rancher-desktop", "lima", "0")
	if _, err := os.Stat(directory); err != nil {
		return false, false
	}
	if free, err := managerAvailableBytes(directory); err == nil {
		if free < 5<<30 {
			fmt.Fprintf(stdout, "warn Rancher Desktop host volume has %.1f GiB available; free space before retrying the VM (do not delete its disk image).\n", float64(free)/float64(1<<30))
		} else {
			fmt.Fprintf(stdout, "ok   Rancher Desktop host volume has %.1f GiB available\n", float64(free)/float64(1<<30))
		}
	}
	serial := readManagerLogTail(filepath.Join(directory, "serialv.log"))
	status := readManagerLogTail(filepath.Join(directory, "ha.stdout.log"))
	backend := readManagerLogTail(filepath.Join(directory, "ha.stderr.log"))
	if strings.Contains(serial, "EXT4-fs") && (strings.Contains(serial, "potential data loss") || strings.Contains(serial, "error -5")) {
		fmt.Fprintln(stdout, "warn Rancher Desktop's last recorded VM boot logged EXT4 write errors; back up important data before repair or reset.")
	}
	if strings.Contains(status, "Failed to wait for guest SSH server") || strings.Contains(status, `"type":"failed"`) || strings.Contains(backend, `did not receive an event with the \"running\" status`) {
		fmt.Fprintln(stdout, "fail Rancher Desktop's last recorded VM boot did not reach guest SSH. This is a VM startup failure, not a Docker plugin symlink failure.")
		return true, true
	}
	if serial != "" || status != "" {
		fmt.Fprintln(stdout, "ok   no failed guest SSH status in Rancher Desktop's last recorded boot")
	}
	return true, false
}

func readManagerLogTail(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	const limit = 256 << 10
	if info.Size() > limit {
		if _, err := file.Seek(-limit, io.SeekEnd); err != nil {
			return ""
		}
	}
	data, _ := io.ReadAll(io.LimitReader(file, limit))
	return string(data)
}
