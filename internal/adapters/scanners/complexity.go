package scanners

import (
	"go/ast"
	"go/token"
)

type complexityWalker struct {
	complexity int
	nilChecked map[string]bool
}

// calculateComplexity calculates the cyclomatic complexity of a function declaration or literal.
func calculateComplexity(n ast.Node) int {
	var body *ast.BlockStmt
	switch fn := n.(type) {
	case *ast.FuncDecl:
		body = fn.Body
	case *ast.FuncLit:
		body = fn.Body
	}

	if body == nil {
		return 0
	}

	w := &complexityWalker{
		complexity: 1,
		nilChecked: make(map[string]bool),
	}

	ast.Walk(w, body)

	return w.complexity
}

// analyzeRepeatedNilGuards identifies variables that are repeatedly checked for nil
// within the same function body or literal.
func analyzeRepeatedNilGuards(n ast.Node) map[string]int {
	var body *ast.BlockStmt
	switch fn := n.(type) {
	case *ast.FuncDecl:
		body = fn.Body
	case *ast.FuncLit:
		body = fn.Body
	}

	if body == nil {
		return nil
	}

	seen := make(map[string]int)
	repeated := make(map[string]int)
	ast.Inspect(body, func(node ast.Node) bool {
		ifStmt, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}

		ident, ok := repeatedNilGuardIdent(ifStmt)
		if !ok {
			return true
		}

		seen[ident]++
		if seen[ident] > 1 {
			repeated[ident] = seen[ident]
		}

		return true
	})

	return repeated
}

// Visit implements the ast.Visitor interface to walk the AST and calculate complexity.
func (w *complexityWalker) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}

	switch node := n.(type) {
	case *ast.IfStmt:
		if !w.isDiscountableErrCheck(node) && !w.isDiscountableNilGuard(node) {
			w.complexity++
		}
		if node.Else != nil {
			// else if is handled by the recursive walk (Visit will be called for the IfStmt in Else)
			// but a plain else { ... } also adds complexity.
			if _, ok := node.Else.(*ast.IfStmt); !ok {
				w.complexity++
			}
		}
	case *ast.ForStmt:
		w.complexity++
	case *ast.RangeStmt:
		w.complexity++
	case *ast.CaseClause:
		// Increment for each case in a switch.
		if node.List != nil {
			w.complexity++
		}
	case *ast.CommClause:
		// Increment for each case in a select.
		if node.Comm != nil {
			w.complexity++
		}
	case *ast.BinaryExpr:
		if node.Op == token.LAND || node.Op == token.LOR {
			w.complexity++
		}
	case *ast.FuncLit:
		// Do not walk into anonymous functions; they are analyzed as separate facts.
		return nil
	}
	return w
}

// isDiscountableErrCheck returns true if the if statement is a plain
// "if err != nil { return ... err ... }" guard with no nested logic or error swallowing.
func (w *complexityWalker) isDiscountableErrCheck(n *ast.IfStmt) bool {
	// Must be if err != nil
	bin, ok := n.Cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	ident, ok := bin.X.(*ast.Ident)
	if !ok || ident.Name != "err" {
		return false
	}
	yIdent, ok := bin.Y.(*ast.Ident)
	if !ok || yIdent.Name != "nil" {
		return false
	}

	// No else branch allowed
	if n.Else != nil {
		return false
	}

	// Body must be up to 3 statements
	if len(n.Body.List) == 0 || len(n.Body.List) > 3 {
		return false
	}

	// Must find a return statement that returns err
	var ret *ast.ReturnStmt
	for _, stmt := range n.Body.List {
		if r, ok := stmt.(*ast.ReturnStmt); ok {
			ret = r
			break
		}
	}

	if ret == nil {
		return false
	}

	// Other statements must be benign (logging, metrics)
	for _, stmt := range n.Body.List {
		if stmt == ret {
			continue
		}
		if !isBenignErrorCheckStmt(stmt) {
			return false
		}
	}

	// Return must not be empty.
	if len(ret.Results) == 0 {
		return false
	}

	// Check that the return still propagates the current err value.
	returnsErr := false
	ast.Inspect(ret, func(inner ast.Node) bool {
		if ident, ok := inner.(*ast.Ident); ok && ident.Name == "err" {
			returnsErr = true
			return false
		}
		return true
	})

	if !returnsErr {
		return false
	}

	// Body must contain no nested control flow.
	hasNested := false
	ast.Inspect(n.Body, func(inner ast.Node) bool {
		if inner == nil || inner == n.Body {
			return true
		}
		switch inner.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			hasNested = true
			return false
		}
		return true
	})

	return !hasNested
}

// isBenignErrorCheckStmt returns true for non-penalizing statements like logging or metrics.
func isBenignErrorCheckStmt(stmt ast.Stmt) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Logging/metrics typically don't branch
	hasControl := false
	ast.Inspect(call, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.SelectStmt, *ast.FuncLit:
			hasControl = true
			return false
		}
		return true
	})

	return !hasControl
}

// isDiscountableNilGuard returns true if the if statement is a plain
// "if x != nil" or "if x == nil" guard with no nested logic,
// and it's not a repeated check on the same variable.
func (w *complexityWalker) isDiscountableNilGuard(n *ast.IfStmt) bool {
	identName, ok := repeatedNilGuardIdent(n)
	if !ok {
		return false
	}

	// Check for repeated nil guards (exceptions to the exception)
	if w.nilChecked[identName] {
		return false
	}
	w.nilChecked[identName] = true

	return true
}

// repeatedNilGuardIdent returns the name of the variable being guarded
// if the if statement is a simple nil guard.
func repeatedNilGuardIdent(n *ast.IfStmt) (string, bool) {
	bin, ok := n.Cond.(*ast.BinaryExpr)
	if !ok || (bin.Op != token.NEQ && bin.Op != token.EQL) {
		return "", false
	}

	ident, ok := nilGuardIdent(bin)
	if !ok {
		return "", false
	}
	if n.Else != nil || hasNestedControlFlow(n.Body) {
		return "", false
	}

	return ident.Name, true
}

// nilGuardIdent returns the identifier involved in a comparison with nil.
func nilGuardIdent(bin *ast.BinaryExpr) (*ast.Ident, bool) {
	xIdent, xOk := bin.X.(*ast.Ident)
	yIdent, yOk := bin.Y.(*ast.Ident)
	if xOk && yOk && yIdent.Name == "nil" {
		return xIdent, true
	}
	if xOk && yOk && xIdent.Name == "nil" {
		return yIdent, true
	}

	return nil, false
}

// hasNestedControlFlow checks if a block of statements contains any nested
// control flow structures like if, for, range, or switch.
func hasNestedControlFlow(body *ast.BlockStmt) bool {
	hasNested := false
	ast.Inspect(body, func(inner ast.Node) bool {
		if inner == nil || inner == body {
			return true
		}
		switch inner.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			hasNested = true
			return false
		}
		return true
	})

	return hasNested
}
