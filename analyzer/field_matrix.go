package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"sort"
)

// AnalyzeFieldMatrix analyzes method×field usage patterns using matrix analysis and PCA
func AnalyzeFieldMatrix(structName string, structType *ast.StructType, file *ast.File, fset *token.FileSet, fields []string) *FieldMatrixAnalysis {
	// Skip analysis if too few fields (PCA unstable)
	if len(fields) < 3 {
		return nil
	}

	// Extract methods and their field usage
	methods := extractMethodsWithFieldsWeighted(structName, file, fields)

	if len(methods) == 0 {
		return nil
	}

	// Filter out getter/setter methods
	filteredMethods := make([]methodFieldUsageWeighted, 0)
	for _, m := range methods {
		if !isUtilityMethod(m.methodName) {
			filteredMethods = append(filteredMethods, m)
		}
	}

	if len(filteredMethods) < 2 {
		// Not enough data for meaningful analysis
		return nil
	}

	// Build weighted usage matrix
	matrix, methodNames := buildWeightedUsageMatrix(filteredMethods, fields)

	if len(matrix) < 2 || len(matrix[0]) < 3 {
		// Not enough data for meaningful analysis
		return nil
	}

	// Perform PCA to estimate number of clusters
	estimatedClusters, explainedVariance := estimateClustersViaPCA(matrix)

	// Generate recommendations
	recommendations := generateRecommendations(estimatedClusters, len(methodNames), len(fields), explainedVariance)

	return &FieldMatrixAnalysis{
		Matrix:                      matrix,
		MethodNames:                 methodNames,
		FieldNames:                  fields,
		EstimatedClusters:           estimatedClusters,
		ExplainedVariance:           explainedVariance,
		HasMultipleResponsibilities: estimatedClusters >= 2,
		Recommendations:             recommendations,
	}
}

// methodFieldUsage tracks which fields a method uses (deprecated, use methodFieldUsageWeighted)
type methodFieldUsage struct {
	methodName string
	usedFields map[string]bool
}

// methodFieldUsageWeighted tracks which fields a method uses with weights
// Weight: 0 = not used, 1 = read, 2 = write, 3 = read+write
type methodFieldUsageWeighted struct {
	methodName string
	fieldUsage map[string]int // field name -> usage weight
}

// extractMethodsWithFields extracts methods and their field usage
func extractMethodsWithFields(structName string, file *ast.File, structFields []string) []methodFieldUsage {
	var methods []methodFieldUsage

	// Create field map for quick lookup
	fieldMap := make(map[string]bool)
	for _, field := range structFields {
		fieldMap[field] = true
	}

	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if this is a method of our struct
		if funcDecl.Recv == nil {
			return true
		}

		for _, recv := range funcDecl.Recv.List {
			var recvTypeName string
			var recvName string

			// Get receiver type name
			switch t := recv.Type.(type) {
			case *ast.Ident:
				recvTypeName = t.Name
			case *ast.StarExpr:
				if ident, ok := t.X.(*ast.Ident); ok {
					recvTypeName = ident.Name
				}
			}

			// Get receiver variable name
			if len(recv.Names) > 0 {
				recvName = recv.Names[0].Name
			}

			if recvTypeName == structName {
				// This is a method of our struct
				usedFields := findUsedFields(funcDecl.Body, recvName, fieldMap)
				methods = append(methods, methodFieldUsage{
					methodName: funcDecl.Name.Name,
					usedFields: usedFields,
				})
			}
		}

		return true
	})

	return methods
}

// extractMethodsWithFieldsWeighted extracts methods and their weighted field usage
func extractMethodsWithFieldsWeighted(structName string, file *ast.File, structFields []string) []methodFieldUsageWeighted {
	var methods []methodFieldUsageWeighted

	// Create field map for quick lookup
	fieldMap := make(map[string]bool)
	for _, field := range structFields {
		fieldMap[field] = true
	}

	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if this is a method of our struct
		if funcDecl.Recv == nil {
			return true
		}

		for _, recv := range funcDecl.Recv.List {
			var recvTypeName string
			var recvName string

			// Get receiver type name
			switch t := recv.Type.(type) {
			case *ast.Ident:
				recvTypeName = t.Name
			case *ast.StarExpr:
				if ident, ok := t.X.(*ast.Ident); ok {
					recvTypeName = ident.Name
				}
			}

			// Get receiver variable name
			if len(recv.Names) > 0 {
				recvName = recv.Names[0].Name
			}

			if recvTypeName == structName {
				// This is a method of our struct
				fieldUsage := findFieldUsageWeighted(funcDecl.Body, recvName, fieldMap)
				methods = append(methods, methodFieldUsageWeighted{
					methodName: funcDecl.Name.Name,
					fieldUsage: fieldUsage,
				})
			}
		}

		return true
	})

	return methods
}

