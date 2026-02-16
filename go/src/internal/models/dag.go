package models

import (
	"fmt"
)

// SubtaskDependency represents a dependency edge between two subtasks.
type SubtaskDependency struct {
	DependentID  string `json:"dependent_id"`
	DependencyID string `json:"dependency_id"`
}

// ValidateDAG checks if the given dependencies form a valid directed acyclic graph.
// Returns an error if a cycle is detected or if any task ID is invalid.
func ValidateDAG(subtaskIDs []string, dependencies []SubtaskDependency) error {
	if len(dependencies) == 0 {
		return nil // No dependencies means no cycles
	}

	// Build ID set for validation
	idSet := make(map[string]bool)
	for _, id := range subtaskIDs {
		idSet[id] = true
	}

	// Build adjacency list
	graph := make(map[string][]string)
	for _, dep := range dependencies {
		// Validate both IDs exist in the subtask list
		if !idSet[dep.DependentID] {
			return fmt.Errorf("dependent task %q not found in subtask list", dep.DependentID)
		}
		if !idSet[dep.DependencyID] {
			return fmt.Errorf("dependency task %q not found in subtask list", dep.DependencyID)
		}

		graph[dep.DependentID] = append(graph[dep.DependentID], dep.DependencyID)
	}

	// Detect cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for id := range idSet {
		if !visited[id] {
			if hasCycle(id, graph, visited, recStack) {
				return fmt.Errorf("circular dependency detected involving task %q", id)
			}
		}
	}

	return nil
}

// hasCycle performs DFS to detect cycles in the dependency graph.
func hasCycle(node string, graph map[string][]string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true

	for _, neighbor := range graph[node] {
		if !visited[neighbor] {
			if hasCycle(neighbor, graph, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[node] = false
	return false
}
