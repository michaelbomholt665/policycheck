// internal/policycheck/utils/complexity.go
// Provides cyclomatic complexity calculation for Go AST nodes.

package utils

const ScopeProjectRepo = true


import (
	"go/ast"
	"go/token"
)

// CalculateComplexity calculates the cyclomatic complexity of a function declaration or literal.
// Complexity starts at 1 and increases for each control structure (if, for, range, case, &&, ||).
// Certain simple error guards and nil guards are discounted from the score.
func CalculateComplexity(n ast.Node) int {
	body := extractFunctionBody(n)
	if body == nil {
		return 0
	}

	complexity := 1
	nilChecked := make(map[string]bool)
	ast.Inspect(body, func(node ast.Node) bool {
		return inspectComplexityNode(node, nilChecked, &complexity)
	})
	return complexity
}

// inspectComplexityNode updates complexity for a visited AST node and reports whether traversal should continue.
func inspectComplexityNode(node ast.Node, nilChecked map[string]bool, complexity *int) bool {
	if node == nil {
		return false
	}
	switch typed := node.(type) {
	case *ast.IfStmt:
		*complexity += scoreIfStatement(typed, nilChecked)
	case *ast.ForStmt, *ast.RangeStmt:
		*complexity++
	case *ast.CaseClause:
		*complexity += scoreCaseClause(typed)
	case *ast.CommClause:
		*complexity += scoreCommClause(typed)
	case *ast.BinaryExpr:
		*complexity += scoreBinaryExpr(typed)
	case *ast.FuncLit:
		// Stop traversal into nested closures — they are scored independently.
		return false
	}
	return true
}

// scoreCaseClause returns the complexity delta for a case clause.
func scoreCaseClause(node *ast.CaseClause) int {
	if node.List != nil {
		return 1
	}
	return 0
}

// scoreCommClause returns the complexity delta for a comm clause.
func scoreCommClause(node *ast.CommClause) int {
	if node.Comm != nil {
		return 1
	}
	return 0
}

// scoreBinaryExpr returns the complexity delta for a binary expression.
func scoreBinaryExpr(node *ast.BinaryExpr) int {
	if node.Op == token.LAND || node.Op == token.LOR {
		return 1
	}
	return 0
}

// scoreIfStatement returns the complexity delta for an if statement, applying discounts.
func scoreIfStatement(node *ast.IfStmt, nilChecked map[string]bool) int {
	delta := 0
	if !isDiscountableErrCheck(node) && !isDiscountableNilGuard(node, nilChecked) {
		delta++
	}
	if node.Else != nil {
		if _, ok := node.Else.(*ast.IfStmt); !ok {
			delta++
		}
	}
	return delta
}

// extractFunctionBody returns the body block statement for a function decl or literal.
func extractFunctionBody(n ast.Node) *ast.BlockStmt {
	switch fn := n.(type) {
	case *ast.FuncDecl:
		return fn.Body
	case *ast.FuncLit:
		return fn.Body
	default:
		return nil
	}
}

// AnalyzeRepeatedNilGuards identifies variables checked for nil multiple times in a function body.
func AnalyzeRepeatedNilGuards(n ast.Node) map[string]int {
	body := extractFunctionBody(n)
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

// isDiscountableErrCheck returns true for simple "if err != nil { return ... err }" guards.
func isDiscountableErrCheck(n *ast.IfStmt) bool {
	if !isErrNotNilCondition(n) {
		return false
	}
	if n.Else != nil || !isErrCheckBodyValid(n.Body) {
		return false
	}
	return errCheckBodyReturnsErr(n.Body) && !hasNestedControlFlow(n.Body)
}

// isErrNotNilCondition returns true if the if-statement condition is "err != nil".
func isErrNotNilCondition(n *ast.IfStmt) bool {
	bin, ok := n.Cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	ident, ok := bin.X.(*ast.Ident)
	if !ok || ident.Name != "err" {
		return false
	}
	yIdent, ok := bin.Y.(*ast.Ident)
	return ok && yIdent.Name == "nil"
}

// isErrCheckBodyValid returns true if the body only has 1-3 statements including a return.
func isErrCheckBodyValid(body *ast.BlockStmt) bool {
	if len(body.List) == 0 || len(body.List) > 3 {
		return false
	}
	for _, stmt := range body.List {
		if _, ok := stmt.(*ast.ReturnStmt); ok {
			return true
		}
	}
	return false
}

// errCheckBodyReturnsErr returns true if the body has a return that propagates err.
func errCheckBodyReturnsErr(body *ast.BlockStmt) bool {
	var ret *ast.ReturnStmt
	for _, stmt := range body.List {
		if r, ok := stmt.(*ast.ReturnStmt); ok {
			ret = r
			break
		}
	}
	if ret == nil || len(ret.Results) == 0 {
		return false
	}
	for _, stmt := range body.List {
		if stmt == ret {
			continue
		}
		if !isBenignErrorCheckStmt(stmt) {
			return false
		}
	}
	returnsErr := false
	ast.Inspect(ret, func(inner ast.Node) bool {
		if id, ok := inner.(*ast.Ident); ok && id.Name == "err" {
			returnsErr = true
			return false
		}
		return true
	})
	return returnsErr
}

// isDiscountableNilGuard returns true for simple first-occurrence nil guards with no nested flow.
func isDiscountableNilGuard(n *ast.IfStmt, nilChecked map[string]bool) bool {
	identName, ok := repeatedNilGuardIdent(n)
	if !ok || nilChecked[identName] {
		return false
	}
	nilChecked[identName] = true
	return true
}

// isBenignErrorCheckStmt determines if a statement is a benign error check call.
func isBenignErrorCheckStmt(stmt ast.Stmt) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return false
	}
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

// repeatedNilGuardIdent extracts the identifier being checked in a nil guard condition.
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

// nilGuardIdent extracts the identifier from a nil comparison binary expression.
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

// hasNestedControlFlow detects if a block statement contains nested control flow structures.
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
