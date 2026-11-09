//go:build !linux

package sysstat

import "errors"

func diskUsage(string) (used, total uint64, err error) {
	return 0, 0, errors.New("disk usage is only collected on Linux")
}
