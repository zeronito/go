// Tests is a helper package to avoid cyclic dependency between go/ast and go/parser.
package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestIssue69183(t *testing.T) {
	const src = `package A
import (
"a"//a
"a")
`
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "test.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	ast.SortImports(fs, f) // should not panic
}

func TestSortImportsSameLastLine(t *testing.T) {
	const src = `package A
import (
"a"//a
"a")
func a() {}
`

	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "test.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	ast.SortImports(fs, f)
	fd := f.Decls[1].(*ast.FuncDecl)
	fdPos := fs.Position(fd.Pos())
	if fdPos.Column != 1 {
		t.Errorf("invalid fdPos.Column = %v; want = 1", fdPos.Column)
	}
	if fdPos.Line != 5 {
		t.Errorf("invalid fdPos.Line = %v; want = 5", fdPos.Line)
	}
}
