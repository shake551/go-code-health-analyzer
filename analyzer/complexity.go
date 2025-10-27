package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// CalculateComplexity calculates cyclomatic complexity for all functions in the package
func CalculateComplexity(pkg *ast.Package, fset *token.FileSet, projectPrefix string) []FunctionResult {
	var results []FunctionResult

	// Traverse all files in the package
	for fileName, file := range pkg.Files {
		// Build import map for this file
		fileImports := buildFileImportMap(file)

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

			// Calculate LoC for this function
			loc := CalculateFunctionLoC(funcDecl, fset)

			// Extract dependencies for this function
			deps := extractFunctionDependencies(funcDecl, fileImports, projectPrefix)
			internalDeps, externalDeps := CategorizeDependencies(deps, projectPrefix)

			// Ce (Efferent): Count of unique packages this function depends on
			efferent := len(deps)

			results = append(results, FunctionResult{
				FuncName:        funcName,
				FilePath:        fileName,
				Complexity:      complexity,
				LoC:             loc,
				Dependencies:    deps,
				InternalDeps:    internalDeps,
				ExternalDeps:    externalDeps,
				DependencyCount: len(deps),
				Efferent:        efferent,
				Afferent:        0, // Will be calculated later in a second pass
				Instability:     0, // Will be calculated later
			})

			return true
		})
	}

	// Calculate afferent coupling (Ca) for each function
	// Build a call graph to see which functions call which
	calculateAfferentCoupling(results, pkg)

	// Calculate instability for each function
	for i := range results {
		total := results[i].Afferent + results[i].Efferent
		if total > 0 {
			results[i].Instability = float64(results[i].Efferent) / float64(total)
		}
	}

	return results
}

// calculateAfferentCoupling calculates how many functions call each function
func calculateAfferentCoupling(functions []FunctionResult, pkg *ast.Package) {
	// Create a map for quick lookup
	funcMap := make(map[string]*FunctionResult)
	for i := range functions {
		funcMap[functions[i].FuncName] = &functions[i]
	}

	// Extract all function names in this package for matching
	localFunctions := make(map[string]bool)
	for _, f := range functions {
		localFunctions[f.FuncName] = true
	}

	// Traverse all functions and find function calls
	for _, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			// Get the name of the current function
			callerName := funcDecl.Name.Name
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
					callerName = recvTypeName + "." + callerName
				}
			}

			// Check if this is a function we're tracking
			if !localFunctions[callerName] {
				return true
			}

			// Find all function calls within this function
			if funcDecl.Body != nil {
				ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
					callExpr, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}

					// Extract the called function name
					var calledName string
					switch fun := callExpr.Fun.(type) {
					case *ast.Ident:
						// Direct function call: funcName()
						calledName = fun.Name
					case *ast.SelectorExpr:
						// Method call or package.Function() call
						if ident, ok := fun.X.(*ast.Ident); ok {
							// Could be method call or package call
							// Check if it's a method call (receiver is a local variable/type)
							if localFunctions[ident.Name+"."+fun.Sel.Name] {
								calledName = ident.Name + "." + fun.Sel.Name
							}
						}
					}

					// If we found a local function being called, increment its afferent count
					if calledName != "" && localFunctions[calledName] {
						if calledFunc, exists := funcMap[calledName]; exists {
							calledFunc.Afferent++
						}
					}

					return true
				})
			}

			return true
		})
	}
}

// buildFileImportMap creates a mapping from package name/alias to full import path
func buildFileImportMap(file *ast.File) map[string]string {
	importMap := make(map[string]string)

	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Determine the package name/alias
		var pkgName string
		if imp.Name != nil {
			// Explicit alias
			pkgName = imp.Name.Name
		} else {
			// Use last component of import path as package name
			parts := strings.Split(importPath, "/")
			pkgName = parts[len(parts)-1]
		}

		importMap[pkgName] = importPath
	}

	return importMap
}

// extractFunctionDependencies extracts package dependencies from a function
func extractFunctionDependencies(funcDecl *ast.FuncDecl, fileImports map[string]string, projectPrefix string) []string {
	if funcDecl.Body == nil {
		return nil
	}

	usedPackages := make(map[string]bool)

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		// Look for selector expressions like "pkg.Function()"
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Check if the X part is an identifier (package name)
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}

		// Check if this identifier is a package (exists in imports)
		if importPath, exists := fileImports[ident.Name]; exists {
			usedPackages[importPath] = true
		}

		return true
	})

	// Convert map to slice
	var deps []string
	for pkg := range usedPackages {
		deps = append(deps, pkg)
	}

	return deps
}

// CategorizeDependencies categorizes dependencies into internal and external
func CategorizeDependencies(deps []string, projectPrefix string) (internal []string, external []string) {
	for _, dep := range deps {
		if strings.HasPrefix(dep, projectPrefix) {
			internal = append(internal, dep)
		} else {
			external = append(external, dep)
		}
	}
	return
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
