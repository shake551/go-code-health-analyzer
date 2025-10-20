package reporter

import (
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"sort"

	"github.com/hiroki-yamauchi/go-code-health-analyzer/analyzer"
)

//go:embed template.html
var htmlTemplate string

// GenerateHTMLReport generates an interactive HTML report from the analysis results
func GenerateHTMLReport(report *analyzer.Report, outputPath string) error {
	// Prepare template data
	data := prepareTemplateData(report)

	// Parse template
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"lcom4Class": func(score int) string {
			if score == 1 {
				return "green"
			} else if score == 2 {
				return "yellow"
			}
			return "red"
		},
		"complexityClass": func(complexity int) string {
			if complexity <= 10 {
				return "green"
			} else if complexity <= 15 {
				return "yellow"
			}
			return "red"
		},
		"instabilityClass": func(instability float64) string {
			if instability <= 0.3 {
				return "green"
			} else if instability <= 0.7 {
				return "yellow"
			}
			return "red"
		},
		"add": func(a, b int) int {
			return a + b
		},
	}).Parse(htmlTemplate)

	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// TemplateData holds the data for the HTML template
type TemplateData struct {
	Summary         Summary
	Diagnostics     []analyzer.DiagnosticResult
	PackageResults  []analyzer.PackageResult
	StructResults   []StructWithPackage
	FunctionResults []FunctionWithPackage
}

// Summary holds summary statistics
type Summary struct {
	TotalPackages        int
	TotalStructs         int
	TotalFunctions       int
	HighLCOM4Count       int // LCOM4 > 2
	HighComplexityCount  int // Complexity > 15
	HighInstabilityCount int // Instability > 0.7
	CriticalIssues       int // Critical diagnostics
	WarningIssues        int // Warning diagnostics
}

// StructWithPackage adds package information to struct results
type StructWithPackage struct {
	PackageName string
	PackagePath string
	analyzer.StructResult
}

// FunctionWithPackage adds package information to function results
type FunctionWithPackage struct {
	PackageName string
	PackagePath string
	analyzer.FunctionResult
}

// prepareTemplateData prepares data for the HTML template
func prepareTemplateData(report *analyzer.Report) TemplateData {
	var data TemplateData

	// Flatten structs and functions with package information
	var structs []StructWithPackage
	var functions []FunctionWithPackage

	for _, pkg := range report.Packages {
		for _, s := range pkg.Structs {
			structs = append(structs, StructWithPackage{
				PackageName:  pkg.Name,
				PackagePath:  pkg.Path,
				StructResult: s,
			})
		}

		for _, f := range pkg.Functions {
			functions = append(functions, FunctionWithPackage{
				PackageName:    pkg.Name,
				PackagePath:    pkg.Path,
				FunctionResult: f,
			})
		}
	}

	// Sort structs by LCOM4 score (descending)
	sort.Slice(structs, func(i, j int) bool {
		return structs[i].LCOM4Score > structs[j].LCOM4Score
	})

	// Sort functions by complexity (descending)
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Complexity > functions[j].Complexity
	})

	// Sort packages alphabetically by name
	packages := make([]analyzer.PackageResult, len(report.Packages))
	copy(packages, report.Packages)
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	// Calculate summary statistics
	summary := Summary{
		TotalPackages:  len(report.Packages),
		TotalStructs:   len(structs),
		TotalFunctions: len(functions),
	}

	for _, s := range structs {
		if s.LCOM4Score > 2 {
			summary.HighLCOM4Count++
		}
	}

	for _, f := range functions {
		if f.Complexity > 15 {
			summary.HighComplexityCount++
		}
	}

	for _, p := range report.Packages {
		if p.Instability > 0.7 {
			summary.HighInstabilityCount++
		}
	}

	// Count diagnostics by severity
	for _, d := range report.Diagnostics {
		if d.Severity == "Critical" {
			summary.CriticalIssues++
		} else if d.Severity == "Warning" {
			summary.WarningIssues++
		}
	}

	data.Summary = summary
	data.Diagnostics = report.Diagnostics
	data.PackageResults = packages
	data.StructResults = structs
	data.FunctionResults = functions

	return data
}
