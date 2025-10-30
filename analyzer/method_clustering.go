package analyzer

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"unicode"
)

// AnalyzeMethodClustering analyzes private method call graph to detect responsibility islands
func AnalyzeMethodClustering(structName string, structType *ast.StructType, file *ast.File, fset *token.FileSet) *MethodClusterAnalysis {
	// Extract all methods of this struct
	methods := extractAllMethods(structName, file)

	if len(methods) == 0 {
		return nil
	}

	// Separate private and public methods
	privateMethods := make(map[string]*methodCallInfo)
	publicMethods := make(map[string]*methodCallInfo)

	for name, info := range methods {
		if isPrivateMethod(name) {
			privateMethods[name] = info
		} else {
			publicMethods[name] = info
		}
	}

	// If no private methods, no clustering analysis needed
	if len(privateMethods) == 0 {
		return nil
	}

	// Build call graph between private methods only
	callGraph := buildPrivateMethodCallGraph(privateMethods, methods)

	// Find clusters using Union-Find
	clusters := findMethodClusters(callGraph, privateMethods)

	// For each cluster, find which public methods call into it
	for i := range clusters {
		clusters[i].CalledBy = findPublicCallers(&clusters[i], publicMethods, methods)
		clusters[i].ResponsibilityHint = suggestResponsibility(clusters[i].Methods)
	}

	return &MethodClusterAnalysis{
		TotalPrivateMethods: len(privateMethods),
		ClusterCount:        len(clusters),
		Clusters:            clusters,
		HasMultipleIslands:  len(clusters) >= 2,
	}
}

// methodCallInfo holds information about a method and its calls
type methodCallInfo struct {
	name         string
	isPrivate    bool
	calls        map[string]int // Map of method names to call frequency
	calledBy     []string       // Names of methods that call this method
	receiverName string         // Receiver variable name (e.g., "s" in "func (s *Service)")
	isUtility    bool           // True if this is a utility/helper/test method
}

// extractAllMethods finds all methods of a struct with their call information
func extractAllMethods(structName string, file *ast.File) map[string]*methodCallInfo {
	methods := make(map[string]*methodCallInfo)

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
			var recvVarName string

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
				recvVarName = recv.Names[0].Name
			}

			if recvTypeName == structName {
				methodName := funcDecl.Name.Name
				fullName := structName + "." + methodName

				// Extract method calls with frequency
				calls := extractMethodCallsWithFrequency(funcDecl.Body, recvVarName, structName)

				// Check if this is a utility method
				isUtil := isUtilityMethod(methodName)

				methods[fullName] = &methodCallInfo{
					name:         fullName,
					isPrivate:    isPrivateMethod(methodName),
					calls:        calls,
					receiverName: recvVarName,
					isUtility:    isUtil,
				}
			}
		}

		return true
	})

	// Build reverse call graph (calledBy)
	for methodName, info := range methods {
		for calledMethod := range info.calls {
			if calledInfo, exists := methods[calledMethod]; exists {
				calledInfo.calledBy = append(calledInfo.calledBy, methodName)
			}
		}
	}

	return methods
}

// extractMethodCallsWithFrequency extracts all method calls with their frequency
func extractMethodCallsWithFrequency(body *ast.BlockStmt, recvName string, structName string) map[string]int {
	calls := make(map[string]int)

	if body == nil {
		return nil
	}

	ast.Inspect(body, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Look for method calls: receiver.method()
		if selector, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := selector.X.(*ast.Ident); ok {
				// Check if calling on the receiver
				if ident.Name == recvName {
					fullName := structName + "." + selector.Sel.Name
					calls[fullName]++ // Increment frequency
				}
			}
		}

		return true
	})

	return calls
}

// isPrivateMethod checks if a method name is private (starts with lowercase)
func isPrivateMethod(methodName string) bool {
	if len(methodName) == 0 {
		return false
	}
	// Extract just the method name if it contains "."
	parts := strings.Split(methodName, ".")
	name := parts[len(parts)-1]
	return unicode.IsLower(rune(name[0]))
}

// isUtilityMethod checks if a method is a utility/helper/test/getter/setter
func isUtilityMethod(methodName string) bool {
	lower := strings.ToLower(methodName)

	// Test/util/helper patterns
	utilityPatterns := []string{"test", "util", "helper", "mock", "stub"}
	for _, pattern := range utilityPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Getter/setter patterns (Get*, Set*, Is*, Has*)
	if strings.HasPrefix(methodName, "Get") && len(methodName) > 3 && unicode.IsUpper(rune(methodName[3])) {
		return true
	}
	if strings.HasPrefix(methodName, "Set") && len(methodName) > 3 && unicode.IsUpper(rune(methodName[3])) {
		return true
	}
	if strings.HasPrefix(methodName, "Is") && len(methodName) > 2 && unicode.IsUpper(rune(methodName[2])) {
		return true
	}
	if strings.HasPrefix(methodName, "Has") && len(methodName) > 3 && unicode.IsUpper(rune(methodName[3])) {
		return true
	}

	return false
}