// findFieldUsageWeighted finds field usage with weights (read=1, write=2, both=3)
func findFieldUsageWeighted(body *ast.BlockStmt, recvName string, fieldMap map[string]bool) map[string]int {
	fieldUsage := make(map[string]int)

	if body == nil {
		return fieldUsage
	}

	ast.Inspect(body, func(n ast.Node) bool {
		// Check for assignments (write)
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				if selector, ok := lhs.(*ast.SelectorExpr); ok {
					if ident, ok := selector.X.(*ast.Ident); ok {
						if ident.Name == recvName && fieldMap[selector.Sel.Name] {
							// Write operation
							if fieldUsage[selector.Sel.Name]&1 == 1 {
								// Already has read, now has both
								fieldUsage[selector.Sel.Name] = 3
							} else {
								// Write only
								fieldUsage[selector.Sel.Name] = 2
							}
						}
					}
				}
			}
			// Also check RHS for reads
			for _, rhs := range assign.Rhs {
				ast.Inspect(rhs, func(n2 ast.Node) bool {
					if selector, ok := n2.(*ast.SelectorExpr); ok {
						if ident, ok := selector.X.(*ast.Ident); ok {
							if ident.Name == recvName && fieldMap[selector.Sel.Name] {
								// Read operation
								if fieldUsage[selector.Sel.Name] == 2 {
									// Already has write, now has both
									fieldUsage[selector.Sel.Name] = 3
								} else if fieldUsage[selector.Sel.Name] == 0 {
									// Read only
									fieldUsage[selector.Sel.Name] = 1
								}
							}
						}
					}
					return true
				})
			}
			return false // Don't traverse children again
		}

		// Check for reads (selector expressions not in assignments)
		if selector, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := selector.X.(*ast.Ident); ok {
				if ident.Name == recvName && fieldMap[selector.Sel.Name] {
					// Read operation (if not already marked as write or both)
					if fieldUsage[selector.Sel.Name] == 0 {
						fieldUsage[selector.Sel.Name] = 1
					} else if fieldUsage[selector.Sel.Name] == 2 {
						// Already has write, now has both
						fieldUsage[selector.Sel.Name] = 3
					}
				}
			}
		}

		return true
	})

	return fieldUsage
}

// buildWeightedUsageMatrix builds a weighted matrix: matrix[i][j] = weight of method i using field j
func buildWeightedUsageMatrix(methods []methodFieldUsageWeighted, fields []string) ([][]int, []string) {
	matrix := make([][]int, len(methods))
	methodNames := make([]string, len(methods))

	for i, method := range methods {
		methodNames[i] = method.methodName
		matrix[i] = make([]int, len(fields))

		for j, field := range fields {
			matrix[i][j] = method.fieldUsage[field] // 0, 1, 2, or 3
		}
	}

	return matrix, methodNames
}

// buildUsageMatrix builds a binary matrix: matrix[i][j] = 1 if method i uses field j
func buildUsageMatrix(methods []methodFieldUsage, fields []string) ([][]int, []string) {
	matrix := make([][]int, len(methods))
	methodNames := make([]string, len(methods))

	for i, method := range methods {
		methodNames[i] = method.methodName
		matrix[i] = make([]int, len(fields))

		for j, field := range fields {
			if method.usedFields[field] {
				matrix[i][j] = 1
			} else {
				matrix[i][j] = 0
			}
		}
	}

	return matrix, methodNames
}

