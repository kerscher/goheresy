package goheresy

import (
	"bytes"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/imports"
)

func TestBlocks(t *testing.T) {
	tests := []struct {
		name, body string
		compact    int
	}{
		{"single", "if err != nil { return err }", 1},
		{"multiple", "if err != nil { return nil, err }", 1},
		{"bare", "if err != nil { return }", 1},
		{"initialiser", "if err := f(); err != nil { return err }", 1},
		{"wrapped", "if err != nil { return fmt.Errorf(\"load: %w\", err) }", 1},
		{"parentheses", "if ((err) != (nil)) { return err }", 1},
		{"adjacent", "if err != nil {return err}; if err != nil {return err}", 2},
		{"nested", "if ready { if err != nil { return err } }", 1},
		{"else if", "if ready { f() } else if err != nil { return err }", 2},
		{"literal", "_ = func() error { if err != nil { return err }; return nil }", 1},
		{"nested return", "if err != nil { return func() error { if err != nil { return err }; return nil }() }", 1},
		{"already compact", "if err != nil { return err; }", 1},
		{"before and after", "// before\nif err != nil { return err } // after\n// next\nf()", 1},
		{"body comment", "if err != nil { // opening\nreturn err\n}", 0},
		{"trailing return comment", "if err != nil { return err // why\n}", 0},
		{"header comment", "if err /* allocation */ != nil { return err }", 0},
		{"block comment", "if err != nil { /* note */ return err }", 0},
		{"condition spans lines", "if err !=\n nil { return err }", 0},
		{"multiline return", "if err != nil { return Result{\nField: 1,\n}, err }", 0},
		{"multiline initialiser", "if err := f(\n1,\n); err != nil { return err }", 0},
		{"multiple statements", "if err != nil { log.Print(err); return err }", 0},
		{"else", "if err != nil { return err } else { f() }", 2},
		{"arbitrary conditions", "if nil != err {return err}; if x != nil {return err}; if err == nil {return err}", 3},
		{"non-return", "if err != nil { panic(err) }", 1},
		{"increment", "if ok { n++ }", 1},
		{"assignment", "if !ok { found = true }", 1},
		{"for", "for i := 0; i < n; i++ { f(i) }", 1},
		{"range", "for _, x := range xs { f(x) }", 1},
		{"standalone block", "{ f() }", 1},
		{"empty", "if ready {}", 0},
		{"switch", "switch x { case 1: }", 0},
		{"select", "select { default: }", 0},
		{"unicode", "if err != nil { return Résultat{\"東京\"}, err }", 1},
		{"raw", "if err != nil { return `a ;} // b`, err }", 1},
		{"multiline raw", "if err != nil { return `a\nb`, err }", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			src := []byte("package p\nfunc f() {\n" + test.body + "\n}\n")
			out := verifyFormat(t, "test.go", src)
			if count := bytes.Count(out, []byte(" }")); count != test.compact {
				t.Fatalf("got %d compact blocks, want %d\n%s", count, test.compact, out)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "test.go", out, parser.ParseComments); err != nil { t.Fatal(err) }
		})
	}
}

func verifyFormat(t *testing.T, name string, src []byte) []byte {
	t.Helper()
	out, err := Format(name, src)
	if err != nil { t.Fatal(err) }
	checkInvariants(t, name, src, out)
	return out
}

func checkInvariants(t *testing.T, name string, src, out []byte) {
	t.Helper()
	second, err := Format(name, out)
	if err != nil || !bytes.Equal(second, out) {
		t.Fatalf("idempotence failed: %v\nfirst:\n%s\nsecond:\n%s", err, out, second)
	}
	opts := &imports.Options{Fragment: true, Comments: true, TabIndent: true, TabWidth: 8}
	want, err := imports.Process(name, src, opts)
	if err != nil { t.Fatal(err) }
	got, err := imports.Process(name, out, opts)
	if err != nil || !bytes.Equal(got, want) { t.Fatalf("normalisation failed: %v\nwant:\n%s\ngot:\n%s", err, want, got) }
}

func TestInputs(t *testing.T) {
	for _, src := range [][]byte{nil, {}, []byte(" \n\t")} {
		out, err := Format("does-not-exist.go", src)
		if err != nil || out == nil || !bytes.Equal(out, src) { t.Fatalf("Format(%q) = %q, %v", src, out, err) }
	}
	if _, err := Format("broken.go", []byte("package p\nfunc f( {")); err == nil { t.Fatal("invalid source accepted") }
	for _, src := range []string{
		"  if err != nil {return err}\n",
		"func f() error { if err != nil {return err}; return nil }",
		"// comment\nif err != nil {return err} // after\n",
	} {
		out := verifyFormat(t, "fragment.go", []byte(src))
		if !bytes.Contains(out, []byte("if err != nil { return err }")) { t.Fatalf("fragment guard not compacted: %s", out) }
	}
}

