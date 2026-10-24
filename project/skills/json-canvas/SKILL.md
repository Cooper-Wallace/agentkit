---
name: json-canvas
description: "Author, update, and programmatically generate visual JSON Canvas (.canvas) files adhering to the open JSON Canvas specification for architecture diagrams, dependency graphs, and agent orchestration DAGs."
---

# JSON Canvas (.canvas) Open Specification Skill

This skill teaches AI agents how to generate, inspect, and maintain visual infinite canvas diagrams (`.canvas` files) conforming to the official **JSON Canvas open specification** (`jsoncanvas.org`) created by Obsidian.

Canvas files provide an intuitive visual companion to Markdown documentation, enabling human developers to explore architecture maps, multi-agent workflows, and cross-repo dependencies directly in Obsidian.

---

## 1. Specification Overview

A `.canvas` file is a UTF-8 JSON document containing two primary arrays: `nodes` and `edges`.

```json
{
  "nodes": [],
  "edges": []
}
```

Every element must have a unique alphanumeric `id` (e.g., 16-character hex string `a1b2c3d4e5f67890` or semantic kebab-case ID like `node-layer-0`).

---

## 2. Node Types & Attributes

All nodes share common spatial coordinates:
- `id` (string, required): Unique identifier.
- `type` (string, required): One of `"text"`, `"file"`, `"link"`, or `"group"`.
- `x` (number, required): X coordinate on canvas plane (integer or float).
- `y` (number, required): Y coordinate on canvas plane (integer or float).
- `width` (number, required): Width in pixels.
- `height` (number, required): Height in pixels.
- `color` (string, optional): Visual accent color. Can be a 1-digit preset code:
  - `"1"`: Red
  - `"2"`: Orange
  - `"3"`: Yellow
  - `"4"`: Green
  - `"5"`: Cyan / Blue
  - `"6"`: Purple
  Or a 6-digit hex code: `"#3b82f6"`, `"#10b981"`, etc.

### 2.1 Text Node (`type: "text"`)
Embeds direct Markdown text directly onto the canvas.
```json
{
  "id": "node-intro",
  "type": "text",
  "x": 0,
  "y": 0,
  "width": 320,
  "height": 180,
  "text": "### Layer 2: agentkit\n- CLI tool permissions\n- Cognitive model profiles\n- Quota management",
  "color": "5"
}
```

### 2.2 File Node (`type: "file"`)
References and embeds an AgentVault Markdown file or media asset.
```json
{
  "id": "node-project-agentkit",
  "type": "file",
  "x": 380,
  "y": 0,
  "width": 400,
  "height": 300,
  "file": "Projects/agentkit.md",
  "subpath": "#Current State",
  "color": "4"
}
```
- `file` (string, required): Relative path inside the AgentVault.
- `subpath` (string, optional): Heading anchor (`#Heading`) or block reference (`#^blockid`).

### 2.3 Link Node (`type: "link"`)
Embeds a previewable external web URL.
```json
{
  "id": "node-kepano-skills",
  "type": "link",
  "x": 840,
  "y": 0,
  "width": 360,
  "height": 260,
  "url": "https://github.com/kepano/obsidian-skills"
}
```

### 2.4 Group Node (`type: "group"`)
Visual container bounding related nodes with an optional title label and background styling.
```json
{
  "id": "group-workstation",
  "type": "group",
  "x": -40,
  "y": -60,
  "width": 1300,
  "height": 500,
  "label": "Workstation Governance Layer",
  "color": "6"
}
```

---

## 3. Edge Attributes & Connections

Edges define directional or bidirectional links between nodes.

- `id` (string, required): Unique identifier.
- `fromNode` (string, required): Node ID where the edge originates.
- `fromSide` (string, optional): `"top"` | `"right"` | `"bottom"` | `"left"`.
- `fromEnd` (string, optional): `"none"` (default) or `"arrow"`.
- `toNode` (string, required): Target node ID.
- `toSide` (string, optional): `"top"` | `"right"` | `"bottom"` | `"left"`.
- `toEnd` (string, optional): `"arrow"` (default) or `"none"`.
- `label` (string, optional): Text label displayed along the path.
- `color` (string, optional): Preset color `"1"`-`"6"` or hex string.

