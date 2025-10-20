package analyzer

import (
	"go/ast"
	"go/token"
)

// CalculateLCOM4 calculates the LCOM4 metric for all structs in the provided AST
func CalculateLCOM4(pkg *ast.Package, fset *token.FileSet) []StructResult {
	var results []StructResult

	// Traverse all files in the package
	for fileName, file := range pkg.Files {
		// Find all struct types
		ast.Inspect(file, func(n ast.Node) bool {
			typeSpec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			// Calculate LCOM4 for this struct
			result := calculateStructLCOM4(typeSpec.Name.Name, structType, file, fset, fileName)
			results = append(results, result)

			return true
		})
	}

	return results
}

// calculateStructLCOM4 calculates LCOM4 for a single struct
func calculateStructLCOM4(structName string, structType *ast.StructType, file *ast.File, fset *token.FileSet, fileName string) StructResult {
	// Extract field names
	fields := extractFields(structType)

	// Extract methods and their field usage
	methods := extractMethods(structName, file, fields)

	// If no methods, LCOM4 is 0
	if len(methods) == 0 {
		return StructResult{
			StructName:       structName,
			FilePath:         fileName,
			LCOM4Score:       0,
			ComponentDetails: [][]string{},
		}
	}

	// Build Union-Find graph: both methods and fields are nodes
	uf := newUnionFind()

	// Add all methods as nodes
	for _, method := range methods {
		uf.add(method.name)
	}

	// Add all fields as nodes
	for _, field := range fields {
		uf.add(field)
	}

	// Connect methods to fields they use
	for _, method := range methods {
		for field := range method.usedFields {
			uf.union(method.name, field)
		}
	}

	// Count connected components
	components := uf.getComponents()

	return StructResult{
		StructName:       structName,
		FilePath:         fileName,
		LCOM4Score:       len(components),
		ComponentDetails: components,
	}
}

// extractFields extracts all field names from a struct
func extractFields(structType *ast.StructType) []string {
	var fields []string
	if structType.Fields == nil {
		return fields
	}

	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			fields = append(fields, name.Name)
		}
	}
	return fields
}

// methodInfo holds information about a method
type methodInfo struct {
	name       string
	usedFields map[string]bool
}

// extractMethods finds all methods of a struct and tracks which fields they use
func extractMethods(structName string, file *ast.File, structFields []string) []methodInfo {
	var methods []methodInfo

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
				methods = append(methods, methodInfo{
					name:       funcDecl.Name.Name,
					usedFields: usedFields,
				})
			}
		}

		return true
	})

	return methods
}

// findUsedFields finds all fields accessed in a function body
func findUsedFields(body *ast.BlockStmt, recvName string, fieldMap map[string]bool) map[string]bool {
	usedFields := make(map[string]bool)

	if body == nil {
		return usedFields
	}

	ast.Inspect(body, func(n ast.Node) bool {
		// Look for selector expressions like "receiver.field"
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Check if this is accessing a field through the receiver
		if ident, ok := selector.X.(*ast.Ident); ok {
			// Only count if:
			// 1. The identifier matches the receiver name
			// 2. The selector name is actually a field of the struct
			if ident.Name == recvName && fieldMap[selector.Sel.Name] {
				usedFields[selector.Sel.Name] = true
			}
		}

		return true
	})

	return usedFields
}

// unionFind implements the Union-Find data structure for tracking connected components
type unionFind struct {
	parent map[string]string
	rank   map[string]int
}

// newUnionFind creates a new Union-Find instance
func newUnionFind() *unionFind {
	return &unionFind{
		parent: make(map[string]string),
		rank:   make(map[string]int),
	}
}

// add adds a new node to the graph
func (uf *unionFind) add(node string) {
	if _, exists := uf.parent[node]; !exists {
		uf.parent[node] = node
		uf.rank[node] = 0
	}
}

// find finds the root of a node with path compression
func (uf *unionFind) find(node string) string {
	if uf.parent[node] != node {
		uf.parent[node] = uf.find(uf.parent[node]) // Path compression
	}
	return uf.parent[node]
}

// union merges two components
func (uf *unionFind) union(node1, node2 string) {
	root1 := uf.find(node1)
	root2 := uf.find(node2)

	if root1 == root2 {
		return
	}

	// Union by rank
	if uf.rank[root1] < uf.rank[root2] {
		uf.parent[root1] = root2
	} else if uf.rank[root1] > uf.rank[root2] {
		uf.parent[root2] = root1
	} else {
		uf.parent[root2] = root1
		uf.rank[root1]++
	}
}

// getComponents returns all connected components
func (uf *unionFind) getComponents() [][]string {
	componentMap := make(map[string][]string)

	for node := range uf.parent {
		root := uf.find(node)
		componentMap[root] = append(componentMap[root], node)
	}

	components := make([][]string, 0, len(componentMap))
	for _, component := range componentMap {
		components = append(components, component)
	}

	return components
}
