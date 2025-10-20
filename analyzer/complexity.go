package analyzer

import (
	"go/ast"
	"go/token"
)

// CalculateComplexity calculates cyclomatic complexity for all functions in the package
func CalculateComplexity(pkg *ast.Package, fset *token.FileSet) []FunctionResult {
	var results []FunctionResult

	// Traverse all files in the package
	for fileName, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			// Calculate complexity for this function
			complexity := calculateFunctionComplexity(funcDecl)
			funcName := funcDecl.Name.Name

			// Add receiver type for methods
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				recv := funcDecl.Recv.List[0]
				var recvTypeName string
				switch t := recv.Type.(type) {
				case *ast.Ident:
					recvTypeName = t.Name
				case *ast.StarExpr:
					if ident, ok := t.X.(*ast.Ident); ok {
						recvTypeName = ident.Name
					}
				}
				if recvTypeName != "" {
					funcName = recvTypeName + "." + funcName
				}
			}

			results = append(results, FunctionResult{
				FuncName:   funcName,
				FilePath:   fileName,
				Complexity: complexity,
			})

			return true
		})
	}

	return results
}

// calculateFunctionComplexity calculates the cyclomatic complexity of a function
func calculateFunctionComplexity(funcDecl *ast.FuncDecl) int {
	// Start with base complexity of 1
	complexity := 1

	if funcDecl.Body == nil {
		return complexity
	}

	// Count decision points
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IfStmt:
			// Each if adds 1 to complexity
			complexity++

		case *ast.ForStmt, *ast.RangeStmt:
			// Each loop adds 1 to complexity
			complexity++

		case *ast.SwitchStmt, *ast.TypeSwitchStmt:
			// Switch statement itself adds 1
			complexity++

		case *ast.CaseClause:
			// Each case (except default) adds 1
			if node.List != nil && len(node.List) > 0 {
				complexity++
			}

		case *ast.CommClause:
			// Each case in select statement adds 1
			if node.Comm != nil {
				complexity++
			}

		case *ast.BinaryExpr:
			// Logical operators add to complexity
			if node.Op == token.LAND || node.Op == token.LOR {
				complexity++
			}
		}

		return true
	})

	return complexity
}
