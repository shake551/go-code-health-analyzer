package analyzer

// Report represents the complete analysis report
type Report struct {
	Diagnostics []DiagnosticResult `json:"diagnostics"` // Integrated analysis results
	Packages    []PackageResult    `json:"packages"`
}

// DiagnosticResult represents an anti-pattern or code smell detected by integrated analysis
type DiagnosticResult struct {
	Type        string                 `json:"type"`         // "God Object", "Unstable Foundation", etc.
	TargetName  string                 `json:"target_name"`  // Name of the problematic package or struct
	Message     string                 `json:"message"`      // Human-readable description
	Severity    string                 `json:"severity"`     // "Critical", "Warning"
	Evidence    map[string]interface{} `json:"evidence"`     // Metric values that support this diagnosis
	RelatedPath string                 `json:"related_path"` // Link to detailed data (e.g., "#lcom-UserManager")
}

// PackageResult represents the analysis results for a single package
type PackageResult struct {
	Name        string           `json:"name"`        // Package name
	Path        string           `json:"path"`        // Package import path
	Afferent    int              `json:"afferent"`    // Ca: Number of packages that depend on this package
	Efferent    int              `json:"efferent"`    // Ce: Number of packages this package depends on
	Instability float64          `json:"instability"` // I: Ce / (Ca + Ce)
	Structs     []StructResult   `json:"structs"`     // Struct analysis results
	Functions   []FunctionResult `json:"functions"`   // Function analysis results
}

// StructResult represents the LCOM4 analysis results for a single struct
type StructResult struct {
	StructName       string     `json:"struct_name"`       // Name of the struct
	FilePath         string     `json:"file_path"`         // Source file path
	LCOM4Score       int        `json:"lcom4_score"`       // LCOM4 score (number of connected components)
	ComponentDetails [][]string `json:"component_details"` // Details of each connected component
}

// FunctionResult represents the cyclomatic complexity analysis results for a single function
type FunctionResult struct {
	FuncName   string `json:"function_name"` // Function/method name
	FilePath   string `json:"file_path"`     // Source file path
	Complexity int    `json:"complexity"`    // Cyclomatic complexity score
}
