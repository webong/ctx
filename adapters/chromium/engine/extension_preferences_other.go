//go:build !darwin

package chromium

import "errors"

func checkMacSystemExternalPath(string) error {
	return errors.New("macOS system external extension permissions must be checked on macOS")
}
