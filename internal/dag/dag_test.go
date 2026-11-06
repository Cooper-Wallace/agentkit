package dag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDAGAddNode(t *testing.T) {
	d := NewDAG("test-dag", "Test DAG")

	// Nil node
	if err := d.AddNode(nil); err == nil {
		t.Fatal("expected error adding nil node, got nil")
	}

	// Empty ID
	if err := d.AddNode(&Node{ID: ""}); err == nil {
		t.Fatal("expected error adding node with empty ID, got nil")
	}

	// Valid node with defaults
	err := d.AddNode(&Node{
		ID:   "node-1",
		Role: "Worker",
	})
	if err != nil {
		t.Fatalf("unexpected error adding node: %v", err)
	}

	n, exists := d.GetNode("node-1")
	if !exists {
		t.Fatal("expected node-1 to exist in DAG")
	}
	if n.Complexity != ComplexityMed {
		t.Errorf("expected default complexity 'med', got %q", n.Complexity)
	}
	if n.State != StatePending {
		t.Errorf("expected default state 'pending', got %q", n.State)
	}

	// Duplicate ID
	if err := d.AddNode(&Node{ID: "node-1"}); err == nil {
		t.Fatal("expected error adding duplicate node ID, got nil")
	}

	// Invalid complexity
	if err := d.AddNode(&Node{ID: "node-2", Complexity: "ultra"}); err == nil {
		t.Fatal("expected error adding invalid complexity, got nil")
	}

	// Invalid state
	if err := d.AddNode(&Node{ID: "node-3", State: "sleeping"}); err == nil {
		t.Fatal("expected error adding invalid state, got nil")
	}
}

func TestDAGAddDependency(t *testing.T) {
	d := NewDAG("test-deps", "Test Dependencies")
	_ = d.AddNode(&Node{ID: "A", Complexity: ComplexityLow})
	_ = d.AddNode(&Node{ID: "B", Complexity: ComplexityMed})

	// Valid dependency: B depends on A
	if err := d.AddDependency("B", "A"); err != nil {
		t.Fatalf("unexpected error adding dependency: %v", err)
	}

	// Self-dependency
	if err := d.AddDependency("A", "A"); err == nil {
		t.Fatal("expected error for self-dependency, got nil")
	}

	// Unknown node
	if err := d.AddDependency("B", "nonexistent"); err == nil {
		t.Fatal("expected error referencing unknown prerequisite, got nil")
	}
	if err := d.AddDependency("nonexistent", "A"); err == nil {
		t.Fatal("expected error referencing unknown dependent, got nil")
	}

	// Idempotent duplicate dependency
	if err := d.AddDependency("B", "A"); err != nil {
		t.Fatalf("unexpected error re-adding same dependency: %v", err)
	}
	nB, _ := d.GetNode("B")
	if len(nB.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(nB.Dependencies))
	}
}

func TestDAGTopologicalSortAndExecutionLayers(t *testing.T) {
	d := DefaultFeatureDAG("sample-feature")

	// Validate DAG
	if err := d.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	// Topological sort
	sorted, err := d.TopologicalSort()
	if err != nil {
		t.Fatalf("topological sort failed: %v", err)
	}
	if len(sorted) != 5 {
		t.Fatalf("expected 5 sorted nodes, got %d", len(sorted))
	}

	// Position checks
	indices := make(map[string]int)
	for i, n := range sorted {
		indices[n.ID] = i
	}

	if indices["p1-planning"] > indices["p2a-backend"] || indices["p1-planning"] > indices["p2b-cli"] {
		t.Error("p1-planning must precede p2a and p2b")
	}
	if indices["p2a-backend"] > indices["p3-verification"] || indices["p2b-cli"] > indices["p3-verification"] {
		t.Error("p2a and p2b must precede p3-verification")
	}
	if indices["p3-verification"] > indices["p4-wrapup"] {
		t.Error("p3-verification must precede p4-wrapup")
	}

	// Execution layers
	layers, err := d.ExecutionLayers()
	if err != nil {
		t.Fatalf("execution layers failed: %v", err)
	}
	if len(layers) != 4 {
		t.Fatalf("expected 4 execution layers, got %d", len(layers))
	}

	// Layer 0: [p1-planning]
	if len(layers[0]) != 1 || layers[0][0].ID != "p1-planning" {
		t.Errorf("unexpected layer 0: %v", layers[0])
	}
	// Layer 1: [p2a-backend, p2b-cli]
	if len(layers[1]) != 2 {
		t.Errorf("expected 2 parallel nodes in layer 1, got %d", len(layers[1]))
	}
	// Layer 2: [p3-verification]
	if len(layers[2]) != 1 || layers[2][0].ID != "p3-verification" {
		t.Errorf("unexpected layer 2: %v", layers[2])
	}
	// Layer 3: [p4-wrapup]
	if len(layers[3]) != 1 || layers[3][0].ID != "p4-wrapup" {
		t.Errorf("unexpected layer 3: %v", layers[3])
	}
}