func TestBlockLayout(t *testing.T) {
	src := []byte(`package p

func target(cmd command, filename string) (string, error) {
	if cmd.srcdir == "" {
		return filename, nil
	}
	return "", nil
}

func format(src, formatted []byte) error {
	if bytes.Equal(src, formatted) {
		return nil
	}
	return err
}

func available(ok bool) bool {
	if !ok {
		return true
	}
	return false
}

func unchanged() {
	f()
}
`)
	want := string(src)
	for _, pair := range [][2]string{
		{"if cmd.srcdir == \"\" {\n\t\treturn filename, nil\n\t}", "if cmd.srcdir == \"\" { return filename, nil }"},
		{"if bytes.Equal(src, formatted) {\n\t\treturn nil\n\t}", "if bytes.Equal(src, formatted) { return nil }"},
		{"if !ok {\n\t\treturn true\n\t}", "if !ok { return true }"},
	} {
		want = strings.Replace(want, pair[0], pair[1], 1)
	}
	want = strings.Replace(want, "package p\n\n", "package p\n\nimport \"bytes\"\n\n", 1)
	if got := verifyFormat(t, "test.go", src); string(got) != want { t.Fatalf("want:\n%s\ngot:\n%s", want, got) }
}

func TestCompactionLineLimit(t *testing.T) {
	const left = "\tif ok { use(\""
	const right = "\") }"
	for _, test := range []struct {
		name, char, suffix string
		length             int
		fold               bool
	}{
		{"below", "x", "", 119, true},
		{"at limit", "x", "", 120, true},
		{"above", "x", "", 121, false},
		{"Unicode at limit", "é", "", 120, true},
		{"trailing comment at limit", "x", " // note", 120, true},
		{"trailing comment above limit", "x", " // note", 121, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := strings.Repeat(test.char, test.length-len(left+right+test.suffix))
			stmt := "use(\"" + payload + "\")"
			const header = "package p\n\nfunc f() {\n"
			const footer = "\n}\n"
			src := header + "\tif ok {\n\t\t" + stmt + "\n\t}" + test.suffix + footer
			want := src
			if test.fold { want = header + left + payload + right + test.suffix + footer }
			got := verifyFormat(t, "test.go", []byte(src))
			if string(got) != want { t.Fatalf("want:\n%s\ngot:\n%s", want, got) }
		})
	}

	t.Run("combined branches", func(t *testing.T) {
		stmt := "use(\"" + strings.Repeat("x", 60) + "\")"
		src := "package p\n\nfunc f() {\n\tif ok {\n\t\t" + stmt + "\n\t} else {\n\t\t" + stmt + "\n\t}\n}\n"
		want := strings.Replace(src, "if ok {\n\t\t"+stmt+"\n\t}", "if ok { "+stmt+" }", 1)
		got := verifyFormat(t, "test.go", []byte(src))
		if string(got) != want { t.Fatalf("want:\n%s\ngot:\n%s", want, got) }
	})
}

func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.in")
	if err != nil || len(inputs) == 0 { t.Fatalf("golden inputs: %v", err) }
	for _, input := range inputs {
		name := strings.TrimSuffix(input, ".in")
		t.Run(filepath.Base(name), func(t *testing.T) {
			src, err := os.ReadFile(input)
			if err != nil { t.Fatal(err) }
			want, err := os.ReadFile(name + ".golden")
			if err != nil { t.Fatal(err) }
			if got := verifyFormat(t, name+".go", src); !bytes.Equal(got, want) { t.Fatalf("want:\n%s\ngot:\n%s", want, got) }
		})
	}
}

func TestCorpus(t *testing.T) {
	if os.Getenv("GOHERESY_TEST_CORPUS") == "" { t.Skip("set GOHERESY_TEST_CORPUS=1") }
	count := 0
	for _, pkg := range []string{"bytes", "encoding/json", "go/ast", "go/parser", "go/printer", "os"} {
		dir := filepath.Join(runtime.GOROOT(), "src", pkg)
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil { return err }
			if entry.IsDir() && path != dir { return filepath.SkipDir }
			if strings.HasSuffix(path, ".go") {
				src, err := os.ReadFile(path)
				if err != nil { return err }
				verifyFormat(t, path, src)
				count++
			}
			return nil
		})
		if err != nil { t.Fatal(err) }
	}
	t.Logf("verified %d files", count)
}

func FuzzFormat(f *testing.F) {
	for _, seed := range []string{
		"package p\nfunc f(err error) error { if err != nil {return err}; return nil }",
		"package p\nfunc f(err error) { if err != nil { /* why */ return } }",
		"package p\nfunc f(err error) (string, error) { if err != nil {\nreturn `a\nb`, err\n}; return \"\", nil }\n",
		"package p\nfunc f(",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		const name = "fuzz.go"
		// The upstream statement-fragment formatter is not idempotent for
		// some multiline raw strings because it reapplies indentation.
		if _, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution); err != nil { return }
		// Isolate syntax and layout from filesystem-dependent import discovery.
		formatter := Options{FormatOnly: true}
		out, err := formatter.Format(name, []byte(src))
		if err != nil { return }
		got, err := formatter.Format(name, out)
		if err != nil || !bytes.Equal(got, out) { t.Fatalf("idempotence failed: %v\nfirst:\n%s\nsecond:\n%s", err, out, got) }
		opts := &imports.Options{Fragment: true, Comments: true, TabIndent: true, TabWidth: 8, FormatOnly: true}
		want, err := imports.Process(name, []byte(src), opts)
		if err != nil { t.Fatal(err) }
		got, err = imports.Process(name, out, opts)
		if err != nil || !bytes.Equal(got, want) { t.Fatalf("normalisation failed: %v\nwant:\n%s\ngot:\n%s", err, want, got) }
	})
}
