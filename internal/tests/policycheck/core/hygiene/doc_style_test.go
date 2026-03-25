// internal/tests/policycheck/core/hygiene/doc_style_test.go
package hygiene_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"policycheck/internal/policycheck/core/hygiene"
)

func TestHasDocWithPrefix(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		symbol   string
		expected bool
	}{
		{
			"valid comment",
			"// ValidateSchema validates the schema\nfunc ValidateSchema() {}",
			"ValidateSchema",
			true,
		},
		{
			"invalid prefix",
			"// This validates the schema\nfunc ValidateSchema() {}",
			"ValidateSchema",
			false,
		},
		{
			"no comment",
			"func ValidateSchema() {}",
			"ValidateSchema",
			false,
		},
		{
			"tab separator",
			"// ValidateSchema\tvalidates the schema\nfunc ValidateSchema() {}",
			"ValidateSchema",
			true,
		},
		{
			"exact match",
			"// ValidateSchema\nfunc ValidateSchema() {}",
			"ValidateSchema",
			true,
		},
		{
			"block comment valid",
			"/* ValidateSchema validates */\nfunc ValidateSchema() {}",
			"ValidateSchema",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", "package test\n"+tt.code, parser.ParseComments)
			assert.NoError(t, err)

			var doc *ast.CommentGroup
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == tt.symbol {
					doc = fn.Doc
				}
			}

			assert.Equal(t, tt.expected, hygiene.HasDocWithPrefix(doc, tt.symbol))
		})
	}
}
