package analyzer

import (
	"go/ast"
	"go/token"
)

// CalculateLoCForPackage calculates lines of code metrics for an entire package
func CalculateLoCForPackage(pkg *ast.Package, fset *token.FileSet) PackageLoC {
	result := PackageLoC{
		TotalLoC:  0,
		FileCount: 0,
		FileLocs:  make(map[string]int),
	}

	for fileName, file := range pkg.Files {
		fileLoC := calculateFileLoC(file, fset)
		result.TotalLoC += fileLoC
		result.FileCount++
		result.FileLocs[fileName] = fileLoC
	}

	return result
}

// PackageLoC holds LoC metrics for a package
type PackageLoC struct {
	TotalLoC  int
	FileCount int
	FileLocs  map[string]int
}

// calculateFileLoC calculates the number of lines of code in a file
func calculateFileLoC(file *ast.File, fset *token.FileSet) int {
	if file == nil {
		return 0
	}

	// Get the file's position range
	startPos := fset.Position(file.Pos())
	endPos := fset.Position(file.End())

	// Calculate the number of lines
	// Note: This gives us the total number of lines in the file (including comments and blank lines)
	// For a more accurate "source lines of code" we could filter out comments and blank lines
	return endPos.Line - startPos.Line + 1
}

// CalculateFunctionLoC calculates lines of code for a function
func CalculateFunctionLoC(funcDecl *ast.FuncDecl, fset *token.FileSet) int {
	if funcDecl == nil || funcDecl.Body == nil {
		return 0
	}

	// Get the function body's position range
	startPos := fset.Position(funcDecl.Body.Lbrace)
	endPos := fset.Position(funcDecl.Body.Rbrace)

	// Calculate the number of lines in the function body
	// We subtract 1 to not count the opening brace line twice
	lines := endPos.Line - startPos.Line
	if lines < 0 {
		return 0
	}
	return lines
}

// CalculateLoCForFunctions calculates LoC for all functions in a package
// and returns them as a map keyed by function name
func CalculateLoCForFunctions(pkg *ast.Package, fset *token.FileSet) map[string]int {
	funcLoCs := make(map[string]int)

	for _, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

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

			loc := CalculateFunctionLoC(funcDecl, fset)
			funcLoCs[funcName] = loc

			return true
		})
	}

	return funcLoCs
}
