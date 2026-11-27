// Package goheresy formats Go source and compacts single-statement blocks.
package goheresy

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"unicode/utf8"

	"golang.org/x/tools/imports"
)

const maxCompactLine = 120

// Format formats src using filename as the import-resolution context.
// A nil src denotes empty input.
func Format(filename string, src []byte) ([]byte, error) {
	return Options{}.Format(filename, src)
}

// Options controls the upstream formatting stage.
type Options struct {
	AllErrors  bool
	FormatOnly bool
}

// Format applies these options and then compacts eligible blocks.
// It accepts complete files and source fragments.
func (o Options) Format(filename string, src []byte) ([]byte, error) {
	if src == nil { src = []byte{} }
	formatted, err := imports.Process(filename, src, &imports.Options{
		Fragment:   true,
		AllErrors:  o.AllErrors,
		Comments:   true,
		TabIndent:  true,
		TabWidth:   8,
		FormatOnly: o.FormatOnly,
	})
	if err != nil { return nil, err }
	if len(formatted) == 0 { return []byte{}, nil }
	return compact(filename, formatted)
}

// parseSource supplies the context omitted by declaration and statement
// fragments. Keep the original bytes intact so offsets can be translated.
func parseSource(filename string, src []byte) (*ast.File, *token.File, int, error) {
	const mode = parser.ParseComments | parser.SkipObjectResolution
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, mode)
	if err == nil { return file, fset.File(file.Pos()), 0, nil }
	for _, prefix := range []string{"package p;\n", "package p; func _() {\n"} {
		wrapped := append([]byte(prefix), src...)
		if prefix != "package p;\n" { wrapped = append(wrapped, '\n', '}') }
		file, wrapErr := parser.ParseFile(fset, filename, wrapped, mode)
		if wrapErr == nil { return file, fset.File(file.Pos()), len(prefix), nil }
	}
	return nil, nil, 0, err
}

func compact(filename string, src []byte) ([]byte, error) {
	file, positions, prefix, err := parseSource(filename, src)
	if err != nil { return nil, err }
	offset := func(pos token.Pos) int { return positions.Offset(pos) - prefix }
	var out []byte
	var stack []ast.Node
	next := 0
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, node)
		block, ok := node.(*ast.BlockStmt)
		if !ok || len(block.List) != 1 { return true }

		header := block.Lbrace
		switch parent := stack[len(stack)-2].(type) {
		case *ast.FuncDecl, *ast.FuncLit, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return true
		case *ast.IfStmt:
			header = parent.Pos()
			if block != parent.Body { header = parent.Body.End() }
		case *ast.ForStmt, *ast.RangeStmt:
			header = parent.Pos()
		}
		stmt := block.List[0]
		if _, empty := stmt.(*ast.EmptyStmt); empty { return true }
		open, close := offset(block.Lbrace), offset(block.Rbrace)
		if open < next || bytes.ContainsAny(src[offset(header):open], "\r\n") { return true }
		stmtStart, stmtEnd := offset(stmt.Pos()), offset(stmt.End())
		if stmtStart <= open || stmtEnd > close { return true }
		before, after := src[open+1:stmtStart], src[stmtEnd:close]
		statement := src[stmtStart:stmtEnd]
		if !oneLineBreak(before) || !oneLineBreak(after) || bytes.ContainsAny(statement, "\r\n") { return true }
		comment := sort.Search(len(file.Comments), func(i int) bool {
			return file.Comments[i].End() > header
		})
		if comment < len(file.Comments) && file.Comments[comment].Pos() < block.End() { return true }

		leading := src[next : open+1]
		lineStart := bytes.LastIndexByte(leading, '\n') + 1
		width := utf8.RuneCount(leading[lineStart:])
		if lineStart == 0 {
			// An earlier compaction may already occupy this line.
			width += utf8.RuneCount(out[bytes.LastIndexByte(out, '\n')+1:])
		}
		trailing, _, _ := bytes.Cut(src[close+1:], []byte{'\n'})
		width += utf8.RuneCount(statement) + utf8.RuneCount(trailing) + 3 // Two spaces and the closing brace.
		if width > maxCompactLine { return true }

		out = append(out, leading...)
		out = append(out, ' ')
		out = append(out, statement...)
		out = append(out, ' ', '}')
		next = close + 1
		return true
	})
	if next == 0 { return src, nil }
	return append(out, src[next:]...), nil
}

func oneLineBreak(src []byte) bool {
	return len(bytes.TrimSpace(src)) == 0 && bytes.Count(src, []byte{'\n'}) == 1
}
