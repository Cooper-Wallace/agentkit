"""
Tests for agy-standardise CLI tool and standardization engine.
"""

import importlib.machinery
import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
import unittest.mock
from pathlib import Path

# Add bin directory and load agy-standardise module
REPO_ROOT = Path(__file__).resolve().parent.parent
BIN_PATH = REPO_ROOT / "bin" / "agy-standardise"

loader = importlib.machinery.SourceFileLoader("agy_standardise", str(BIN_PATH))
spec = importlib.util.spec_from_loader("agy_standardise", loader)
agy_std = importlib.util.module_from_spec(spec)
loader.exec_module(agy_std)

COMMIT_PATTERN = agy_std.COMMIT_PATTERN
BIN_CMD = [sys.executable, str(BIN_PATH)]


class TestPermissionsCatalog(unittest.TestCase):
    def test_permissions_json_validity(self):
        catalog = agy_std.get_standard_permissions_catalog()
        self.assertIn("categories", catalog)
        self.assertGreater(len(catalog["categories"]), 0)

        for cat_name, cat_data in catalog["categories"].items():
            self.assertIn("commands", cat_data, f"Category {cat_name} missing 'commands'")
            self.assertGreater(len(cat_data["commands"]), 0, f"Category {cat_name} has empty commands")

    def test_flat_permissions_generation(self):
        flat = agy_std.get_flat_standard_permissions()
        self.assertGreater(len(flat), 20)
        for perm in flat:
            self.assertTrue(perm.startswith("command("), f"Invalid format: {perm}")
            self.assertTrue(perm.endswith(")"), f"Invalid format: {perm}")

    def test_settings_default_file_exists(self):
        default_settings = agy_std.load_json(agy_std.SETTINGS_DEFAULT_FILE)
        self.assertIn("allowNonWorkspaceAccess", default_settings)
        self.assertIn("colorScheme", default_settings)
        self.assertIn("model", default_settings)


class TestTemplatesAndArchetypes(unittest.TestCase):
    def test_hooks_template_validity(self):
        hooks_file = REPO_ROOT / "templates" / "hooks.json"
        self.assertTrue(hooks_file.exists())
        data = json.loads(hooks_file.read_text(encoding="utf-8"))
        self.assertIn("command-safety-guard", data)
        self.assertIn("code-linter-formatter", data)
        self.assertFalse(data["command-safety-guard"]["enabled"])

    def test_plugins_json_template_validity(self):
        plugins_file = REPO_ROOT / "templates" / "plugins.json"
        self.assertTrue(plugins_file.exists())
        data = json.loads(plugins_file.read_text(encoding="utf-8"))
        self.assertIn("entries", data)
        self.assertIsInstance(data["entries"], list)

    def test_plugin_archetype_validity(self):
        plugin_dir = REPO_ROOT / "templates" / "plugin"
        self.assertTrue(plugin_dir.exists())

        manifest = json.loads((plugin_dir / "plugin.json").read_text(encoding="utf-8"))
        self.assertIn("name", manifest)
        self.assertTrue(manifest.get("disabled", False))

        hooks = json.loads((plugin_dir / "hooks.json").read_text(encoding="utf-8"))
        self.assertIsInstance(hooks, dict)

        mcp = json.loads((plugin_dir / "mcp_config.json").read_text(encoding="utf-8"))
        self.assertIn("mcpServers", mcp)

        agents_rule = plugin_dir / "rules" / "AGENTS.md"
        self.assertTrue(agents_rule.exists())

        sample_skill = plugin_dir / "skills" / "sample-skill" / "SKILL.md"
        self.assertTrue(sample_skill.exists())


