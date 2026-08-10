//go:build !windows

package main

import "os"

func replaceFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
