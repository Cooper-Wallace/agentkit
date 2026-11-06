package dag

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NodeState describes the lifecycle state of a DAG task node.
type NodeState string

const (
	StatePending   NodeState = "pending"
	StateRunning   NodeState = "running"
	StateCompleted NodeState = "completed"
	StateFailed    NodeState = "failed"
)

// Complexity indicates the cognitive resource profile for the subagent worker.
type Complexity string

const (
	ComplexityLow  Complexity = "low"
	ComplexityMed  Complexity = "med"
	ComplexityHigh Complexity = "high"
)

// Node represents a discrete execution task within a multi-agent DAG.
type Node struct {
	ID           string     `json:"id"`
	Role         string     `json:"role"`
	Prompt       string     `json:"prompt"`
	Complexity   Complexity `json:"complexity"`
	Dependencies []string   `json:"dependencies"`
	State        NodeState  `json:"state"`
	Result       string     `json:"result,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// DAG represents a directed acyclic graph of task nodes.
type DAG struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Nodes     map[string]*Node `json:"nodes"`
	NodeOrder []string         `json:"node_order"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// NewDAG creates an empty execution DAG.
func NewDAG(id, name string) *DAG {
	now := time.Now()
	return &DAG{
		ID:        id,
		Name:      name,
		Nodes:     make(map[string]*Node),
		NodeOrder: []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddNode adds a new task node to the DAG.
func (d *DAG) AddNode(node *Node) error {
	if node == nil {
		return errors.New("cannot add nil node")
	}
	id := strings.TrimSpace(node.ID)
	if id == "" {
		return errors.New("node ID cannot be empty")
	}
	if _, exists := d.Nodes[id]; exists {
		return fmt.Errorf("node with ID %q already exists in DAG", id)
	}

	complexity := node.Complexity
	switch complexity {
	case ComplexityLow, ComplexityMed, ComplexityHigh:
	case "":
		complexity = ComplexityMed
	default:
		return fmt.Errorf("invalid complexity %q: must be low, med, or high", complexity)
	}

	state := node.State
	switch state {
	case StatePending, StateRunning, StateCompleted, StateFailed:
	case "":
		state = StatePending
	default:
		return fmt.Errorf("invalid node state %q", state)
	}

	deps := node.Dependencies
	if deps == nil {
		deps = []string{}
	}

	clonedNode := &Node{
		ID:           id,
		Role:         node.Role,
		Prompt:       node.Prompt,
		Complexity:   complexity,
		Dependencies: deps,
		State:        state,
		Result:       node.Result,
		StartedAt:    node.StartedAt,
		CompletedAt:  node.CompletedAt,
	}

	d.Nodes[id] = clonedNode
	d.NodeOrder = append(d.NodeOrder, id)
	d.UpdatedAt = time.Now()
	return nil
}

// AddDependency configures nodeID to depend on dependsOnID.
func (d *DAG) AddDependency(nodeID, dependsOnID string) error {
	nodeID = strings.TrimSpace(nodeID)
	dependsOnID = strings.TrimSpace(dependsOnID)

	if nodeID == "" || dependsOnID == "" {
		return errors.New("node IDs cannot be empty")
	}
	if nodeID == dependsOnID {
		return fmt.Errorf("self-dependency not permitted on node %q", nodeID)
	}

	node, exists := d.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("dependent node %q not found in DAG", nodeID)
	}
	if _, exists := d.Nodes[dependsOnID]; !exists {
		return fmt.Errorf("prerequisite node %q not found in DAG", dependsOnID)
	}

	for _, dep := range node.Dependencies {
		if dep == dependsOnID {
			return nil // already registered
		}
	}

	node.Dependencies = append(node.Dependencies, dependsOnID)
	d.UpdatedAt = time.Now()
	return nil
}

// GetNode retrieves a node by its identifier.
func (d *DAG) GetNode(id string) (*Node, bool) {
	node, exists := d.Nodes[id]
	return node, exists
}

// Validate verifies node references and asserts absence of cycles.
func (d *DAG) Validate() error {
	// 1. Verify all dependencies exist
	for id, node := range d.Nodes {
		for _, dep := range node.Dependencies {
			if _, exists := d.Nodes[dep]; !exists {
				return fmt.Errorf("node %q references unknown dependency %q", id, dep)
			}
		}
	}

	// 2. Cycle detection via topological sort
	_, err := d.TopologicalSort()
	return err
}

// TopologicalSort returns a linear dependency execution order using Kahn's algorithm.
func (d *DAG) TopologicalSort() ([]*Node, error) {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // from dep -> list of nodes that depend on dep

	for id := range d.Nodes {
		inDegree[id] = 0
	}

	for id, node := range d.Nodes {
		inDegree[id] = len(node.Dependencies)
		for _, dep := range node.Dependencies {
			dependents[dep] = append(dependents[dep], id)
		}
	}

	var queue []string
	for _, id := range d.NodeOrder {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []*Node
	for len(queue) > 0 {
		currID := queue[0]
		queue = queue[1:]
		sorted = append(sorted, d.Nodes[currID])

		// For all nodes that depend on currID, decrement in-degree
		for _, depID := range dependents[currID] {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				queue = append(queue, depID)
			}
		}
	}

	if len(sorted) != len(d.Nodes) {
		return nil, errors.New("cycle detected in execution DAG")
	}

	return sorted, nil
}

// ExecutionLayers groups nodes into parallel dispatch layers.
// All nodes in layer 0 have no dependencies; all nodes in layer K depend only on layers < K.
func (d *DAG) ExecutionLayers() ([][]*Node, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}

	nodeLayer := make(map[string]int)

	// Calculate layer for each node
	sorted, err := d.TopologicalSort()
	if err != nil {
		return nil, err
	}

	maxLayer := 0
	for _, node := range sorted {
		layer := 0
		for _, dep := range node.Dependencies {
			depLayer := nodeLayer[dep]
			if depLayer+1 > layer {
				layer = depLayer + 1
			}
		}
		nodeLayer[node.ID] = layer
		if layer > maxLayer {
			maxLayer = layer
		}
	}

	layers := make([][]*Node, maxLayer+1)
	for _, id := range d.NodeOrder {
		l := nodeLayer[id]
		layers[l] = append(layers[l], d.Nodes[id])
	}

	return layers, nil
}

// ReadyNodes returns all pending nodes whose prerequisites are all completed.
func (d *DAG) ReadyNodes() []*Node {
	var ready []*Node
	for _, id := range d.NodeOrder {
		node := d.Nodes[id]
		if node.State != StatePending {
			continue
		}

		allDepsCompleted := true
		for _, depID := range node.Dependencies {
			depNode, exists := d.Nodes[depID]
			if !exists || depNode.State != StateCompleted {
				allDepsCompleted = false
				break
			}
		}

		if allDepsCompleted {
			ready = append(ready, node)
		}
	}
	return ready
}

// SetNodeState updates the state and result of a task node.
func (d *DAG) SetNodeState(id string, state NodeState, result string) error {
	node, exists := d.Nodes[id]
	if !exists {
		return fmt.Errorf("node %q not found in DAG", id)
	}

	node.State = state
	node.Result = result
	now := time.Now()

	switch state {
	case StateRunning:
		if node.StartedAt == nil {
			node.StartedAt = &now
		}
	case StateCompleted, StateFailed:
		if node.StartedAt == nil {
			node.StartedAt = &now
		}
		node.CompletedAt = &now
	}

	d.UpdatedAt = now
	return nil
}

// IsCompleted returns true if every node in the DAG has completed successfully.
func (d *DAG) IsCompleted() bool {
	if len(d.Nodes) == 0 {
		return false
	}
	for _, node := range d.Nodes {
		if node.State != StateCompleted {
			return false
		}
	}
	return true
}

// HasFailed returns true if any node in the DAG has failed.
func (d *DAG) HasFailed() bool {
	for _, node := range d.Nodes {
		if node.State == StateFailed {
			return true
		}
	}
	return false
}

// Progress returns completed node count, total node count, and percentage.
func (d *DAG) Progress() (int, int, float64) {
	total := len(d.Nodes)
	if total == 0 {
		return 0, 0, 0.0
	}
	completed := 0
	for _, node := range d.Nodes {
		if node.State == StateCompleted {
			completed++
		}
	}
	percent := (float64(completed) / float64(total)) * 100.0
	return completed, total, percent
}

// ToJSON marshals the DAG to JSON bytes.
func (d *DAG) ToJSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

// FromJSON unmarshals JSON data into the DAG.
func FromJSON(data []byte) (*DAG, error) {
	var d DAG
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	if d.Nodes == nil {
		d.Nodes = make(map[string]*Node)
	}
	return &d, nil
}

// SaveToFile writes the DAG JSON to disk.
func (d *DAG) SaveToFile(filePath string) error {
	_ = os.MkdirAll(filepath.Dir(filePath), 0755)
	data, err := d.ToJSON()
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// LoadFromFile reads and parses a DAG JSON file from disk.
func LoadFromFile(filePath string) (*DAG, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read DAG file %s: %w", filePath, err)
	}
	return FromJSON(data)
}

// SQLiteSchema returns the DDL statements for DAG state persistence.
func SQLiteSchema() string {
	return `CREATE TABLE IF NOT EXISTS dags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    state TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    dag_id TEXT NOT NULL,
    role TEXT NOT NULL,
    prompt TEXT NOT NULL,
    complexity TEXT NOT NULL,
    state TEXT NOT NULL,
    result TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY(dag_id) REFERENCES dags(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS node_dependencies (
    dag_id TEXT NOT NULL,
    node_id TEXT NOT NULL,
    depends_on_id TEXT NOT NULL,
    PRIMARY KEY (dag_id, node_id, depends_on_id),
    FOREIGN KEY(dag_id) REFERENCES dags(id) ON DELETE CASCADE
);
`
}

// ToSQLiteInserts generates SQL INSERT OR REPLACE statements to checkpoint the DAG in SQLite.
func (d *DAG) ToSQLiteInserts() string {
	var sb strings.Builder

	overallState := "pending"
	if d.IsCompleted() {
		overallState = "completed"
	} else if d.HasFailed() {
		overallState = "failed"
	} else {
		for _, n := range d.Nodes {
			if n.State == StateRunning || n.State == StateCompleted {
				overallState = "running"
				break
			}
		}
	}

	sb.WriteString(fmt.Sprintf(
		"INSERT OR REPLACE INTO dags (id, name, state, created_at, updated_at) VALUES ('%s', '%s', '%s', '%s', '%s');\n",
		escapeSQL(d.ID),
		escapeSQL(d.Name),
		overallState,
		d.CreatedAt.Format(time.RFC3339),
		d.UpdatedAt.Format(time.RFC3339),
	))

	for _, id := range d.NodeOrder {
		node := d.Nodes[id]
		var startedStr, completedStr string
		if node.StartedAt != nil {
			startedStr = fmt.Sprintf("'%s'", node.StartedAt.Format(time.RFC3339))
		} else {
			startedStr = "NULL"
		}
		if node.CompletedAt != nil {
			completedStr = fmt.Sprintf("'%s'", node.CompletedAt.Format(time.RFC3339))
		} else {
			completedStr = "NULL"
		}

		sb.WriteString(fmt.Sprintf(
			"INSERT OR REPLACE INTO nodes (id, dag_id, role, prompt, complexity, state, result, started_at, completed_at) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s', %s, %s);\n",
			escapeSQL(node.ID),
			escapeSQL(d.ID),
			escapeSQL(node.Role),
			escapeSQL(node.Prompt),
			escapeSQL(string(node.Complexity)),
			escapeSQL(string(node.State)),
			escapeSQL(node.Result),
			startedStr,
			completedStr,
		))

		for _, dep := range node.Dependencies {
			sb.WriteString(fmt.Sprintf(
				"INSERT OR REPLACE INTO node_dependencies (dag_id, node_id, depends_on_id) VALUES ('%s', '%s', '%s');\n",
				escapeSQL(d.ID),
				escapeSQL(node.ID),
				escapeSQL(dep),
			))
		}
	}

	return sb.String()
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// DefaultFeatureDAG returns a standard 4-phase autonomous feature lifecycle DAG.
func DefaultFeatureDAG(name string) *DAG {
	if name == "" {
		name = "feature-lifecycle"
	}
	d := NewDAG(name, fmt.Sprintf("Feature Lifecycle: %s", name))

	_ = d.AddNode(&Node{
		ID:         "p1-planning",
		Role:       "Architectural Planner",
		Prompt:     "Phase 1: Contract Freeze, Architectural Plan & Subagent Task DAG generation.",
		Complexity: ComplexityHigh,
	})

	_ = d.AddNode(&Node{
		ID:           "p2a-backend",
		Role:         "Backend & Test Worker",
		Prompt:       "Phase 2: Implement core backend logic, internal packages, and unit tests.",
		Complexity:   ComplexityMed,
		Dependencies: []string{"p1-planning"},
	})

	_ = d.AddNode(&Node{
		ID:           "p2b-cli",
		Role:         "CLI & Interface Worker",
		Prompt:       "Phase 2: Implement command line interface, flags, human/JSON formatters.",
		Complexity:   ComplexityMed,
		Dependencies: []string{"p1-planning"},
	})

	_ = d.AddNode(&Node{
		ID:           "p3-verification",
		Role:         "Adversarial Reviewer",
		Prompt:       "Phase 3: Adversarial diff audit, hermetic test execution, and coverage verification.",
		Complexity:   ComplexityHigh,
		Dependencies: []string{"p2a-backend", "p2b-cli"},
	})

	_ = d.AddNode(&Node{
		ID:           "p4-wrapup",
		Role:         "Session Wrap-Up & Docs",
		Prompt:       "Phase 4: Update living project notes, sync AgentVault, and author session handoff.",
		Complexity:   ComplexityLow,
		Dependencies: []string{"p3-verification"},
	})

	return d
}