class TestPermissionMerger(unittest.TestCase):
    def test_merge_into_empty_settings(self):
        standard = ["command(ls)", "command(cat)"]
        default_settings = {"colorScheme": "dark"}
        merged, added = agy_std.merge_permissions({}, standard, default_settings)

        self.assertEqual(merged["colorScheme"], "dark")
        self.assertEqual(added, ["command(ls)", "command(cat)"])
        self.assertEqual(merged["permissions"]["allow"], ["command(ls)", "command(cat)"])
        self.assertEqual(merged["trustedWorkspaces"], [])

    def test_merge_preserves_custom_permissions(self):
        existing = {
            "permissions": {
                "allow": ["command(custom_script.sh)", "command(ls)"]
            },
            "trustedWorkspaces": ["/custom/path"],
            "model": "Custom Model"
        }
        standard = ["command(ls)", "command(cat)", "command(grep)"]
        merged, added = agy_std.merge_permissions(existing, standard)

        self.assertEqual(added, ["command(cat)", "command(grep)"])
        self.assertIn("command(custom_script.sh)", merged["permissions"]["allow"])
        self.assertIn("command(ls)", merged["permissions"]["allow"])
        self.assertIn("command(cat)", merged["permissions"]["allow"])
        self.assertIn("command(grep)", merged["permissions"]["allow"])
        self.assertEqual(merged["trustedWorkspaces"], ["/custom/path"])
        self.assertEqual(merged["model"], "Custom Model")

    def test_merge_idempotence(self):
        existing = {
            "permissions": {
                "allow": ["command(ls)", "command(cat)"]
            }
        }
        standard = ["command(ls)", "command(cat)"]
        merged, added = agy_std.merge_permissions(existing, standard)

        self.assertEqual(added, [])
        self.assertEqual(len(merged["permissions"]["allow"]), 2)


