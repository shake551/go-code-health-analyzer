package analyzer

import "fmt"

// PerformDiagnostics performs integrated analysis to detect anti-patterns and code smells
func PerformDiagnostics(packages []PackageResult) []DiagnosticResult {
	var diagnostics []DiagnosticResult

	// Detect God Objects
	diagnostics = append(diagnostics, detectGodObjects(packages)...)

	// Detect Unstable Foundations
	diagnostics = append(diagnostics, detectUnstableFoundations(packages)...)

	// Detect Overly Complex Functions
	diagnostics = append(diagnostics, detectComplexFunctions(packages)...)

	// Detect Ambiguous Structs
	diagnostics = append(diagnostics, detectAmbiguousStructs(packages)...)

	// Detect Split Responsibilities via Method Islands
	diagnostics = append(diagnostics, detectMethodIslands(packages)...)

	// Detect Split Responsibilities via Field Clustering
	diagnostics = append(diagnostics, detectFieldClusters(packages)...)

	return diagnostics
}

// detectGodObjects detects structs with excessive responsibilities
// Criteria: LCOM4 >= 5 AND package Ca >= 10
func detectGodObjects(packages []PackageResult) []DiagnosticResult {
	var results []DiagnosticResult

	for _, pkg := range packages {
		// Only consider packages with high afferent coupling
		if pkg.Afferent < 10 {
			continue
		}

		for _, s := range pkg.Structs {
			if s.LCOM4Score >= 5 {
				results = append(results, DiagnosticResult{
					Type:       "God Object",
					TargetName: fmt.Sprintf("%s.%s", pkg.Name, s.StructName),
					Message: fmt.Sprintf(
						"Struct '%s' has excessive responsibilities (LCOM4=%d) and is heavily depended upon (Ca=%d). Consider splitting into smaller, focused structs.",
						s.StructName, s.LCOM4Score, pkg.Afferent,
					),
					Severity: "Critical",
					Evidence: map[string]interface{}{
						"lcom4_score": s.LCOM4Score,
						"afferent":    pkg.Afferent,
						"package":     pkg.Name,
						"file_path":   s.FilePath,
					},
					RelatedPath: fmt.Sprintf("#struct-%s-%s", pkg.Path, s.StructName),
				})
			}
		}
	}

	return results
}

// detectUnstableFoundations detects packages that are heavily depended upon but unstable
// Criteria: Ca >= 10 AND Instability >= 0.7
func detectUnstableFoundations(packages []PackageResult) []DiagnosticResult {
	var results []DiagnosticResult

	for _, pkg := range packages {
		if pkg.Afferent >= 10 && pkg.Instability >= 0.7 {
			results = append(results, DiagnosticResult{
				Type:       "Unstable Foundation",
				TargetName: pkg.Name,
				Message: fmt.Sprintf(
					"Package '%s' is heavily depended upon (Ca=%d) but highly unstable (I=%.2f). This creates a fragile foundation. Consider stabilizing this package by reducing dependencies.",
					pkg.Name, pkg.Afferent, pkg.Instability,
				),
				Severity: "Critical",
				Evidence: map[string]interface{}{
					"afferent":    pkg.Afferent,
					"efferent":    pkg.Efferent,
					"instability": pkg.Instability,
					"package":     pkg.Name,
				},
				RelatedPath: fmt.Sprintf("#package-%s", pkg.Path),
			})
		}
	}

	return results
}

// detectComplexFunctions detects functions with excessive cyclomatic complexity
// Criteria: Complexity >= 15
func detectComplexFunctions(packages []PackageResult) []DiagnosticResult {
	var results []DiagnosticResult

	for _, pkg := range packages {
		for _, f := range pkg.Functions {
			if f.Complexity >= 15 {
				results = append(results, DiagnosticResult{
					Type:       "Overly Complex Function",
					TargetName: fmt.Sprintf("%s.%s", pkg.Name, f.FuncName),
					Message: fmt.Sprintf(
						"Function '%s' is too complex (Complexity=%d). High complexity makes code hard to test and maintain. Consider refactoring into smaller functions.",
						f.FuncName, f.Complexity,
					),
					Severity: "Warning",
					Evidence: map[string]interface{}{
						"complexity": f.Complexity,
						"function":   f.FuncName,
						"package":    pkg.Name,
						"file_path":  f.FilePath,
					},
					RelatedPath: fmt.Sprintf("#function-%s-%s", pkg.Path, f.FuncName),
				})
			}
		}
	}

	return results
}

