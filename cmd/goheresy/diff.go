package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func printDiff(filename string, src, formatted []byte) error {
	dir, err := os.MkdirTemp("", "goheresy-diff-")
	if err != nil { return err }
	defer os.RemoveAll(dir)

	oldPath, newPath := filepath.Join(dir, "old"), filepath.Join(dir, "new")
	if err := os.WriteFile(oldPath, src, 0600); err != nil { return err }
	if err := os.WriteFile(newPath, formatted, 0600); err != nil { return err }

	name := filepath.ToSlash(filename)
	diff := "diff"
	if runtime.GOOS == "plan9" { diff = "/bin/ape/diff" }
	cmd := exec.Command(diff, "-u", "-L", name+".orig", "-L", name, oldPath, newPath)
	data, err := cmd.Output()
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) { return fmt.Errorf("computing diff: %w", err) }
	if _, err := fmt.Fprintf(os.Stdout, "diff -u %s.orig %s\n", name, name); err != nil { return err }
	_, err = os.Stdout.Write(data)
	return err
}
