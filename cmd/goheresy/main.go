package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/scanner"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kerscher/goheresy"
	"golang.org/x/tools/imports"
)

type command struct {
	goheresy.Options
	list, write, diff bool
	srcdir            string
}

func main() {
	var cmd command
	flag.BoolVar(&cmd.list, "l", false, "list files whose formatting differs")
	flag.BoolVar(&cmd.write, "w", false, "write formatted source to the input files")
	flag.BoolVar(&cmd.diff, "d", false, "display unified diffs")
	flag.BoolVar(&cmd.AllErrors, "e", false, "report all parser errors")
	flag.StringVar(&cmd.srcdir, "srcdir", "", "resolve imports relative to this directory or editor filename")
	flag.StringVar(&imports.LocalPrefix, "local", "", "comma-separated local import prefixes")
	flag.BoolVar(&cmd.FormatOnly, "format-only", false, "disable import insertion and removal")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: goheresy [flags] [path ...]")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	os.Exit(cmd.run(flag.Args()))
}

func (cmd command) run(paths []string) int {
	status := 0
	report := func(err error) {
		if err != nil {
			scanner.PrintError(os.Stderr, err)
			status = 2
		}
	}
	if len(paths) == 0 {
		src, err := io.ReadAll(os.Stdin)
		if err == nil { err = cmd.process("<standard input>", src, stdin) }
		report(err)
		return status
	}

	mode := singleFile
	if len(paths) > 1 { mode = multipleFiles }
	for _, path := range paths {
		info, err := os.Stat(path)
		switch {
		case err != nil:
			report(err)
		case info.IsDir():
			report(filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
				if err == nil && !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && strings.HasSuffix(entry.Name(), ".go") {
					err = cmd.processFile(path, multipleFiles)
				}
				report(err)
				return nil
			}))
		default:
			report(cmd.processFile(path, mode))
		}
	}
	return status
}

type inputMode int

const (
	stdin inputMode = iota
	singleFile
	multipleFiles
)

func (cmd command) target(filename string, mode inputMode) (string, error) {
	if cmd.srcdir == "" { return filename, nil }
	info, err := os.Stat(cmd.srcdir)
	if err == nil && info.Mode().IsRegular() {
		if mode == multipleFiles {
			return "", errors.New("-srcdir value can't be a file when passing multiple arguments or when walking directories")
		}
		return cmd.srcdir, nil
	}
	if mode == singleFile && strings.HasSuffix(cmd.srcdir, ".go") && (err != nil || !info.IsDir()) {
		return cmd.srcdir, nil
	}
	return filepath.Join(cmd.srcdir, filepath.Base(filename)), nil
}

func (cmd command) processFile(filename string, mode inputMode) error {
	src, err := os.ReadFile(filename)
	if err != nil { return err }
	return cmd.process(filename, src, mode)
}

func (cmd command) process(filename string, src []byte, mode inputMode) error {
	target, err := cmd.target(filename, mode)
	if err != nil { return err }
	formatted, err := cmd.Format(target, src)
	if err != nil { return err }
	if !cmd.list && !cmd.write && !cmd.diff {
		_, err := os.Stdout.Write(formatted)
		return err
	}
	if bytes.Equal(src, formatted) { return nil }
	if cmd.list {
		if _, err := fmt.Fprintln(os.Stdout, filename); err != nil { return err }
	}
	if cmd.write {
		if mode == stdin { return errors.New("can't use -w on stdin") }
		info, err := os.Stat(filename)
		if err != nil { return err }
		if err := os.WriteFile(filename, formatted, info.Mode().Perm()); err != nil { return err }
	}
	if cmd.diff {
		if mode == stdin { filename = "stdin.go" }
		return printDiff(filename, src, formatted)
	}
	return nil
}
