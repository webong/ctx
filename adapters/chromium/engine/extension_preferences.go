package chromium

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// makeExternalDirectories sets Chrome's documented traversal permissions on
// newly created directories even when the installer has a restrictive umask.
// Existing directories retain their permissions and ownership.
func makeExternalDirectories(directory string) error {
	var missing []string
	for current := directory; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("external extensions parent is not a directory: %s", current)
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, current)
		if filepath.Dir(current) == current {
			return errors.New("external extensions path has no existing parent")
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0755); err != nil {
			if os.IsExist(err) {
				info, err := os.Lstat(missing[i])
				if err == nil && info.IsDir() {
					continue
				}
			}
			return err
		}
		if err := os.Chmod(missing[i], 0755); err != nil {
			return err
		}
	}
	return nil
}

func checkExternalPreferenceFile(path string, macSystem bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("external extension preference must be a regular file, not a symlink")
	}
	if macSystem {
		return checkMacSystemExternalPath(path)
	}
	return nil
}