// estimateClustersViaPCA estimates the number of responsibility clusters using PCA
func estimateClustersViaPCA(matrix [][]int) (int, []float64) {
	// Convert int matrix to float64 for calculations
	floatMatrix := make([][]float64, len(matrix))
	for i := range matrix {
		floatMatrix[i] = make([]float64, len(matrix[i]))
		for j := range matrix[i] {
			floatMatrix[i][j] = float64(matrix[i][j])
		}
	}

	// Center the data (subtract mean)
	centeredMatrix := centerMatrix(floatMatrix)

	// Compute covariance matrix
	covMatrix := computeCovarianceMatrix(centeredMatrix)

	// Compute eigenvalues (simplified approach using power iteration)
	eigenvalues := computeTopEigenvalues(covMatrix, 5)

	// Calculate explained variance ratios
	totalVariance := 0.0
	for _, ev := range eigenvalues {
		if ev > 0 {
			totalVariance += ev
		}
	}

	explainedVariance := make([]float64, len(eigenvalues))
	for i, ev := range eigenvalues {
		if totalVariance > 0 {
			explainedVariance[i] = ev / totalVariance
		}
	}

	// Estimate number of clusters using Kaiser criterion (eigenvalue > 1)
	// Or using elbow method (significant drop in explained variance)
	clusters := estimateClusterCount(eigenvalues, explainedVariance)

	return clusters, explainedVariance
}

// centerMatrix subtracts the mean from each column
func centerMatrix(matrix [][]float64) [][]float64 {
	if len(matrix) == 0 {
		return matrix
	}

	rows := len(matrix)
	cols := len(matrix[0])

	// Calculate column means
	means := make([]float64, cols)
	for j := 0; j < cols; j++ {
		sum := 0.0
		for i := 0; i < rows; i++ {
			sum += matrix[i][j]
		}
		means[j] = sum / float64(rows)
	}

	// Center the matrix
	centered := make([][]float64, rows)
	for i := 0; i < rows; i++ {
		centered[i] = make([]float64, cols)
		for j := 0; j < cols; j++ {
			centered[i][j] = matrix[i][j] - means[j]
		}
	}

	return centered
}

// computeCovarianceMatrix computes the covariance matrix
func computeCovarianceMatrix(matrix [][]float64) [][]float64 {
	if len(matrix) == 0 {
		return nil
	}

	rows := len(matrix)
	cols := len(matrix[0])

	// Covariance matrix is cols x cols
	cov := make([][]float64, cols)
	for i := range cov {
		cov[i] = make([]float64, cols)
	}

	// Compute covariance between each pair of columns
	for i := 0; i < cols; i++ {
		for j := i; j < cols; j++ {
			sum := 0.0
			for k := 0; k < rows; k++ {
				sum += matrix[k][i] * matrix[k][j]
			}
			cov[i][j] = sum / float64(rows-1)
			cov[j][i] = cov[i][j] // Symmetric
		}
	}

	return cov
}

// computeTopEigenvalues computes the top k eigenvalues using power iteration
func computeTopEigenvalues(matrix [][]float64, k int) []float64 {
	if len(matrix) == 0 {
		return nil
	}

	n := len(matrix)
	if k > n {
		k = n
	}

	eigenvalues := make([]float64, 0, k)
	workMatrix := copyMatrix(matrix)

	for iter := 0; iter < k; iter++ {
		// Use power iteration to find dominant eigenvalue
		eigenvalue := powerIteration(workMatrix, 100)

		if eigenvalue <= 1e-10 {
			break // No more significant eigenvalues
		}

		eigenvalues = append(eigenvalues, eigenvalue)

		// Deflate matrix (remove the found eigenvalue's contribution)
		// This is a simplified version; in practice, we'd use the eigenvector
		deflateMatrix(workMatrix, eigenvalue)
	}

	return eigenvalues
}

// powerIteration finds the dominant eigenvalue using power iteration
func powerIteration(matrix [][]float64, maxIter int) float64 {
	if len(matrix) == 0 {
		return 0
	}

	n := len(matrix)

	// Initialize with random vector
	v := make([]float64, n)
	for i := range v {
		v[i] = 1.0 / math.Sqrt(float64(n))
	}

	var eigenvalue float64

	for iter := 0; iter < maxIter; iter++ {
		// Multiply matrix by vector
		newV := make([]float64, n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				newV[i] += matrix[i][j] * v[j]
			}
		}

		// Calculate eigenvalue (Rayleigh quotient)
		numerator := 0.0
		denominator := 0.0
		for i := 0; i < n; i++ {
			numerator += newV[i] * v[i]
			denominator += v[i] * v[i]
		}

		if denominator > 0 {
			eigenvalue = numerator / denominator
		}

		// Normalize
		norm := 0.0
		for i := 0; i < n; i++ {
			norm += newV[i] * newV[i]
		}
		norm = math.Sqrt(norm)

		if norm < 1e-10 {
			break
		}

		for i := 0; i < n; i++ {
			v[i] = newV[i] / norm
		}
	}

	return math.Abs(eigenvalue)
}

