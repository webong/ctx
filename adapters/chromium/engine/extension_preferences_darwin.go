//go:build darwin

package chromium

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

// Chrome rejects an all-users external preferences path with untrusted
// ownership, world-writable components, or symbolic links. Check before
// creating a request and again after creating its file; do not repair an
// existing administrator-owned path as a side effect.
func checkMacSystemExternalPath(path string) error {
	groups := map[uint32]bool{}
	for _, name := range []string{"admin", "wheel"} {
		group, err := user.LookupGroup(name)
		if err != nil {
			return fmt.Errorf("cannot resolve macOS group %s for external extension permissions: %w", name, err)
		}
		id, err := strconv.ParseUint(group.Gid, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid macOS group ID for %s", name)
		}
		groups[uint32(id)] = true
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || stat.Uid != 0 || !groups[stat.Gid] || info.Mode().Perm()&0002 != 0 || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("macOS system external extension path %s must be owned by root with group admin or wheel, must not be world-writable, and must not contain symlinks; an administrator must correct this path", current)
			}
			if current != path && !info.IsDir() {
				return fmt.Errorf("macOS system external extension parent is not a directory: %s", current)
			}
			if info.IsDir() && info.Mode().Perm()&0001 == 0 {
				return fmt.Errorf("macOS system external extension directory %s must be traversable by all Chrome users; an administrator must correct this path", current)
			}
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return nil
}