// detectAmbiguousStructs detects structs with low cohesion and complex methods
// Criteria: LCOM4 >= 3 AND at least one method with Complexity >= 10
func detectAmbiguousStructs(packages []PackageResult) []DiagnosticResult {
	var results []DiagnosticResult

	for _, pkg := range packages {
		// Build map of method complexity for this package
		methodComplexity := make(map[string]int)
		for _, f := range pkg.Functions {
			methodComplexity[f.FuncName] = f.Complexity
		}

		for _, s := range pkg.Structs {
			if s.LCOM4Score < 3 {
				continue
			}

			// Check if any method of this struct has high complexity
			hasComplexMethod := false
			var complexMethods []string

			// Extract struct name prefix to identify methods
			structPrefix := s.StructName + "."
			for funcName, complexity := range methodComplexity {
				// Check if function name starts with struct name (method naming)
				if len(funcName) > len(structPrefix) && funcName[:len(structPrefix)] == structPrefix {
					if complexity >= 10 {
						hasComplexMethod = true
						complexMethods = append(complexMethods, funcName)
					}
				}
			}

			if hasComplexMethod {
				results = append(results, DiagnosticResult{
					Type:       "Ambiguous Struct",
					TargetName: fmt.Sprintf("%s.%s", pkg.Name, s.StructName),
					Message: fmt.Sprintf(
						"Struct '%s' has unclear responsibilities (LCOM4=%d) and contains complex logic. This suggests mixed concerns. Consider refactoring.",
						s.StructName, s.LCOM4Score,
					),
					Severity: "Warning",
					Evidence: map[string]interface{}{
						"lcom4_score":      s.LCOM4Score,
						"complex_methods":  complexMethods,
						"package":          pkg.Name,
						"file_path":        s.FilePath,
					},
					RelatedPath: fmt.Sprintf("#struct-%s-%s", pkg.Path, s.StructName),
				})
			}
		}
	}

	return results
}

// detectMethodIslands detects structs with multiple isolated private method clusters
// Criteria: MethodClusters.HasMultipleIslands == true (>= 2 clusters)
func detectMethodIslands(packages []PackageResult) []DiagnosticResult {
	var results []DiagnosticResult

	for _, pkg := range packages {
		for _, s := range pkg.Structs {
			if s.MethodClusters == nil || !s.MethodClusters.HasMultipleIslands {
				continue
			}

			mc := s.MethodClusters

			// Build cluster summary
			clusterSummary := ""
			for i, cluster := range mc.Clusters {
				if i > 0 {
					clusterSummary += "; "
				}
				clusterSummary += fmt.Sprintf("Cluster %d (%d methods): %s",
					cluster.ID, cluster.Size, cluster.ResponsibilityHint)
			}

			results = append(results, DiagnosticResult{
				Type:       "Split Responsibility (Method Islands)",
				TargetName: fmt.Sprintf("%s.%s", pkg.Name, s.StructName),
				Message: fmt.Sprintf(
					"Struct '%s' has %d isolated groups of private methods, suggesting %d distinct responsibilities. "+
						"Private methods that don't call each other likely serve different purposes. "+
						"Clusters: %s. Consider splitting into separate structs.",
					s.StructName, mc.ClusterCount, mc.ClusterCount, clusterSummary,
				),
				Severity: "Warning",
				Evidence: map[string]interface{}{
					"cluster_count":         mc.ClusterCount,
					"total_private_methods": mc.TotalPrivateMethods,
					"clusters":              mc.Clusters,
					"package":               pkg.Name,
					"file_path":             s.FilePath,
				},
				RelatedPath: fmt.Sprintf("#struct-%s-%s", pkg.Path, s.StructName),
			})
		}
	}

	return results
}

// detectFieldClusters detects structs with multiple responsibility clusters via PCA
// Criteria: FieldMatrix.HasMultipleResponsibilities == true (estimated clusters >= 2)
func detectFieldClusters(packages []PackageResult) []DiagnosticResult {
	var results []DiagnosticResult

	for _, pkg := range packages {
		for _, s := range pkg.Structs {
			if s.FieldMatrix == nil || !s.FieldMatrix.HasMultipleResponsibilities {
				continue
			}

			fm := s.FieldMatrix

			// Determine severity based on number of clusters and variance
			severity := "Warning"
			if fm.EstimatedClusters >= 3 {
				severity = "Critical"
			}

			results = append(results, DiagnosticResult{
				Type:       "Split Responsibility (Field Clusters)",
				TargetName: fmt.Sprintf("%s.%s", pkg.Name, s.StructName),
				Message: fmt.Sprintf(
					"Struct '%s' shows %d distinct responsibility patterns in method-field usage (PCA analysis). "+
						"%s",
					s.StructName, fm.EstimatedClusters, fm.Recommendations,
				),
				Severity: severity,
				Evidence: map[string]interface{}{
					"estimated_clusters": fm.EstimatedClusters,
					"explained_variance": fm.ExplainedVariance,
					"method_count":       len(fm.MethodNames),
					"field_count":        len(fm.FieldNames),
					"package":            pkg.Name,
					"file_path":          s.FilePath,
					"recommendations":    fm.Recommendations,
				},
				RelatedPath: fmt.Sprintf("#struct-%s-%s", pkg.Path, s.StructName),
			})
		}
	}

	return results
}
