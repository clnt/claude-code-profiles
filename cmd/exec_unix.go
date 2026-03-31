//go:build !windows

package cmd

import "syscall"

// execvp replaces the current process with the given binary.
func execvp(bin string, argv []string, env []string) error {
	return syscall.Exec(bin, argv, env)
}