class TestCLIOperations(unittest.TestCase):
    def setUp(self):
        self.test_dir = tempfile.mkdtemp(prefix="agy_test_")
        self.settings_file = Path(self.test_dir) / "settings.json"
        self.gemini_dir = Path(self.test_dir) / ".gemini"
        self.plugins_dir = self.gemini_dir / "config" / "plugins"
        self.mcp_file = self.gemini_dir / "config" / "mcp_config.json"

    def tearDown(self):
        shutil.rmtree(self.test_dir, ignore_errors=True)

    def test_audit_non_compliant(self):
        # Write incomplete settings
        with open(self.settings_file, "w") as f:
            json.dump({"permissions": {"allow": ["command(ls)"]}}, f)

        res = subprocess.run(
            BIN_CMD + [
                "audit",
                "--settings-path", str(self.settings_file),
                "--gemini-dir", str(self.gemini_dir),
                "--plugins-dir", str(self.plugins_dir),
                "--mcp-path", str(self.mcp_file),
            ],
            capture_output=True,
            text=True,
        )
        self.assertNotEqual(res.returncode, 0)
        self.assertIn("[NON-COMPLIANT]", res.stdout)

    def test_apply_and_audit_flow(self):
        # Initial empty settings
        with open(self.settings_file, "w") as f:
            json.dump({}, f)

        # Apply standards
        res_apply = subprocess.run(
            BIN_CMD + [
                "apply",
                "--settings-path", str(self.settings_file),
                "--gemini-dir", str(self.gemini_dir),
                "--plugins-dir", str(self.plugins_dir),
                "--mcp-path", str(self.mcp_file),
                "--no-backup",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_apply.returncode, 0, f"Apply failed: {res_apply.stderr}")

        # Verify file content
        with open(self.settings_file, "r") as f:
            saved = json.load(f)
        self.assertIn("permissions", saved)
        self.assertIn("command(git status)", saved["permissions"]["allow"])
        self.assertIn("command(pytest)", saved["permissions"]["allow"])
        self.assertTrue(saved.get("allowNonWorkspaceAccess"))

        # Verify plugin deployment
        deployed_plugin = self.plugins_dir / "autonomous-orchestrator"
        self.assertTrue(deployed_plugin.exists())
        self.assertTrue((deployed_plugin / "plugin.json").exists())
        self.assertTrue((deployed_plugin / "hooks.json").exists())
        guard_script = deployed_plugin / "scripts" / "guard-git-operations.sh"
        self.assertTrue(guard_script.exists())
        self.assertTrue(os.access(guard_script, os.X_OK))

    def test_apply_dry_run_does_not_modify_file(self):
        initial_data = {"permissions": {"allow": ["command(ls)"]}}
        with open(self.settings_file, "w") as f:
            json.dump(initial_data, f)

        res_dry = subprocess.run(
            BIN_CMD + ["apply", "--settings-path", str(self.settings_file), "--dry-run"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_dry.returncode, 0)
        self.assertIn("[DRY-RUN]", res_dry.stdout)

        with open(self.settings_file, "r") as f:
            current = json.load(f)
        self.assertEqual(current, initial_data)

    def test_diff_command(self):
        with open(self.settings_file, "w") as f:
            json.dump({"permissions": {"allow": ["command(my_unique_script.sh)"]}}, f)

        res_diff = subprocess.run(
            BIN_CMD + ["diff", "--settings-path", str(self.settings_file)],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_diff.returncode, 0)
        self.assertIn("Missing Standard Permissions", res_diff.stdout)
        self.assertIn("command(my_unique_script.sh)", res_diff.stdout)

    def test_snapshot_command(self):
        with open(self.settings_file, "w") as f:
            json.dump({"permissions": {"allow": ["command(custom_tool)", "command(ls)"]}}, f)

        res_snap = subprocess.run(
            BIN_CMD + ["snapshot", "--settings-path", str(self.settings_file), "--json"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_snap.returncode, 0)
        data = json.loads(res_snap.stdout)
        self.assertIn("command(custom_tool)", data["unmanaged_permissions"])
        self.assertNotIn("command(ls)", data["unmanaged_permissions"])

    def test_template_scaffolding(self):
        target_dir = Path(self.test_dir) / "scaffold_proj"
        res = subprocess.run(
            BIN_CMD + [
                "template",
                "--output", str(target_dir),
                "--name", "demo-service",
                "--lang", "Go",
                "--test", "go test ./...",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0)
        gemini_file = target_dir / "GEMINI.md"
        agents_file = target_dir / "AGENTS.md"

        self.assertTrue(gemini_file.exists())
        self.assertTrue(agents_file.exists())
        content = gemini_file.read_text()
        self.assertIn("demo-service", content)
        self.assertIn("Go", content)
        self.assertIn("go test ./...", content)


class TestConventionalCommits(unittest.TestCase):
    def test_valid_commit_messages(self):
        valid = [
            "feat: add permission catalog",
            "fix(cli): handle missing permissions key gracefully",
            "docs: update multi-machine replication guide",
            "style: format code according to PEP8",
            "refactor(core): extract permission set logic",
            "perf: optimize settings audit search",
            "test: add unit tests for diff command",
            "build: update python packaging metadata",
            "ci: add github actions test workflow",
            "chore: bump catalog version",
            "revert: revert previous commit",
            "feat(auth)!: breaking token renewal change",
        ]
        for msg in valid:
            with self.subTest(msg=msg):
                self.assertIsNotNone(re.match(COMMIT_PATTERN, msg), f"Should match: {msg}")

    def test_invalid_commit_messages(self):
        invalid = [
            "WIP",
            "Fixed bug",
            "added feature",
            "feat:no space after colon",
            "feature: wrong type name",
            "fix(): empty scope",
        ]
        for msg in invalid:
            with self.subTest(msg=msg):
                self.assertIsNone(re.match(COMMIT_PATTERN, msg), f"Should not match: {msg}")


class TestMCPManagement(unittest.TestCase):
    def test_mcp_catalog_validity(self):
        catalog = agy_std.get_standard_mcp_catalog()
        self.assertIn("mcpServers", catalog)
        self.assertIn("_requirements", catalog)
        self.assertIn("github", catalog["mcpServers"])
        self.assertIn("fetch", catalog["mcpServers"])
        self.assertIn("memory", catalog["mcpServers"])
        self.assertIn("repomix", catalog["mcpServers"])

    def test_mcp_merger_preserves_custom_servers(self):
        existing = {
            "mcpServers": {
                "custom-service": {"command": "custom-cmd"}
            }
        }
        catalog = {
            "mcpServers": {
                "github": {"command": "npx"}
            }
        }
        merged, added = agy_std.merge_mcp_servers(existing, catalog)
        self.assertEqual(added, ["github"])
        self.assertIn("custom-service", merged["mcpServers"])
        self.assertIn("github", merged["mcpServers"])

    def test_mcp_diagnostics_detection(self):
        catalog = {
            "mcpServers": {
                "test-server": {"command": "fake-bin-12345"}
            },
            "_requirements": {
                "test-server": {
                    "required_bins": ["nonexistent_binary_xyz_987"],
                    "required_env": ["NONEXISTENT_ENV_VAR_XYZ_987"],
                    "setup_hint": "Export var"
                }
            }
        }
        diagnostics = agy_std.audit_mcp_requirements(catalog)
        self.assertEqual(len(diagnostics), 1)
        self.assertIn("nonexistent_binary_xyz_987", diagnostics[0]["missing_bins"])
        self.assertIn("NONEXISTENT_ENV_VAR_XYZ_987", diagnostics[0]["missing_env"])
        self.assertFalse(diagnostics[0]["ready"])

    def test_is_env_var_configured(self):
        # 1. Detected from active os.environ
        test_var = "TEST_AGY_STD_VAR_123"
        os.environ[test_var] = "present"
        try:
            self.assertTrue(agy_std.is_env_var_configured(test_var))
        finally:
            del os.environ[test_var]

        # 2. Missing from environment and files
        self.assertFalse(agy_std.is_env_var_configured("NONEXISTENT_AGY_VAR_9999"))

        # 3. Detected from shell export in ~/.bashrc or ~/.profile
        with tempfile.TemporaryDirectory() as tmp_home:
            tmp_home_path = Path(tmp_home)
            bashrc = tmp_home_path / ".bashrc"
            bashrc.write_text("export TEST_SHELL_EXPORT_VAR=\"secret\"\n", encoding="utf-8")

            with unittest.mock.patch.object(agy_std.Path, "home", return_value=tmp_home_path):
                self.assertTrue(agy_std.is_env_var_configured("TEST_SHELL_EXPORT_VAR"))
                self.assertFalse(agy_std.is_env_var_configured("NONEXISTENT_VAR_IN_FILE"))


class TestAutonomousOrchestratorPlugin(unittest.TestCase):
    def setUp(self):
        self.plugin_dir = REPO_ROOT / "plugins" / "autonomous-orchestrator"
        self.guard_script = self.plugin_dir / "scripts" / "guard-git-operations.sh"
        self.verify_script = self.plugin_dir / "scripts" / "verify-clean-completion.sh"
        self.headroom_script = self.plugin_dir / "scripts" / "headroom-wrap.py"
        self.filter_wrap_script = self.plugin_dir / "scripts" / "filter-and-wrap.py"
        self.guard_delegation_script = self.plugin_dir / "scripts" / "guard-subagent-delegation.sh"

    def test_plugin_structure(self):
        self.assertTrue(self.plugin_dir.exists())
        self.assertTrue((self.plugin_dir / "plugin.json").exists())
        self.assertTrue((self.plugin_dir / "hooks.json").exists())
        self.assertTrue((self.plugin_dir / "rules" / "AGENTS.md").exists())
        self.assertTrue((self.plugin_dir / "skills" / "auto-lifecycle" / "SKILL.md").exists())
        self.assertTrue(self.guard_script.exists())
        self.assertTrue(self.verify_script.exists())
        self.assertTrue(self.headroom_script.exists())
        self.assertTrue(self.filter_wrap_script.exists())
        self.assertTrue(self.guard_delegation_script.exists())
        self.assertTrue(os.access(self.guard_script, os.X_OK))
        self.assertTrue(os.access(self.verify_script, os.X_OK))
        self.assertTrue(os.access(self.headroom_script, os.X_OK))
        self.assertTrue(os.access(self.filter_wrap_script, os.X_OK))
        self.assertTrue(os.access(self.guard_delegation_script, os.X_OK))

    def test_guard_git_operations_hook(self):
        # 1. Direct push to main is blocked
        res_push_main = subprocess.run(
            [str(self.guard_script), "--test", "git push origin main"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_push_main.returncode, 0)
        data = json.loads(res_push_main.stdout)
        self.assertEqual(data["decision"], "deny")
        self.assertIn("SECURITY VIOLATION", data["reason"])

        # 2. Force push is blocked
        res_force_push = subprocess.run(
            [str(self.guard_script), "--test", "git push origin --force feat/branch"],
            capture_output=True,
            text=True,
        )
        data = json.loads(res_force_push.stdout)
        self.assertEqual(data["decision"], "deny")

        # 3. Destructive deletion prompts user (decision: ask)
        res_rm = subprocess.run(
            [str(self.guard_script), "--test", "rm -rf /tmp/test-dir"],
            capture_output=True,
            text=True,
        )
        data = json.loads(res_rm.stdout)
        self.assertEqual(data["decision"], "ask")

        # 4. Safe git command allowed
        res_safe = subprocess.run(
            [str(self.guard_script), "--test", "git checkout -b feat/new-feature"],
            capture_output=True,
            text=True,
        )
        data = json.loads(res_safe.stdout)
        self.assertEqual(data["decision"], "allow")

    def test_verify_clean_completion_hook(self):
        # 1. Non-NO_TOOL_CALL exits cleanly
        res_cancel = subprocess.run(
            [str(self.verify_script)],
            input=json.dumps({"terminationReason": "USER_CANCELED"}),
            capture_output=True,
            text=True,
        )
        data = json.loads(res_cancel.stdout)
        self.assertEqual(data["decision"], "stop")

        # 2. Dry run check
        res_dry = subprocess.run(
            [str(self.verify_script), "--dry-run"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_dry.returncode, 0)

    def test_filter_and_wrap_hook(self):
        # 1. Verify --dry-run exits 0
        res_dry = subprocess.run(
            [str(self.filter_wrap_script), "--dry-run"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_dry.returncode, 0)

        # 2. Verify verbose commands return decision == "allow" and overwrite.CommandLine containing headroom-wrap.py --
        verbose_cmds = [
            "pytest tests -v",
            "git diff",
            "python3 -m unittest discover -s tests -v",
        ]
        for cmd in verbose_cmds:
            with self.subTest(cmd=cmd):
                res = subprocess.run(
                    [str(self.filter_wrap_script), "--test", cmd],
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(res.returncode, 0)
                data = json.loads(res.stdout)
                self.assertEqual(data.get("decision"), "allow")
                self.assertIn("overwrite", data)
                self.assertIn("headroom-wrap.py --", data["overwrite"].get("CommandLine", ""))

        # 3. Verify non-verbose or mutation commands return decision == "allow" with NO overwrite
        non_verbose_cmds = [
            "git status",
            "git checkout -b feat/x",
            'git commit -m "feat: x"',
            "ls -la",
        ]
        for cmd in non_verbose_cmds:
            with self.subTest(cmd=cmd):
                res = subprocess.run(
                    [str(self.filter_wrap_script), "--test", cmd],
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(res.returncode, 0)
                data = json.loads(res.stdout)
                self.assertEqual(data.get("decision"), "allow")
                self.assertNotIn("overwrite", data)

        # 4. Verify idempotence: command already containing headroom-wrap.py is not wrapped again
        already_wrapped = f"python3 {self.headroom_script} -- pytest tests -v"
        res_wrapped = subprocess.run(
            [str(self.filter_wrap_script), "--test", already_wrapped],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_wrapped.returncode, 0)
        data_wrapped = json.loads(res_wrapped.stdout)
        self.assertEqual(data_wrapped.get("decision"), "allow")
        self.assertNotIn("overwrite", data_wrapped)

        # 5. Verify daemon skip: JSON input with "IsDaemon": True is not wrapped
        daemon_payload = {
            "toolCall": {
                "name": "run_command",
                "args": {
                    "CommandLine": "pytest tests -v",
                    "IsDaemon": True,
                },
            }
        }
        res_daemon = subprocess.run(
            [str(self.filter_wrap_script)],
            input=json.dumps(daemon_payload),
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_daemon.returncode, 0)
        data_daemon = json.loads(res_daemon.stdout)
        self.assertEqual(data_daemon.get("decision"), "allow")
        self.assertNotIn("overwrite", data_daemon)

        # 6. Verify safety preservation: git push origin main returns decision == "deny" and rm -rf /tmp/x returns decision == "ask"
        res_push = subprocess.run(
            [str(self.filter_wrap_script), "--test", "git push origin main"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_push.returncode, 0)
        data_push = json.loads(res_push.stdout)
        self.assertEqual(data_push.get("decision"), "deny")

        res_rm = subprocess.run(
            [str(self.filter_wrap_script), "--test", "rm -rf /tmp/x"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_rm.returncode, 0)
        data_rm = json.loads(res_rm.stdout)
        self.assertEqual(data_rm.get("decision"), "ask")

    def test_headroom_wrap_execution_and_ccr(self):
        # 1. Verify --dry-run exits 0
        res_dry = subprocess.run(
            [str(self.headroom_script), "--dry-run"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_dry.returncode, 0)

        # 2. Verify short output (< 1500 chars) passes through unmodified with exit code 0
        res_short = subprocess.run(
            [str(self.headroom_script), "--", "echo", "antigravity test output"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_short.returncode, 0)
        self.assertEqual(res_short.stdout.strip(), "antigravity test output")
        self.assertNotIn("[Headroom CCR:", res_short.stdout)

        # 3. Verify non-zero exit code preservation
        res_err = subprocess.run(
            [str(self.headroom_script), "--", "ls", "/nonexistent_path_xyz_987"],
            capture_output=True,
            text=True,
        )
        self.assertNotEqual(res_err.returncode, 0)

        # 4. Verify large log output (> 2000 chars with a FAIL: test_important_case line in the middle)
        log_script = (
            "import sys\n"
            "lines = [f'test_{i:03d} (module.tests.StandardTestCase) ... ok [duration=0.012s]' for i in range(40)]\n"
            "lines.append('FAIL: test_important_case (module.tests.CriticalSuite)')\n"
            "lines.extend([f'test_{i:03d} (module.tests.StandardTestCase) ... ok [duration=0.012s]' for i in range(41, 85)])\n"
            "sys.stdout.write('\\n'.join(lines) + '\\n')\n"
        )
        res_log = subprocess.run(
            [str(self.headroom_script), "--", sys.executable, "-c", log_script],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_log.returncode, 0)
        self.assertIn("FAIL: test_important_case", res_log.stdout)
        self.assertIn("[Headroom CCR:", res_log.stdout)

        cache_match = re.search(r"Full output cached at (/[^\s\]]+\.txt)", res_log.stdout)
        self.assertIsNotNone(cache_match, "Breadcrumb should contain full cached file path")
        cached_file = Path(cache_match.group(1))
        self.assertTrue(cached_file.exists())
        cached_content = cached_file.read_text(encoding="utf-8")
        self.assertIn("FAIL: test_important_case", cached_content)
        self.assertGreater(len(cached_content), 2000)
        self.assertLess(len(res_log.stdout), len(cached_content))

        # 5. Verify large JSON array output (> 2000 chars) is compressed by JSON SmartCrusher and appends [Headroom CCR:
        json_script = (
            "import json, sys\n"
            "data = [{'index': i, 'name': f'node_{i}', 'payload': 'x' * 60} for i in range(40)]\n"
            "sys.stdout.write(json.dumps(data, indent=2) + '\\n')\n"
        )
        res_json = subprocess.run(
            [str(self.headroom_script), "--", sys.executable, "-c", json_script],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_json.returncode, 0)
        self.assertIn("[Headroom CCR:", res_json.stdout)
        self.assertIn("_headroom_omitted", res_json.stdout)

        # 6. Verify multi-hunk diff output (> 2000 chars) preserves headers while compressing hunk bodies
        diff_lines = [
            "diff --git a/foo.py b/foo.py",
            "index 1234567..89abcdef 100644",
            "--- a/foo.py",
            "+++ b/foo.py",
            "@@ -1,40 +1,40 @@ def func_one():",
        ]
        diff_lines.extend([f"+    line_hunk1_{i:03d} = 'some data payload {i:03d}'" for i in range(35)])
        diff_lines.append("@@ -50,40 +50,40 @@ def func_two():")
        diff_lines.extend([f"+    line_hunk2_{i:03d} = 'some data payload {i:03d}'" for i in range(35)])

        diff_script = (
            "import sys\n"
            f"lines = {repr(diff_lines)}\n"
            "sys.stdout.write('\\n'.join(lines) + '\\n')\n"
        )
        res_diff = subprocess.run(
            [str(self.headroom_script), "--", sys.executable, "-c", diff_script],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_diff.returncode, 0)
        self.assertIn("diff --git a/foo.py b/foo.py", res_diff.stdout)
        self.assertIn("@@ -1,40 +1,40 @@ def func_one():", res_diff.stdout)
        self.assertIn("@@ -50,40 +50,40 @@ def func_two():", res_diff.stdout)
        self.assertIn("hunk lines compressed by Headroom CCR", res_diff.stdout)
        self.assertIn("[Headroom CCR:", res_diff.stdout)

    def test_guard_subagent_delegation_hook(self):
        # 1. Verify --dry-run exits 0
        res_dry = subprocess.run(
            [str(self.guard_delegation_script), "--dry-run"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_dry.returncode, 0)

        # 2. Verify Flash subagent returns decision == "allow"
        flash_payload = {
            "toolCall": {
                "name": "edit_file",
                "args": {"Path": str(Path.home() / "Projects/repo/src/app.py")},
            },
            "modelName": "gemini-3.8-flash",
        }
        res_flash = subprocess.run(
            [str(self.guard_delegation_script)],
            input=json.dumps(flash_payload),
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_flash.returncode, 0)
        data_flash = json.loads(res_flash.stdout)
        self.assertEqual(data_flash.get("decision"), "allow")

        # 3. Verify brain artifact path returns decision == "allow"
        brain_payload = {
            "toolCall": {
                "name": "write_file",
                "args": {"Path": str(Path.home() / ".gemini/antigravity-cli/brain/123/plan.md")},
            },
            "modelName": "gemini-4-argon",
        }
        res_brain = subprocess.run(
            [str(self.guard_delegation_script)],
            input=json.dumps(brain_payload),
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_brain.returncode, 0)
        data_brain = json.loads(res_brain.stdout)
        self.assertEqual(data_brain.get("decision"), "allow")

        # 4. Verify direct root edit on project file returns decision == "deny"
        root_payload = {
            "toolCall": {
                "name": "edit_file",
                "args": {"Path": str(Path.home() / "Projects/repo/src/app.py")},
            },
            "modelName": "gemini-4-argon",
        }
        res_root = subprocess.run(
            [str(self.guard_delegation_script)],
            input=json.dumps(root_payload),
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_root.returncode, 0)
        data_root = json.loads(res_root.stdout)
        self.assertEqual(data_root.get("decision"), "deny")


if __name__ == "__main__":
    unittest.main()
