package reporter

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hiroki-yamauchi/go-code-health-analyzer/analyzer"
)

// GenerateJSONReport generates a JSON report from the analysis results
func GenerateJSONReport(report *analyzer.Report, outputPath string) error {
	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Create JSON encoder with indentation for readability
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	// Encode report to JSON
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