```json
{
  "id": "edge-layer1-to-layer2",
  "fromNode": "node-layer-1",
  "fromSide": "bottom",
  "toNode": "node-layer-2",
  "toSide": "top",
  "toEnd": "arrow",
  "label": "Provides toolchains & paths",
  "color": "5"
}
```

---

## 4. Standard Workstation Stack Canvas Generator

When synthesizing architecture diagrams for the 4-layer workstation stack, agents should generate structured `.canvas` files into the Obsidian AgentVault at `$HOME/Documents/AgentVault/Architecture/` (or `$AGENTVAULT/Architecture/`, `$VAULT/Architecture/`, e.g. `Workstation-Stack.canvas`).

### Complete Example: `Workstation-Stack.canvas`

```json
{
  "nodes": [
    {
      "id": "group-stack",
      "type": "group",
      "x": 0,
      "y": 0,
      "width": 1000,
      "height": 960,
      "label": "4-Layer Workstation Engineering Stack",
      "color": "6"
    },
    {
      "id": "node-layer-0",
      "type": "text",
      "x": 100,
      "y": 60,
      "width": 800,
      "height": 160,
      "text": "## Layer 0: riced-kubuntu-os (RKO)\n- **Scope**: Kubuntu OS packages, KDE Plasma 6 desktop styling, system-level dotfiles\n- **Role**: Base OS foundation and hardware-level environment",
      "color": "1"
    },
    {
      "id": "node-layer-1",
      "type": "text",
      "x": 100,
      "y": 280,
      "width": 800,
      "height": 160,
      "text": "## Layer 1: workstation-setup\n- **Scope**: Developer toolchains (Node.js, Python, uv), shell profiles (`~/.profile`, `~/.bashrc`), keyrings\n- **Role**: Environment bridging and developer runtimes",
      "color": "2"
    },
    {
      "id": "node-layer-2",
      "type": "text",
      "x": 100,
      "y": 500,
      "width": 800,
      "height": 180,
      "text": "## Layer 2: agentkit (CLI: `agentkit`)\n- **Scope**: AI agent harness standards, pre-approved CLI permissions, switchable model profiles, quota telemetry, Obsidian AgentVault\n- **Role**: Cognitive tiering, autonomous orchestration, long-term memory",
      "color": "4"
    },
    {
      "id": "node-layer-3",
      "type": "text",
      "x": 100,
      "y": 740,
      "width": 800,
      "height": 160,
      "text": "## Layer 3: repokit\n- **Scope**: GitHub repository governance, branch protection rulesets, squash merge enforcement, semantic PR checks\n- **Role**: Trunk hygiene and pull request quality gate",
      "color": "5"
    }
  ],
  "edges": [
    {
      "id": "edge-0-to-1",
      "fromNode": "node-layer-0",
      "fromSide": "bottom",
      "toNode": "node-layer-1",
      "toSide": "top",
      "toEnd": "arrow",
      "label": "Provides OS Packages & Display Server"
    },
    {
      "id": "edge-1-to-2",
      "fromNode": "node-layer-1",
      "fromSide": "bottom",
      "toNode": "node-layer-2",
      "toSide": "top",
      "toEnd": "arrow",
      "label": "Provides Python 3 Runtimes & Paths"
    },
    {
      "id": "edge-2-to-3",
      "fromNode": "node-layer-2",
      "fromSide": "bottom",
      "toNode": "node-layer-3",
      "toSide": "top",
      "toEnd": "arrow",
      "label": "Enforces Agent Directives & Git Hygiene"
    }
  ]
}
```

---

## 5. Agent Best Practices for Canvas Authoring

1. **Deterministic Grid Positioning**:
   - Align nodes to a 20px or 40px grid.
   - Maintain consistent spacing between rows (e.g. 60-100px vertical gap) and columns.
2. **Standard Sizing**:
   - Compact status cards: `width: 320`, `height: 180`
   - Detailed component cards: `width: 480` to `800`, `height: 160` to `300`
3. **Semantic Color Usage**:
   - `1` (Red): Failing tests, security boundaries, destructive warnings
   - `2` (Orange): In-flight work, review gates, environment configs
   - `4` (Green): Verified features, core skills, passing tests
   - `5` (Blue/Cyan): Architecture layers, CLI tooling, interfaces
   - `6` (Purple): High-level groups, orchestration DAGs, root planners
4. **Validation**:
   - Always ensure JSON syntax is strictly valid before writing `.canvas` files.
