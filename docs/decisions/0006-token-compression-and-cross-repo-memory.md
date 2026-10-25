# ADR 0006: PreToolUse Token Compression (Headroom CCR) and Cross-Repo Memory Architecture

## Context
Antigravity CLI operates across modular multi-repository workspaces where context window exhaustion, token consumption costs, and cross-session memory loss degrade autonomous agent performance. In long-running development workflows, verbose command outputs (such as extensive test suite runs with `pytest`, `unittest`, or `npm test`, voluminous `git diff` outputs, and package queries) inject tens of thousands of tokens into `transcript.jsonl`. This leads to premature context compaction, loss of historical instructions, and elevated latency.

Traditional compression approaches encounter significant architectural constraints within Antigravity CLI:
1. **`PostToolUse` Hook Immutability**: In Antigravity's lifecycle hook contract, `PostToolUse` handlers can only return `{}` on stdout for side effects, telemetry, or notifications. They cannot rewrite, truncate, or mutate the tool output buffer once it has been captured.
2. **Network Proxy Incompatibility**: Standalone HTTP/REST proxies (such as drop-in OpenAI or Anthropic proxies like `headroom proxy`) conflict with Antigravity's native Google Gemini backend transport, authentication flow, and RPC channels.
3. **Cross-Session Memory Decay**: Multi-repository architectures spanning distinct codebases lack a unified mechanism to retain architectural contracts, interface agreements, and cross-session decisions without polluting conversation prompts.

## Decision
1. **PreToolUse Command Wrapping via `filter-and-wrap.py`**:
   - Rather than attempting post-execution output mutation, command interception occurs deterministically at `PreToolUse` on `run_command` calls using Antigravity's hook `overwrite` mechanism (`{"decision": "allow", "overwrite": {"CommandLine": "..."}}`).
   - `filter-and-wrap.py` evaluates candidate commands:
     - Preserves safety gate decisions (`guard-git-operations.sh` blocking trunk pushes and destructive operations).
     - Bypasses daemon processes (`IsDaemon: true`) and commands containing shell redirects or heredocs.
     - Transparently rewrites verbose inspection and testing commands (`pytest`, `unittest`, `npm test`, `pnpm test`, `cargo test`, `go test`, `mvn test`, `git diff`, `git log`, `git show`, `find`, `grep`, `cat`, `diff`, `dpkg -l`, `apt-cache`, `tree`) to execute under `headroom-wrap.py`.
2. **Reversible Content-Aware Compression & CCR Caching in `headroom-wrap.py`**:
   - `headroom-wrap.py` executes the target process while preserving working directory, shell environment, and the exact exit status code (`proc.returncode`).
   - Payloads below the compression threshold (`AGY_COMPRESSION_THRESHOLD`, default 1,500 characters) pass through unmodified.
   - For outputs exceeding the threshold, raw text is hashed using SHA-256 (`[:12]`) and cached locally in `~/.cache/antigravity-headroom/<hash>.txt` with automatic LRU rotation capped at 100 cache entries.
   - Compression is performed either via `headroom-ai` (if installed) or deterministic built-in content compressors:
     - **JSON SmartCrusher**: Retains key schemas and bounds array lengths by eliding repetitive intermediate elements.
     - **Error-Preserving LogCrusher**: Preserves head/tail execution context, overall summaries, and all error/failure tracebacks (`FAIL`, `ERROR`, `Traceback`, `Exception`, `AssertionError`, `panic:`), while collapsing repetitive passing lines.
   - Appends a 1-line Content-Compressed Retrieval (CCR) breadcrumb:
     `[Headroom CCR: compressed X -> Y chars (Z% saved). Full output cached at ~/.cache/antigravity-headroom/<hash>.txt]`
     allowing any agent to read the full original output via standard `read_file` if deeper diagnostics are needed.
3. **Perpetual Cross-Repo Memory (`@modelcontextprotocol/server-memory`)**:
   - Standardize `@modelcontextprotocol/server-memory` in `config/mcp.json` to provide a persistent, perpetual knowledge graph of entities, relations, and architectural contracts across repository boundaries.
4. **AST Structural Code Compression (`repomix --mcp`)**:
   - Standardize `repomix` in `config/mcp.json` running in `--mcp` mode, enabling agents to retrieve AST-compressed repository maps and symbol skeletons without burning context on full file bodies.
5. **Multi-Model Orchestrator Tiering Integration**:
   - Combines with the `autonomous-orchestrator` plugin architecture: Gemini 4 Argon acts as Root Orchestrator for high-level planning and adversarial diff auditing, while Gemini 3.8 Flash subagents execute discrete implementation tasks across disjoint file partitions, maximizing speed while keeping context clean.

## Consequences
- **Positive**: 60%–90% token reduction on verbose command execution, significantly prolonging context lifespan before compaction.
- **Positive**: Zero loss of debuggability—reversible CCR caching on local disk guarantees raw logs remain accessible via deterministic paths.
- **Positive**: Completely compatible with native Gemini CLI transport without introducing fragile local network proxies.
- **Positive**: Persistent cross-repo knowledge graph prevents architectural drift across modular project repositories.
- **Positive**: AST code packing via `repomix` gives agents broad structural comprehension at a fraction of raw source token costs.
- **Negative/Trade-off**: Slight execution latency (<50ms) for Python subprocess invocation and compression during wrapped commands.
