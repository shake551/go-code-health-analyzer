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

// CalculateDependencyDepth calculates the maximum depth of the internal dependency chain for each package
func CalculateDependencyDepth(pkgDeps map[string]*PackageDependency, projectPrefix string) map[string]int {
	depths := make(map[string]int)
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)

	// Create mapping from full import path to relative path
	fullToRelPath := make(map[string]string)
	for pkgPath := range pkgDeps {
		fullPath := projectPrefix
		if pkgPath != "" {
			fullPath = projectPrefix + "/" + pkgPath
		}
		fullToRelPath[fullPath] = pkgPath
	}

	// DFS to calculate depth for each package
	var dfs func(pkgPath string) int
	dfs = func(pkgPath string) int {
		// If already calculated, return cached result
		if visited[pkgPath] {
			return depths[pkgPath]
		}

		// Detect circular dependencies
		if inProgress[pkgPath] {
			return 0
		}

		inProgress[pkgPath] = true
		maxDepth := 0

		// Get dependencies for this package
		dep := pkgDeps[pkgPath]
		if dep != nil {
			// Only consider internal dependencies (within the project)
			for _, importPath := range dep.Imports {
				if strings.HasPrefix(importPath, projectPrefix) {
					// Convert full import path to relative path
					if relPath, exists := fullToRelPath[importPath]; exists {
						childDepth := dfs(relPath)
						if childDepth > maxDepth {
							maxDepth = childDepth
						}
					}
				}
			}
		}

		// Depth is 1 + max depth of dependencies
		if maxDepth > 0 || (dep != nil && len(dep.Imports) > 0) {
			depths[pkgPath] = maxDepth + 1
		} else {
			depths[pkgPath] = 0 // No internal dependencies
		}

		visited[pkgPath] = true
		inProgress[pkgPath] = false

		return depths[pkgPath]
	}

	// Calculate depth for each package
	for pkgPath := range pkgDeps {
		if !visited[pkgPath] {
			dfs(pkgPath)
		}
	}

	return depths
}
