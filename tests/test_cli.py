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


class TestAutonomousOrchestratorPlugin(unittest.TestCase):
    def setUp(self):
        self.plugin_dir = REPO_ROOT / "plugins" / "autonomous-orchestrator"
        self.guard_script = self.plugin_dir / "scripts" / "guard-git-operations.sh"
        self.verify_script = self.plugin_dir / "scripts" / "verify-clean-completion.sh"

    def test_plugin_structure(self):
        self.assertTrue(self.plugin_dir.exists())
        self.assertTrue((self.plugin_dir / "plugin.json").exists())
        self.assertTrue((self.plugin_dir / "hooks.json").exists())
        self.assertTrue((self.plugin_dir / "rules" / "AGENTS.md").exists())
        self.assertTrue((self.plugin_dir / "skills" / "auto-lifecycle" / "SKILL.md").exists())
        self.assertTrue(self.guard_script.exists())
        self.assertTrue(self.verify_script.exists())
        self.assertTrue(os.access(self.guard_script, os.X_OK))
        self.assertTrue(os.access(self.verify_script, os.X_OK))

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


if __name__ == "__main__":
    unittest.main()
