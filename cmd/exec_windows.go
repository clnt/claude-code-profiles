//go:build windows

package cmd

import (
	"os"
	"os/exec"
)

// execvp on Windows can't replace the process, so we run as a child instead.
func execvp(bin string, argv []string, env []string) error {
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
}
