package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiroki-yamauchi/go-code-health-analyzer/analyzer"
	"github.com/hiroki-yamauchi/go-code-health-analyzer/reporter"
)

func main() {
	// Define command line flags
	formatFlag := flag.String("format", "html", "Output format: html, json, or both")
	outputFlag := flag.String("output", "", "Output file path (default: code_health_report.html or code_health_report.json)")
	excludeFlag := flag.String("exclude", "", "Comma-separated list of directory names to exclude (e.g., vendor,node_modules,tmp)")
	flag.Usage = printUsage
	flag.Parse()

	// Get target path from positional argument
	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	targetPath := args[0]

	// Check if target path exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Target path does not exist: %s\n", targetPath)
		os.Exit(1)
	}

	// Parse exclude patterns
	var excludeDirs []string
	if *excludeFlag != "" {
		excludeDirs = strings.Split(*excludeFlag, ",")
		// Trim whitespace from each pattern
		for i := range excludeDirs {
			excludeDirs[i] = strings.TrimSpace(excludeDirs[i])
		}
	}

	fmt.Printf("Analyzing Go project at: %s\n", targetPath)
	if len(excludeDirs) > 0 {
		fmt.Printf("Excluding directories: %s\n", strings.Join(excludeDirs, ", "))
	}

	// Perform analysis
	report, err := analyzer.Analyze(targetPath, excludeDirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during analysis: %v\n", err)
		os.Exit(1)
	}

	// Normalize format flag
	format := strings.ToLower(*formatFlag)

	// Generate reports based on format
	switch format {
	case "html":
		if err := generateHTML(report, *outputFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "json":
		if err := generateJSON(report, *outputFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "both":
		htmlOutput := *outputFlag
		if htmlOutput == "" {
			htmlOutput = "code_health_report.html"
		}
		jsonOutput := strings.TrimSuffix(htmlOutput, ".html") + ".json"

		if err := generateHTML(report, htmlOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating HTML: %v\n", err)
			os.Exit(1)
		}
		if err := generateJSON(report, jsonOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating JSON: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid format '%s'. Use 'html', 'json', or 'both'\n", format)
		os.Exit(1)
	}

	// Print summary
	printSummary(report)
}

func generateHTML(report *analyzer.Report, outputPath string) error {
	if outputPath == "" {
		outputPath = "code_health_report.html"
	}

	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("error resolving output path: %w", err)
	}

	fmt.Printf("Generating HTML report...\n")
	if err := reporter.GenerateHTMLReport(report, absOutputPath); err != nil {
		return fmt.Errorf("error generating HTML report: %w", err)
	}

	fmt.Printf("📊 HTML report saved to: %s\n", absOutputPath)
	return nil
}

func generateJSON(report *analyzer.Report, outputPath string) error {
	if outputPath == "" {
		outputPath = "code_health_report.json"
	}

	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("error resolving output path: %w", err)
	}

	fmt.Printf("Generating JSON report...\n")
	if err := reporter.GenerateJSONReport(report, absOutputPath); err != nil {
		return fmt.Errorf("error generating JSON report: %w", err)
	}

	fmt.Printf("📊 JSON report saved to: %s\n", absOutputPath)
	return nil
}

func printSummary(report *analyzer.Report) {
	fmt.Printf("\n✅ Analysis complete!\n")
	fmt.Printf("   Analyzed packages: %d\n", len(report.Packages))

	totalStructs := 0
	totalFunctions := 0
	for _, pkg := range report.Packages {
		totalStructs += len(pkg.Structs)
		totalFunctions += len(pkg.Functions)
	}

	fmt.Printf("   Analyzed structs: %d\n", totalStructs)
	fmt.Printf("   Analyzed functions: %d\n", totalFunctions)
	fmt.Println()
}

func printUsage() {
	fmt.Println("Go Code Health Analyzer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go-code-health-analyzer [options] <target-directory>")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -format string")
	fmt.Println("        Output format: html, json, or both (default: html)")
	fmt.Println("  -output string")
	fmt.Println("        Output file path (default: code_health_report.html or .json)")
	fmt.Println("  -exclude string")
	fmt.Println("        Comma-separated list of directory names to exclude")
	fmt.Println("        Default excludes: vendor, testdata (always excluded)")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  target-directory  Path to the Go project directory to analyze")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Generate HTML report (default)")
	fmt.Println("  go-code-health-analyzer ./myproject")
	fmt.Println()
	fmt.Println("  # Generate JSON report")
	fmt.Println("  go-code-health-analyzer -format json ./myproject")
	fmt.Println()
	fmt.Println("  # Generate both HTML and JSON reports")
	fmt.Println("  go-code-health-analyzer -format both ./myproject")
	fmt.Println()
	fmt.Println("  # Exclude specific directories")
	fmt.Println("  go-code-health-analyzer -exclude \"build,dist,tmp\" ./myproject")
	fmt.Println()
	fmt.Println("  # Combine multiple options")
	fmt.Println("  go-code-health-analyzer -format json -exclude \"node_modules,build\" -output report.json ./myproject")
}
