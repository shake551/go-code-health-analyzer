package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Analyze performs comprehensive code analysis on the provided directory
func Analyze(targetPath string, excludeDirs []string) (*Report, error) {
	// Normalize the target path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Determine project module path (for coupling calculation)
	projectPrefix := determineProjectPrefix(absPath)

	// Parse all Go packages in the directory
	packages, err := parsePackages(absPath, excludeDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse packages: %w", err)
	}

	// Build package dependency graph
	pkgDeps := buildDependencyGraph(packages, projectPrefix)

	// Calculate coupling metrics
	couplingMetrics := CalculateCoupling(pkgDeps, projectPrefix)

	// Calculate dependency depth
	depthMetrics := CalculateDependencyDepth(pkgDeps, projectPrefix)

	// Generate report for each package
	var packageResults []PackageResult
	totalProjectLoC := 0

	for pkgPath, pkg := range packages {
		// Calculate LCOM4 for all structs
		structs := CalculateLCOM4(pkg.Package, pkg.FileSet)

		// Calculate cyclomatic complexity and LoC for all functions
		functions := CalculateComplexity(pkg.Package, pkg.FileSet, projectPrefix)

		// Calculate LoC for the package
		pkgLoC := CalculateLoCForPackage(pkg.Package, pkg.FileSet)
		totalProjectLoC += pkgLoC.TotalLoC

		// Calculate derived metrics
		funcCount := len(functions)
		avgFuncLoC := 0.0
		if funcCount > 0 {
			totalFuncLoC := 0
			for _, f := range functions {
				totalFuncLoC += f.LoC
			}
			avgFuncLoC = float64(totalFuncLoC) / float64(funcCount)
		}

		// Get coupling metrics
		coupling := couplingMetrics[pkgPath]

		// Get dependency depth
		depth := depthMetrics[pkgPath]

		packageResults = append(packageResults, PackageResult{
			Name:            pkg.Package.Name,
			Path:            pkgPath,
			Afferent:        coupling.Afferent,
			Efferent:        coupling.Efferent,
			Instability:     coupling.Instability,
			Structs:         structs,
			Functions:       functions,
			TotalLoC:        pkgLoC.TotalLoC,
			AvgFuncLoC:      avgFuncLoC,
			FuncCount:       funcCount,
			FileCount:       pkgLoC.FileCount,
			DependencyDepth: depth,
		})
	}

	// Perform integrated diagnostics
	diagnostics := PerformDiagnostics(packageResults)

	return &Report{
		Diagnostics: diagnostics,
		Packages:    packageResults,
		TotalLoC:    totalProjectLoC,
	}, nil
}

// ParsedPackage holds a parsed package and its file set
type ParsedPackage struct {
	Package *ast.Package
	FileSet *token.FileSet
}

// parsePackages parses all Go packages in the given directory
func parsePackages(rootPath string, excludeDirs []string) (map[string]*ParsedPackage, error) {
	packages := make(map[string]*ParsedPackage)

	// Default exclude patterns
	defaultExcludes := []string{"vendor", "testdata"}
	allExcludes := append(defaultExcludes, excludeDirs...)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-directories
		if !info.IsDir() {
			return nil
		}

		baseName := filepath.Base(path)

		// Skip hidden directories
		if strings.HasPrefix(baseName, ".") {
			return filepath.SkipDir
		}

		// Calculate relative path from root
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			relPath = baseName
		}
		// Normalize to use forward slashes for consistent matching
		relPath = filepath.ToSlash(relPath)

		// Skip excluded directories (check both basename and relative path)
		for _, exclude := range allExcludes {
			// Normalize exclude pattern to forward slashes
			normalizedExclude := filepath.ToSlash(exclude)

			// Match by basename (e.g., "vendor") or by relative path (e.g., "hoge/fuga")
			if baseName == normalizedExclude || relPath == normalizedExclude {
				return filepath.SkipDir
			}
		}

		// Try to parse Go files in this directory
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, path, func(fi os.FileInfo) bool {
			// Skip test files
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ParseComments)

		if err != nil {
			// Skip directories with parse errors
			return nil
		}

		// Store each package found
		for _, pkg := range pkgs {
			// Generate package path relative to root
			relPath, _ := filepath.Rel(rootPath, path)
			pkgPath := filepath.ToSlash(relPath)
			if pkgPath == "." {
				pkgPath = ""
			}

			packages[pkgPath] = &ParsedPackage{
				Package: pkg,
				FileSet: fset,
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return packages, nil
}

// buildDependencyGraph builds a dependency graph for all packages
func buildDependencyGraph(packages map[string]*ParsedPackage, projectPrefix string) map[string]*PackageDependency {
	deps := make(map[string]*PackageDependency)

	// Create mapping from full import path to relative path
	fullToRelPath := make(map[string]string)
	for pkgPath := range packages {
		fullPath := projectPrefix
		if pkgPath != "" {
			fullPath = projectPrefix + "/" + pkgPath
		}
		fullToRelPath[fullPath] = pkgPath
	}

	// Initialize dependency info for each package (using relative path as key)
	for pkgPath := range packages {
		fullPath := projectPrefix
		if pkgPath != "" {
			fullPath = projectPrefix + "/" + pkgPath
		}
		deps[pkgPath] = &PackageDependency{
			PkgPath:    fullPath,
			Imports:    []string{},
			ImportedBy: []string{},
		}
	}

	// Extract imports for each package
	for pkgPath, pkg := range packages {
		fullPath := projectPrefix
		if pkgPath != "" {
			fullPath = projectPrefix + "/" + pkgPath
		}

		imports := ExtractImports(pkg.Package)
		deps[pkgPath].Imports = imports

		// Update ImportedBy for imported packages
		for _, imp := range imports {
			// Convert import path to relative path
			if relPath, exists := fullToRelPath[imp]; exists {
				deps[relPath].ImportedBy = append(deps[relPath].ImportedBy, fullPath)
			}
		}
	}

	return deps
}

// determineProjectPrefix tries to determine the project's module path
func determineProjectPrefix(rootPath string) string {
	// Try to read go.mod file
	goModPath := filepath.Join(rootPath, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err == nil {
		// Parse module line
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module"))
			}
		}
	}

	// Fallback: use directory name
	return filepath.Base(rootPath)
}
