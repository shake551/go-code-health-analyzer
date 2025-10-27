package analyzer

// Report represents the complete analysis report
type Report struct {
	Diagnostics []DiagnosticResult `json:"diagnostics"` // Integrated analysis results
	Packages    []PackageResult    `json:"packages"`
	TotalLoC    int                `json:"total_loc"` // Total lines of code in the project
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
	Name            string           `json:"name"`             // Package name
	Path            string           `json:"path"`             // Package import path
	Afferent        int              `json:"afferent"`         // Ca: Number of packages that depend on this package
	Efferent        int              `json:"efferent"`         // Ce: Number of packages this package depends on
	Instability     float64          `json:"instability"`      // I: Ce / (Ca + Ce)
	Structs         []StructResult   `json:"structs"`          // Struct analysis results
	Functions       []FunctionResult `json:"functions"`        // Function analysis results
	TotalLoC        int              `json:"total_loc"`        // Total lines of code in this package
	AvgFuncLoC      float64          `json:"avg_func_loc"`     // Average lines of code per function
	FuncCount       int              `json:"func_count"`       // Number of functions/methods in this package
	FileCount       int              `json:"file_count"`       // Number of files in this package
	DependencyDepth int              `json:"dependency_depth"` // Maximum depth of internal dependency chain
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
	FuncName         string   `json:"function_name"`      // Function/method name
	FilePath         string   `json:"file_path"`          // Source file path
	Complexity       int      `json:"complexity"`         // Cyclomatic complexity score
	LoC              int      `json:"loc"`                // Lines of code in this function
	Dependencies     []string `json:"dependencies"`       // List of external packages this function depends on
	InternalDeps     []string `json:"internal_deps"`      // List of internal (project) packages this function depends on
	ExternalDeps     []string `json:"external_deps"`      // List of external (3rd party) packages this function depends on
	DependencyCount  int      `json:"dependency_count"`   // Total number of package dependencies
	Afferent         int      `json:"afferent"`           // Ca: Number of functions that call this function (within project)
	Efferent         int      `json:"efferent"`           // Ce: Number of external functions/packages this function calls
	Instability      float64  `json:"instability"`        // I: Ce / (Ca + Ce)
}