func TestDAGCycleDetection(t *testing.T) {
	d := NewDAG("cyclic-dag", "Cyclic DAG")
	_ = d.AddNode(&Node{ID: "A"})
	_ = d.AddNode(&Node{ID: "B"})
	_ = d.AddNode(&Node{ID: "C"})

	_ = d.AddDependency("B", "A") // B depends on A
	_ = d.AddDependency("C", "B") // C depends on B
	_ = d.AddDependency("A", "C") // A depends on C (Cycle!)

	err := d.Validate()
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected error mentioning 'cycle', got %q", err.Error())
	}

	_, sortErr := d.TopologicalSort()
	if sortErr == nil {
		t.Fatal("expected topological sort to fail on cycle, got nil")
	}

	_, layerErr := d.ExecutionLayers()
	if layerErr == nil {
		t.Fatal("expected execution layers to fail on cycle, got nil")
	}
}

func TestDAGReadyNodesAndStateTransitions(t *testing.T) {
	d := DefaultFeatureDAG("lifecycle-test")

	// Initially, only layer 0 is ready
	ready := d.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "p1-planning" {
		t.Fatalf("expected only p1-planning ready, got %v", ready)
	}

	if d.IsCompleted() {
		t.Fatal("DAG should not be completed initially")
	}
	if d.HasFailed() {
		t.Fatal("DAG should not have failed initially")
	}

	comp, total, pct := d.Progress()
	if comp != 0 || total != 5 || pct != 0.0 {
		t.Fatalf("expected 0/5 (0%%), got %d/%d (%.1f%%)", comp, total, pct)
	}

	// Run p1-planning
	_ = d.SetNodeState("p1-planning", StateRunning, "")
	p1, _ := d.GetNode("p1-planning")
	if p1.StartedAt == nil {
		t.Fatal("expected StartedAt to be set")
	}

	// Complete p1-planning
	_ = d.SetNodeState("p1-planning", StateCompleted, "Contracts frozen")
	if p1.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}

	// Now p2a-backend and p2b-cli must both be ready
	ready = d.ReadyNodes()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready nodes, got %d", len(ready))
	}

	// Complete p2a-backend
	_ = d.SetNodeState("p2a-backend", StateCompleted, "Backend done")
	ready = d.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "p2b-cli" {
		t.Fatalf("expected only p2b-cli ready, got %v", ready)
	}

	// Complete p2b-cli
	_ = d.SetNodeState("p2b-cli", StateCompleted, "CLI done")
	ready = d.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "p3-verification" {
		t.Fatalf("expected p3-verification ready, got %v", ready)
	}

	// Mark p3-verification failed
	_ = d.SetNodeState("p3-verification", StateFailed, "Test assertion error")
	if !d.HasFailed() {
		t.Fatal("DAG should report HasFailed() = true")
	}

	// Re-run and complete p3-verification
	_ = d.SetNodeState("p3-verification", StateCompleted, "All tests passing")
	_ = d.SetNodeState("p4-wrapup", StateCompleted, "Handoff generated")

	if !d.IsCompleted() {
		t.Fatal("DAG should be completed")
	}
	comp, total, pct = d.Progress()
	if comp != 5 || total != 5 || pct != 100.0 {
		t.Fatalf("expected 5/5 (100%%), got %d/%d (%.1f%%)", comp, total, pct)
	}

	// Set invalid node state
	if err := d.SetNodeState("nonexistent", StateCompleted, ""); err == nil {
		t.Fatal("expected error updating unknown node, got nil")
	}
}

func TestDAGSerializationAndSQLite(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "dag_test.json")

	d := DefaultFeatureDAG("serialization-test")
	_ = d.SetNodeState("p1-planning", StateCompleted, "Plan ready")

	// SaveToFile
	if err := d.SaveToFile(filePath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// LoadFromFile
	loaded, err := LoadFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	if loaded.ID != d.ID {
		t.Errorf("expected ID %q, got %q", d.ID, loaded.ID)
	}
	if len(loaded.Nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(loaded.Nodes))
	}
	p1, ok := loaded.GetNode("p1-planning")
	if !ok || p1.State != StateCompleted || p1.Result != "Plan ready" {
		t.Errorf("p1-planning state mismatch in loaded DAG: %+v", p1)
	}

	// SQLite Schema and SQL statements
	schema := SQLiteSchema()
	if !strings.Contains(schema, "CREATE TABLE IF NOT EXISTS dags") ||
		!strings.Contains(schema, "CREATE TABLE IF NOT EXISTS nodes") ||
		!strings.Contains(schema, "CREATE TABLE IF NOT EXISTS node_dependencies") {
		t.Errorf("invalid SQLite schema: %s", schema)
	}

	inserts := loaded.ToSQLiteInserts()
	if !strings.Contains(inserts, "INSERT OR REPLACE INTO dags") ||
		!strings.Contains(inserts, "INSERT OR REPLACE INTO nodes") ||
		!strings.Contains(inserts, "INSERT OR REPLACE INTO node_dependencies") {
		t.Errorf("invalid SQLite inserts: %s", inserts)
	}

	// Corrupt JSON file load
	corruptPath := filepath.Join(tempDir, "corrupt.json")
	_ = os.WriteFile(corruptPath, []byte("invalid-json"), 0644)
	if _, err := LoadFromFile(corruptPath); err == nil {
		t.Fatal("expected error loading corrupt JSON, got nil")
	}
}
