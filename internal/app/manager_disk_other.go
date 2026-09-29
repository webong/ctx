//go:build !darwin

package app

import "fmt"

func managerAvailableBytes(string) (uint64, error) {
	return 0, fmt.Errorf("host disk check is only available on macOS")
}
