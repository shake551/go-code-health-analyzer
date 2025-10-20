package analyzer

import (
	"go/ast"
	"strings"
)

// PackageDependency holds dependency information for packages
type PackageDependency struct {
	PkgPath      string
	Imports      []string // Packages this package imports
	ImportedBy   []string // Packages that import this package
}

// CalculateCoupling calculates coupling metrics for packages
func CalculateCoupling(pkgDeps map[string]*PackageDependency, projectPrefix string) map[string]CouplingMetrics {
	metrics := make(map[string]CouplingMetrics)

	for pkgPath, dep := range pkgDeps {
		// Count internal (project) dependencies only
		ca := 0 // Afferent coupling
		ce := 0 // Efferent coupling

		// Count packages that depend on this package (Ca)
		for _, importingPkg := range dep.ImportedBy {
			if strings.HasPrefix(importingPkg, projectPrefix) {
				ca++
			}
		}

		// Count packages this package depends on (Ce)
		for _, importedPkg := range dep.Imports {
			if strings.HasPrefix(importedPkg, projectPrefix) {
				ce++
			}
		}

		// Calculate instability: I = Ce / (Ca + Ce)
		instability := 0.0
		if ca+ce > 0 {
			instability = float64(ce) / float64(ca+ce)
		}

		metrics[pkgPath] = CouplingMetrics{
			Afferent:    ca,
			Efferent:    ce,
			Instability: instability,
		}
	}

	return metrics
}

// CouplingMetrics holds coupling metrics for a package
type CouplingMetrics struct {
	Afferent    int
	Efferent    int
	Instability float64
}

// ExtractImports extracts all import statements from a package
func ExtractImports(pkg *ast.Package) []string {
	importsMap := make(map[string]bool)

	for _, file := range pkg.Files {
		for _, imp := range file.Imports {
			// Remove quotes from import path
			path := strings.Trim(imp.Path.Value, `"`)
			importsMap[path] = true
		}
	}

	// Convert map to slice
	var imports []string
	for imp := range importsMap {
		imports = append(imports, imp)
	}

	return imports
}