// Configuration for clustering
const (
	WeightThreshold = 1   // Minimum call frequency to consider an edge (1 = at least one call)
	MinClusterSize  = 2   // Minimum number of nodes in a cluster to be considered significant
	MinClusterRatio = 0.2 // Minimum ratio of cluster size to total methods (ignore tiny clusters)
)

// buildPrivateMethodCallGraph builds a weighted call graph between private methods
// Returns a map of method -> list of (method, weight) pairs
func buildPrivateMethodCallGraph(privateMethods map[string]*methodCallInfo, allMethods map[string]*methodCallInfo) map[string]map[string]int {
	graph := make(map[string]map[string]int)

	for privateMethod := range privateMethods {
		graph[privateMethod] = make(map[string]int)

		// Find all private methods this method calls
		if info, exists := allMethods[privateMethod]; exists {
			// Skip utility methods
			if info.isUtility {
				continue
			}

			for calledMethod, frequency := range info.calls {
				// Only include edges to other non-utility private methods
				if calledInfo, isPrivate := privateMethods[calledMethod]; isPrivate && !calledInfo.isUtility {
					// Apply weight threshold
					if frequency >= WeightThreshold {
						graph[privateMethod][calledMethod] = frequency
					}
				}
			}
		}
	}

	return graph
}

// findMethodClusters finds connected components (clusters) in the weighted call graph
func findMethodClusters(callGraph map[string]map[string]int, privateMethods map[string]*methodCallInfo) []MethodCluster {
	uf := newUnionFind()

	// Add all non-utility private methods as nodes
	totalMethods := 0
	for method, info := range privateMethods {
		if !info.isUtility {
			uf.add(method)
			totalMethods++
		}
	}

	// Connect methods that call each other (undirected graph)
	for caller, callees := range callGraph {
		for callee := range callees {
			uf.union(caller, callee)
		}
	}

	// Get connected components
	components := uf.getComponents()

	// Filter out small clusters based on MinClusterSize and MinClusterRatio
	minSize := MinClusterSize
	if totalMethods > 0 {
		ratioBasedMin := int(float64(totalMethods) * MinClusterRatio)
		if ratioBasedMin < minSize {
			ratioBasedMin = minSize
		}
	}

	// Convert to MethodCluster format with filtering
	clusters := make([]MethodCluster, 0)
	for _, component := range components {
		// Filter: cluster must have at least MinClusterSize nodes
		// Unless it's a singleton and there's only one cluster total
		if len(component) >= minSize || len(components) == 1 {
			// Sort methods for consistent output
			sort.Strings(component)

			clusters = append(clusters, MethodCluster{
				ID:      len(clusters) + 1,
				Methods: component,
				Size:    len(component),
			})
		}
	}

	// Sort clusters by size (largest first)
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Size > clusters[j].Size
	})

	// Reassign IDs after sorting
	for i := range clusters {
		clusters[i].ID = i + 1
	}

	return clusters
}

// findPublicCallers finds which public methods call into a cluster
func findPublicCallers(cluster *MethodCluster, publicMethods map[string]*methodCallInfo, allMethods map[string]*methodCallInfo) []string {
	callers := make(map[string]bool)

	// For each method in the cluster
	for _, privateMethod := range cluster.Methods {
		if info, exists := allMethods[privateMethod]; exists {
			// Check who calls this private method
			for _, caller := range info.calledBy {
				// If caller is a public method, add it
				if _, isPublic := publicMethods[caller]; isPublic {
					callers[caller] = true
				}
			}
		}
	}

	result := make([]string, 0, len(callers))
	for caller := range callers {
		result = append(result, caller)
	}
	sort.Strings(result)
	return result
}

// suggestResponsibility suggests a responsibility name based on method names
func suggestResponsibility(methods []string) string {
	if len(methods) == 0 {
		return "Unknown"
	}

	// Extract common prefixes or keywords from method names
	keywords := make(map[string]int)

	for _, method := range methods {
		// Remove struct prefix
		parts := strings.Split(method, ".")
		if len(parts) < 2 {
			continue
		}
		methodName := parts[len(parts)-1]

		// Extract words from camelCase
		words := splitCamelCase(methodName)
		for _, word := range words {
			word = strings.ToLower(word)
			// Skip common words
			if word == "get" || word == "set" || word == "is" || word == "has" || word == "do" {
				continue
			}
			keywords[word]++
		}
	}

	// Find most common keyword
	maxCount := 0
	commonWord := ""
	for word, count := range keywords {
		if count > maxCount {
			maxCount = count
			commonWord = word
		}
	}

	if commonWord != "" {
		return strings.Title(commonWord) + "-related operations"
	}

	return "Mixed operations"
}

// splitCamelCase splits a camelCase string into words
func splitCamelCase(s string) []string {
	var words []string
	var currentWord []rune

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			if len(currentWord) > 0 {
				words = append(words, string(currentWord))
				currentWord = []rune{}
			}
		}
		currentWord = append(currentWord, r)
	}

	if len(currentWord) > 0 {
		words = append(words, string(currentWord))
	}

	return words
}