// deflateMatrix removes the contribution of an eigenvalue (simplified)
func deflateMatrix(matrix [][]float64, eigenvalue float64) {
	n := len(matrix)
	for i := 0; i < n; i++ {
		matrix[i][i] -= eigenvalue * 0.5 // Simplified deflation
	}
}

// copyMatrix creates a deep copy of a matrix
func copyMatrix(matrix [][]float64) [][]float64 {
	copy := make([][]float64, len(matrix))
	for i := range matrix {
		copy[i] = make([]float64, len(matrix[i]))
		for j := range matrix[i] {
			copy[i][j] = matrix[i][j]
		}
	}
	return copy
}

// estimateClusterCount estimates the number of clusters from eigenvalues
func estimateClusterCount(eigenvalues []float64, explainedVariance []float64) int {
	if len(eigenvalues) == 0 {
		return 1
	}

	// Method 1: Count eigenvalues > 1 (Kaiser criterion)
	kaiserCount := 0
	for _, ev := range eigenvalues {
		if ev > 1.0 {
			kaiserCount++
		}
	}

	// Method 2: Elbow method - look for significant drop in explained variance
	elbowCount := 1
	for i := 0; i < len(explainedVariance)-1; i++ {
		// If explained variance is still > 10%, count it
		if explainedVariance[i] > 0.1 {
			elbowCount = i + 1
		} else {
			break
		}
	}

	// Method 3: Cumulative variance threshold (e.g., 80%)
	cumulativeVariance := 0.0
	varianceCount := 0
	for i, ratio := range explainedVariance {
		cumulativeVariance += ratio
		varianceCount = i + 1
		if cumulativeVariance >= 0.8 {
			break
		}
	}

	// Use the maximum of these methods, capped at reasonable number
	estimate := kaiserCount
	if elbowCount > estimate {
		estimate = elbowCount
	}
	if varianceCount < estimate {
		estimate = varianceCount
	}

	// Ensure at least 1, at most 5 for practical purposes
	if estimate < 1 {
		estimate = 1
	}
	if estimate > 5 {
		estimate = 5
	}

	return estimate
}

// generateRecommendations generates human-readable recommendations
func generateRecommendations(clusters int, numMethods int, numFields int, explainedVariance []float64) string {
	if clusters == 1 {
		return fmt.Sprintf(
			"Analysis suggests a single cohesive responsibility. "+
				"The %d methods work together on %d fields in a unified way. "+
				"This is a good sign of high cohesion.",
			numMethods, numFields,
		)
	}

	// Calculate primary cluster strength
	primaryStrength := "moderate"
	if len(explainedVariance) > 0 && explainedVariance[0] > 0.5 {
		primaryStrength = "strong"
	} else if len(explainedVariance) > 0 && explainedVariance[0] < 0.3 {
		primaryStrength = "weak"
	}

	// Sort explained variance for display
	topVariances := make([]float64, 0)
	for i := 0; i < clusters && i < len(explainedVariance); i++ {
		topVariances = append(topVariances, explainedVariance[i])
	}
	sort.Float64s(topVariances)
	// Reverse to get descending order
	for i, j := 0, len(topVariances)-1; i < j; i, j = i+1, j-1 {
		topVariances[i], topVariances[j] = topVariances[j], topVariances[i]
	}

	varianceStr := ""
	for i, v := range topVariances {
		if i > 0 {
			varianceStr += ", "
		}
		varianceStr += fmt.Sprintf("%.1f%%", v*100)
	}

	return fmt.Sprintf(
		"Analysis detects %d distinct responsibility clusters (variance explained: %s). "+
			"The primary cluster shows %s separation. "+
			"Consider splitting this struct into %d smaller, focused structs, "+
			"each handling one specific responsibility. "+
			"Group methods and fields based on which cluster they belong to.",
		clusters, varianceStr, primaryStrength, clusters,
	)
}
