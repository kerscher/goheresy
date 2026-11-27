package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type invocation struct {
	stdout, stderr string
	status         int
}

func invoke(t *testing.T, exe, dir, input string, args ...string) invocation {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	status := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) { t.Fatal(err) }
		status = exit.ExitCode()
	}
	return invocation{stdout.String(), stderr.String(), status}
}

func TestCLI(t *testing.T) {
	bin := t.TempDir()
	exe := filepath.Join(bin, "goheresy")
	if runtime.GOOS == "windows" { exe += ".exe" }
	if out, err := exec.Command("go", "build", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build command: %v\n%s", err, out)
	}

	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(path, []byte(src), 0644); err != nil { t.Fatal(err) }
	}
	read := func(name string) string {
		t.Helper()
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil { t.Fatal(err) }
		return string(src)
	}
	const unformatted = "package p\nvar x=1\n"
	const standard = "package p\n\nvar x = 1\n"
	const invalid = "package p\nfunc f( {\n"
	const guard = "package p\nfunc f(err error) error { if err != nil { return err }; return nil }\n"
	const compact = "package p\n\nfunc f(err error) error {\n\tif err != nil { return err }\n\treturn nil\n}\n"
	const defaultImports = "package p\n\nimport \"fmt\"\n\nvar _ = fmt.Sprint(1)\n"
	const importRepair = "package p\nimport \"strings\"\nvar _=fmt.Sprint(1)\n"
	const formatOnly = "package p\n\nimport \"strings\"\n\nvar _ = fmt.Sprint(1)\n"
	const groupInput = "package p\nimport (\"fmt\"; _ \"example.org/remote\"; _ \"example.org/local/pkg\")\nvar _=fmt.Sprint(1)\n"
	const grouped = "package p\n\nimport (\n\t\"fmt\"\n\n\t_ \"example.org/remote\"\n\n\t_ \"example.org/local/pkg\"\n)\n\nvar _ = fmt.Sprint(1)\n"
	type testCase struct {
		name, input string
		args        []string
		status      int
		stdout      string
		stderr      string
		file        string
		diff        bool
	}
	tests := []testCase{
		{name: "stdin", input: unformatted, stdout: standard},
		{name: "unchanged stdin write", input: standard, args: []string{"-w"}},
		{name: "stdin list and write", input: unformatted, args: []string{"-l", "-w", "-d"}, status: 2, stdout: "<standard input>\n", stderr: "can't use -w on stdin"},
		{name: "stdin diff", input: unformatted, args: []string{"-d"}, stdout: "diff -u stdin.go.orig stdin.go\n", diff: true},
		{name: "file", args: []string{"x.go"}, stdout: standard},
		{name: "explicit non-go file", args: []string{"tree/notes.txt"}, stdout: standard},
		{name: "directory", args: []string{"-l", "tree"}, stdout: strings.Join([]string{filepath.Join("tree", ".sub", "a.go"), filepath.Join("tree", "_b.go"), filepath.Join("tree", "b.go"), ""}, "\n")},
		{name: "write and list", args: []string{"-l", "-w", "x.go"}, stdout: "x.go\n", file: standard},
		{name: "write list and diff", args: []string{"-l", "-w", "-d", "x.go"}, stdout: "x.go\ndiff -u x.go.orig x.go\n", file: standard, diff: true},
		{name: "missing and valid", args: []string{"missing.go", "x.go"}, status: 2, stdout: standard, stderr: "missing.go"},
		{name: "invalid write", args: []string{"-w", "bad.go"}, status: 2, stderr: "bad.go:"},
		{name: "all parser errors", args: []string{"-e", "bad.go"}, status: 2, stderr: "bad.go:"},
		{name: "srcdir stdin filename", input: invalid, args: []string{"-srcdir", "x.go"}, status: 2, stderr: "x.go:"},
		{name: "srcdir directory", args: []string{"-srcdir", "tree", "bad.go"}, status: 2, stderr: filepath.Join("tree", "bad.go") + ":"},
		{name: "srcdir new filename", args: []string{"-srcdir", "new.go", "bad.go"}, status: 2, stderr: "new.go:"},
		{name: "srcdir multiple files", args: []string{"-srcdir", "x.go", "x.go", "bad.go"}, status: 2, stderr: "can't be a file"},
		{name: "srcdir directory walk", args: []string{"-srcdir", "x.go", "tree"}, status: 2, stderr: "can't be a file"},
		{name: "import repair", input: importRepair, stdout: defaultImports},
		{name: "format only", input: importRepair, args: []string{"-format-only"}, stdout: formatOnly},
		{name: "local", input: groupInput, args: []string{"-local", "example.org/local,example.net"}, stdout: grouped},
	}

	var upstream string
	if os.Getenv("GOHERESY_TEST_UPSTREAM") != "" {
		cmd := exec.Command("go", "install", "golang.org/x/tools/cmd/goimports@v0.38.0")
		cmd.Env = append(os.Environ(), "GOBIN="+bin, "GOTELEMETRY=off")
		if out, err := cmd.CombinedOutput(); err != nil { t.Fatalf("build upstream: %v\n%s", err, out) }
		upstream = filepath.Join(bin, "goimports"+filepath.Ext(exe))
	}

	// Upstream temporary-file timestamps vary across invocations.
	normaliseDiff := func(s string) string {
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") { lines[i], _, _ = strings.Cut(line, "\t") }
		}
		return strings.Join(lines, "\n")
	}

	write("tree/.hidden.go", invalid)
	write("tree/.sub/a.go", unformatted)
	write("tree/_b.go", unformatted)
	write("tree/b.go", unformatted)
	write("tree/notes.txt", unformatted)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			check := func(binary string) invocation {
				write("x.go", unformatted)
				write("bad.go", invalid)
				got := invoke(t, binary, dir, test.input, test.args...)
				if read("bad.go") != invalid { t.Fatal("invalid source was changed") }
				wantFile := test.file
				if wantFile == "" { wantFile = unformatted }
				if got := read("x.go"); got != wantFile { t.Fatalf("x.go = %q, want %q", got, wantFile) }
				got.stdout = normaliseDiff(got.stdout)
				return got
			}
			got := check(exe)
			if got.status != test.status || !strings.Contains(got.stderr, test.stderr) || (test.stderr == "" && got.stderr != "") {
				t.Fatalf("invocation = %+v", got)
			}
			if test.diff {
				if !strings.HasPrefix(got.stdout, test.stdout) || !strings.Contains(got.stdout, "+var x = 1\n") {
					t.Fatalf("diff = %q", got.stdout)
				}
			} else if got.stdout != test.stdout { t.Fatalf("stdout = %q, want %q", got.stdout, test.stdout) }
			if upstream != "" {
				if want := check(upstream); got != want { t.Fatalf("goheresy = %+v\ngoimports = %+v", got, want) }
			}
		})
	}
	t.Run("compact stdin", func(t *testing.T) {
		if got := invoke(t, exe, dir, guard); got != (invocation{stdout: compact}) { t.Fatalf("invocation = %+v", got) }
	})
	t.Run("compact in-place", func(t *testing.T) {
		write("guard.go", guard)
		if got := invoke(t, exe, dir, "", "-w", "guard.go"); got != (invocation{}) || read("guard.go") != compact {
			t.Fatalf("invocation = %+v, guard.go = %q", got, read("guard.go"))
		}
		if got := invoke(t, exe, dir, "", "-l", "guard.go"); got != (invocation{}) {
			t.Fatalf("repeated invocation = %+v", got)
		}
	})
}
