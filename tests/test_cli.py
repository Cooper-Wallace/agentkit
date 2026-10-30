"""
Tests for agentkit CLI tool, harness abstraction, and standardization engine.
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

# Add bin directory and load agentkit module
REPO_ROOT = Path(__file__).resolve().parent.parent
BIN_PATH = REPO_ROOT / "bin" / "agentkit"

loader = importlib.machinery.SourceFileLoader("agentkit", str(BIN_PATH))
spec = importlib.util.spec_from_loader("agentkit", loader)
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

        # Test that config/permissions.json contains mcp_tools and tools
        self.assertIn("mcp_tools", catalog)
        self.assertGreater(len(catalog["mcp_tools"]), 0)
        for mcp_name, mcp_data in catalog["mcp_tools"].items():
            self.assertIn("permissions", mcp_data, f"MCP category {mcp_name} missing 'permissions'")
            self.assertGreater(len(mcp_data["permissions"]), 0, f"MCP category {mcp_name} has empty permissions")

        self.assertIn("tools", catalog)
        self.assertGreater(len(catalog["tools"]), 0)
        for tool_name, tool_data in catalog["tools"].items():
            self.assertIn("permissions", tool_data, f"Tool category {tool_name} missing 'permissions'")
            self.assertGreater(len(tool_data["permissions"]), 0, f"Tool category {tool_name} has empty permissions")

        # Test that config/permissions.json contains file_access
        self.assertIn("file_access", catalog)
        self.assertGreater(len(catalog["file_access"]), 0)
        for fa_name, fa_data in catalog["file_access"].items():
            self.assertIn("permissions", fa_data, f"File access category {fa_name} missing 'permissions'")
            self.assertGreater(len(fa_data["permissions"]), 0, f"File access category {fa_name} has empty permissions")

    test_standard_permissions_catalog_validity = test_permissions_json_validity

    def test_flat_permissions_generation(self):
        flat = agy_std.get_flat_standard_permissions()
        self.assertGreater(len(flat), 20)

        command_perms = [p for p in flat if p.startswith("command(")]
        mcp_perms = [p for p in flat if p.startswith("mcp(")]
        file_perms = [p for p in flat if p.startswith("read_file(") or p.startswith("write_file(")]
        tool_perms = [
            p for p in flat
            if not p.startswith("command(") and not p.startswith("mcp(") and not p.startswith("read_file(") and not p.startswith("write_file(")
        ]

        # Test that get_flat_standard_permissions() contains command(...), mcp(...), tool, and file access permissions
        self.assertGreater(len(command_perms), 0, "Expected command(...) permissions in flat list")
        self.assertGreater(len(mcp_perms), 0, "Expected mcp(...) permissions in flat list")
        self.assertGreater(len(tool_perms), 0, "Expected tool permissions in flat list")
        self.assertGreater(len(file_perms), 0, "Expected read_file/write_file permissions in flat list")

        # Specifically assert that mcp(memory) permissions are removed
        self.assertNotIn("mcp(memory)", flat)
        self.assertNotIn("mcp(memory/create_entities)", flat)
        self.assertNotIn("mcp(memory/add_observations)", flat)

        # Assert active standard MCP permissions are present
        self.assertIn("mcp(repomix)", flat)
        self.assertIn("mcp(github)", flat)
        self.assertIn("mcp(fetch)", flat)

        # Specifically assert that call_mcp_tool is present
        self.assertIn("call_mcp_tool", flat)

        # Specifically assert that native file access grants are present
        self.assertIn(f"read_file({Path.home() / 'Documents' / 'AgentVault'})", flat)
        self.assertIn(f"write_file({Path.home() / 'Documents' / 'AgentVault'})", flat)
        self.assertIn(f"read_file({Path.home() / 'Projects' / 'agentkit'})", flat)
        self.assertIn(f"write_file({Path.home() / 'Projects' / 'agentkit'})", flat)
        self.assertIn(f"read_file({Path.home() / 'Projects' / 'repokit'})", flat)
        self.assertIn(f"write_file({Path.home() / 'Projects' / 'repokit'})", flat)

        # Validate formatting of each item
        for perm in flat:
            if perm.startswith("command("):
                self.assertTrue(perm.endswith(")"), f"Invalid command format: {perm}")
            elif perm.startswith("mcp("):
                self.assertTrue(perm.endswith(")"), f"Invalid mcp format: {perm}")
            elif perm.startswith("read_file(") or perm.startswith("write_file("):
                self.assertTrue(perm.endswith(")"), f"Invalid file access format: {perm}")
            else:
                self.assertTrue(
                    perm in {"call_mcp_tool", "list_resources", "read_resource"} or perm.startswith("call_mcp_tool("),
                    f"Unexpected tool permission: {perm}"
                )

    def test_gh_pr_permissions_and_no_merge(self):
        catalog = agy_std.get_standard_permissions_catalog()
        gh_gov_commands = catalog["categories"]["github_governance"]["commands"]
        self.assertNotIn("gh pr", gh_gov_commands)
        self.assertNotIn("gh pr merge", gh_gov_commands)

        safe_gh_pr = [
            "gh pr create",
            "gh pr list",
            "gh pr view",
            "gh pr status",
            "gh pr checks",
            "gh pr diff",
            "gh pr comment",
        ]
        for cmd in safe_gh_pr:
            self.assertIn(cmd, gh_gov_commands)

        flat = agy_std.get_flat_standard_permissions()
        self.assertNotIn("command(gh pr)", flat)
        self.assertNotIn("command(gh pr merge)", flat)
        for cmd in safe_gh_pr:
            self.assertIn(f"command({cmd})", flat)

    def test_settings_default_file_exists(self):
        default_settings = agy_std.load_json(agy_std.SETTINGS_DEFAULT_FILE)
        self.assertIn("allowNonWorkspaceAccess", default_settings)
        self.assertIn("colorScheme", default_settings)
        self.assertIn("model", default_settings)
        self.assertIn("trustedWorkspaces", default_settings)
        self.assertEqual(default_settings["trustedWorkspaces"], [])


class TestTemplatesAndArchetypes(unittest.TestCase):
    def test_hooks_template_validity(self):
        hooks_file = REPO_ROOT / "harness" / "hooks" / "hooks.json"
        self.assertTrue(hooks_file.exists())
        data = json.loads(hooks_file.read_text(encoding="utf-8"))
        self.assertIn("command-safety-guard", data)
        self.assertIn("code-linter-formatter", data)
        self.assertFalse(data["command-safety-guard"]["enabled"])

    def test_plugins_json_template_validity(self):
        plugins_file = REPO_ROOT / "project" / "templates" / "plugins.json"
        self.assertTrue(plugins_file.exists())
        data = json.loads(plugins_file.read_text(encoding="utf-8"))
        self.assertIn("entries", data)
        self.assertIsInstance(data["entries"], list)

    def test_plugin_archetype_validity(self):
        plugin_dir = REPO_ROOT / "project" / "templates" / "plugin"
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

    def test_human_only_trunk_promotion_directives(self):
        directive_files = [
            REPO_ROOT / "AGENTS.md",
            REPO_ROOT / "harness" / "rules" / "AGENTS.md",
            REPO_ROOT / "project" / "templates" / "AGENTS.md",
        ]
        phrase = "Human-Only Trunk Promotion & Auto-Merge Prohibition"
        for path in directive_files:
            content = path.read_text(encoding="utf-8")
            self.assertIn(phrase, content, f"Missing directive in {path}")
            self.assertIn("MUST NEVER execute `gh pr merge`", content, f"Missing merge prohibition in {path}")

        skill_file = REPO_ROOT / "harness" / "plugins" / "autonomous-orchestrator" / "skills" / "auto-lifecycle" / "SKILL.md"
        skill_content = skill_file.read_text(encoding="utf-8")
        self.assertIn("Human-Only Trunk Promotion Gate", skill_content)
        self.assertIn("Autonomous agents MUST NEVER execute `gh pr merge`", skill_content)


class TestBinaryExecutable(unittest.TestCase):
    def test_binary_executable(self):
        bin_dir = REPO_ROOT / "bin"
        primary = bin_dir / "agentkit"
        self.assertTrue(primary.exists(), "agentkit binary must exist")
        self.assertTrue(os.access(primary, os.X_OK), "agentkit must be executable")

        # Assert that legacy binaries and symlinks do NOT exist
        for name in ["ak", "agy-standardise", "antigravity-standardise"]:
            with self.subTest(legacy=name):
                self.assertFalse((REPO_ROOT / "bin" / name).exists(), f"Legacy binary {name} should not exist")


class TestOrchestrationProfiles(unittest.TestCase):
    def setUp(self):
        self.test_dir = tempfile.mkdtemp(prefix="agentkit_orch_test_")
        self.settings_file = Path(self.test_dir) / "settings.json"
        self.gemini_dir = Path(self.test_dir) / ".gemini"
        self.plugins_dir = self.gemini_dir / "config" / "plugins"
        self.mcp_file = self.gemini_dir / "config" / "mcp_config.json"
        self.orch_file = self.gemini_dir / "config" / "orchestration.json"

    def tearDown(self):
        shutil.rmtree(self.test_dir, ignore_errors=True)

    def test_orchestration_json_catalog_validity(self):
        orch_path = REPO_ROOT / "config" / "orchestration.json"
        self.assertTrue(orch_path.exists(), "config/orchestration.json must exist")
        data = json.loads(orch_path.read_text(encoding="utf-8"))

        self.assertIn("default_profile", data)
        self.assertEqual(data["default_profile"], "argon-flash")
        self.assertIn("profiles", data)

        required_profiles = ["argon-flash", "opus-flash", "sonnet-flash", "flash-high"]
        for prof in required_profiles:
            self.assertIn(prof, data["profiles"], f"Missing profile {prof}")
            prof_data = data["profiles"][prof]
            self.assertIn("root_model", prof_data, f"Profile {prof} missing 'root_model'")
            self.assertTrue(prof_data["root_model"], f"Profile {prof} 'root_model' is empty")
            self.assertIn("worker_model", prof_data, f"Profile {prof} missing 'worker_model'")
            self.assertTrue(prof_data["worker_model"], f"Profile {prof} 'worker_model' is empty")

    def test_apply_records_orchestration_profile(self):
        with open(self.settings_file, "w") as f:
            json.dump({}, f)

        res = subprocess.run(
            BIN_CMD + [
                "apply",
                "--settings-path", str(self.settings_file),
                "--gemini-dir", str(self.gemini_dir),
                "--plugins-dir", str(self.plugins_dir),
                "--mcp-path", str(self.mcp_file),
                "--orchestration-path", str(self.orch_file),
                "--model-profile", "opus-flash",
                "--no-backup",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0, f"Apply failed: {res.stderr}")

        # Verify profile is recorded in settings.json
        with open(self.settings_file, "r") as f:
            settings_data = json.load(f)
        self.assertEqual(settings_data.get("orchestrationProfile"), "opus-flash")
        self.assertIn("orchestration", settings_data)
        self.assertEqual(settings_data["orchestration"].get("profile"), "opus-flash")

        # Verify profile is recorded in orchestration.json file
        self.assertTrue(self.orch_file.exists(), f"Orchestration file must exist: {self.orch_file}")
        with open(self.orch_file, "r") as f:
            orch_data = json.load(f)
        self.assertEqual(orch_data.get("active_profile"), "opus-flash")

    def test_audit_reports_active_orchestration_profile(self):
        with open(self.settings_file, "w") as f:
            json.dump({
                "permissions": {"allow": agy_std.get_flat_standard_permissions()},
                "allowNonWorkspaceAccess": True,
                "colorScheme": "dark",
                "orchestrationProfile": "opus-flash",
            }, f)

        res = subprocess.run(
            BIN_CMD + [
                "audit",
                "--settings-path", str(self.settings_file),
                "--gemini-dir", str(self.gemini_dir),
                "--plugins-dir", str(self.plugins_dir),
                "--mcp-path", str(self.mcp_file),
                "--orchestration-path", str(self.orch_file),
                "--no-mcp",
                "--no-plugins",
            ],
            capture_output=True,
            text=True,
        )
        self.assertIn("Orchestration Profile: 'opus-flash' active", res.stdout)


class TestHarnessAbstraction(unittest.TestCase):
    def test_harness_registry_contains_antigravity(self):
        self.assertIn("antigravity", agy_std.SUPPORTED_HARNESSES)
        adapter = agy_std.get_harness_adapter("antigravity")
        self.assertIsInstance(adapter, agy_std.AntigravityHarnessAdapter)
        self.assertEqual(adapter.name, "antigravity")

        # Check expected paths
        settings_path = adapter.get_default_settings_path()
        self.assertEqual(settings_path.name, "settings.json")
        self.assertIn(".gemini", str(settings_path))

        mcp_path = adapter.get_default_mcp_path()
        self.assertEqual(mcp_path.name, "mcp_config.json")
        self.assertIn(".gemini", str(mcp_path))

        plugins_dir = adapter.get_default_plugins_dir()
        self.assertEqual(plugins_dir.name, "plugins")
        self.assertIn(".gemini", str(plugins_dir))

        orch_path = adapter.get_default_orchestration_path()
        self.assertEqual(orch_path.name, "orchestration.json")

        directive_files = adapter.get_directive_filenames()
        self.assertEqual(directive_files, ["AGENTS.md", "GEMINI.md"])

    def test_harness_default_is_antigravity(self):
        default_adapter = agy_std.get_harness_adapter()
        self.assertEqual(default_adapter.name, "antigravity")

        parser = agy_std.build_parser()
        args = parser.parse_args(["audit"])
        self.assertEqual(args.harness, "antigravity")

    def test_cli_explicit_harness_flag(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            settings_file = Path(tmp_dir) / "settings.json"
            settings_file.write_text("{}", encoding="utf-8")
            res = subprocess.run(
                BIN_CMD + ["--harness", "antigravity", "diff", "--settings-path", str(settings_file), "--no-mcp"],
                capture_output=True,
                text=True,
            )
            self.assertEqual(res.returncode, 0)


EXPECTED_FILE_PERMS = [
    f"read_file({Path.home() / 'Documents' / 'AgentVault'})",
    f"write_file({Path.home() / 'Documents' / 'AgentVault'})",
    f"read_file({Path.home() / 'AgentVault'})",
    f"write_file({Path.home() / 'AgentVault'})",
    f"read_file({Path.home() / 'Projects' / 'agentkit'})",
    f"write_file({Path.home() / 'Projects' / 'agentkit'})",
    f"read_file({Path.home() / 'Projects' / 'repokit'})",
    f"write_file({Path.home() / 'Projects' / 'repokit'})",
]


class TestPermissionMerger(unittest.TestCase):
    def test_merge_into_empty_settings(self):
        standard = ["command(ls)", "command(cat)"]
        default_settings = {"colorScheme": "dark"}
        merged, added = agy_std.merge_permissions({}, standard, default_settings)

        self.assertEqual(merged["colorScheme"], "dark")
        self.assertEqual(added, ["command(ls)", "command(cat)"] + EXPECTED_FILE_PERMS)
        self.assertEqual(merged["permissions"]["allow"], ["command(ls)", "command(cat)"] + EXPECTED_FILE_PERMS)
        expected_workspaces = [
            str(Path.home() / "Documents" / "AgentVault"),
            str(Path.home() / "AgentVault"),
            str(Path.home() / "Projects" / "agentkit"),
            str(Path.home() / "Projects" / "repokit"),
        ]
        self.assertEqual(merged["trustedWorkspaces"], expected_workspaces)

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

        self.assertEqual(added, ["command(cat)", "command(grep)"] + EXPECTED_FILE_PERMS)
        self.assertIn("command(custom_script.sh)", merged["permissions"]["allow"])
        self.assertIn("command(ls)", merged["permissions"]["allow"])
        self.assertIn("command(cat)", merged["permissions"]["allow"])
        self.assertIn("command(grep)", merged["permissions"]["allow"])
        for perm in EXPECTED_FILE_PERMS:
            self.assertIn(perm, merged["permissions"]["allow"])
        self.assertIn("/custom/path", merged["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "Documents" / "AgentVault"), merged["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "AgentVault"), merged["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "Projects" / "agentkit"), merged["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "Projects" / "repokit"), merged["trustedWorkspaces"])
        self.assertEqual(merged["model"], "Custom Model")

    def test_merge_includes_agentkit_vault_path_env(self):
        standard = ["command(ls)"]
        custom_vault = "/tmp/custom_agentkit_vault"
        with unittest.mock.patch.dict(os.environ, {"AGENTKIT_VAULT_PATH": custom_vault}):
            merged, _ = agy_std.merge_permissions({}, standard)
            resolved_vault = str(Path(custom_vault).expanduser().resolve())
            self.assertIn(resolved_vault, merged["trustedWorkspaces"])
            self.assertIn(f"read_file({resolved_vault})", merged["permissions"]["allow"])
            self.assertIn(f"write_file({resolved_vault})", merged["permissions"]["allow"])
            self.assertIn(str(Path.home() / "Documents" / "AgentVault"), merged["trustedWorkspaces"])
            self.assertIn(str(Path.home() / "AgentVault"), merged["trustedWorkspaces"])
            self.assertIn(str(Path.home() / "Projects" / "agentkit"), merged["trustedWorkspaces"])
            self.assertIn(str(Path.home() / "Projects" / "repokit"), merged["trustedWorkspaces"])

    def test_merge_includes_agentvault_path_env(self):
        standard = ["command(ls)"]
        custom_vault = "/tmp/custom_agentvault_path"
        with unittest.mock.patch.dict(os.environ, {"AGENTKIT_AGENTVAULT_PATH": custom_vault}):
            merged, _ = agy_std.merge_permissions({}, standard)
            resolved_vault = str(Path(custom_vault).expanduser().resolve())
            self.assertIn(resolved_vault, merged["trustedWorkspaces"])
            self.assertIn(f"read_file({resolved_vault})", merged["permissions"]["allow"])
            self.assertIn(f"write_file({resolved_vault})", merged["permissions"]["allow"])
            self.assertIn(str(Path.home() / "Documents" / "AgentVault"), merged["trustedWorkspaces"])
            self.assertIn(str(Path.home() / "AgentVault"), merged["trustedWorkspaces"])
            self.assertIn(str(Path.home() / "Projects" / "agentkit"), merged["trustedWorkspaces"])
            self.assertIn(str(Path.home() / "Projects" / "repokit"), merged["trustedWorkspaces"])

    def test_merge_trusted_workspaces_no_duplicates(self):
        existing = {
            "trustedWorkspaces": [
                str(Path.home() / "Documents" / "AgentVault"),
                str(Path.home() / "AgentVault"),
                str(Path.home() / "Projects" / "agentkit"),
                str(Path.home() / "Projects" / "repokit"),
            ]
        }
        merged, _ = agy_std.merge_permissions(existing, [])
        self.assertEqual(
            merged["trustedWorkspaces"],
            [
                str(Path.home() / "Documents" / "AgentVault"),
                str(Path.home() / "AgentVault"),
                str(Path.home() / "Projects" / "agentkit"),
                str(Path.home() / "Projects" / "repokit"),
            ]
        )

    def test_merge_includes_agentkit_and_repokit_workspaces(self):
        merged, _ = agy_std.merge_permissions({}, [])
        self.assertIn(str(Path.home() / "Projects" / "agentkit"), merged["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "Projects" / "repokit"), merged["trustedWorkspaces"])

    def test_merge_idempotence(self):
        existing = {
            "permissions": {
                "allow": ["command(ls)", "command(cat)"] + EXPECTED_FILE_PERMS
            }
        }
        standard = ["command(ls)", "command(cat)"]
        merged, added = agy_std.merge_permissions(existing, standard)

        self.assertEqual(added, [])
        self.assertEqual(len(merged["permissions"]["allow"]), 2 + len(EXPECTED_FILE_PERMS))

    def test_merge_mixed_command_mcp_and_tool_permissions(self):
        existing = {
            "permissions": {
                "allow": ["command(git status)", "mcp(repomix/read_repomix_output)"]
            }
        }
        standard = [
            "command(git status)",
            "command(pytest)",
            "mcp(repomix)",
            "mcp(repomix/pack_codebase)",
            "call_mcp_tool",
            "list_resources",
        ]
        merged, added = agy_std.merge_permissions(existing, standard)

        expected_added = [
            "command(pytest)",
            "mcp(repomix)",
            "mcp(repomix/pack_codebase)",
            "call_mcp_tool",
            "list_resources",
        ] + EXPECTED_FILE_PERMS
        self.assertEqual(added, expected_added)
        self.assertIn("command(git status)", merged["permissions"]["allow"])
        self.assertIn("command(pytest)", merged["permissions"]["allow"])
        self.assertIn("mcp(repomix/read_repomix_output)", merged["permissions"]["allow"])
        self.assertIn("mcp(repomix)", merged["permissions"]["allow"])
        self.assertIn("mcp(repomix/pack_codebase)", merged["permissions"]["allow"])
        self.assertIn("call_mcp_tool", merged["permissions"]["allow"])
        self.assertIn("list_resources", merged["permissions"]["allow"])
        for perm in EXPECTED_FILE_PERMS:
            self.assertIn(perm, merged["permissions"]["allow"])

    def test_merge_native_file_access_grants(self):
        merged, added = agy_std.merge_permissions({}, [])
        for perm in EXPECTED_FILE_PERMS:
            self.assertIn(perm, merged["permissions"]["allow"])
            self.assertIn(perm, added)


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
        self.assertIn("Trusted Workspaces: Required workspace path(s) missing from trustedWorkspaces", res.stdout)
        self.assertIn("AgentVault", res.stdout)
        self.assertIn("agentkit", res.stdout)
        self.assertIn("repokit", res.stdout)

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
        self.assertIn(f"read_file({Path.home() / 'Documents' / 'AgentVault'})", saved["permissions"]["allow"])
        self.assertIn(f"write_file({Path.home() / 'Documents' / 'AgentVault'})", saved["permissions"]["allow"])
        self.assertIn(f"read_file({Path.home() / 'Projects' / 'agentkit'})", saved["permissions"]["allow"])
        self.assertIn(f"write_file({Path.home() / 'Projects' / 'agentkit'})", saved["permissions"]["allow"])
        self.assertIn(f"read_file({Path.home() / 'Projects' / 'repokit'})", saved["permissions"]["allow"])
        self.assertIn(f"write_file({Path.home() / 'Projects' / 'repokit'})", saved["permissions"]["allow"])
        self.assertTrue(saved.get("allowNonWorkspaceAccess"))
        self.assertIn("trustedWorkspaces", saved)
        self.assertIn(str(Path.home() / "Documents" / "AgentVault"), saved["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "AgentVault"), saved["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "Projects" / "agentkit"), saved["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "Projects" / "repokit"), saved["trustedWorkspaces"])

        # Verify plugin deployment
        deployed_plugin = self.plugins_dir / "autonomous-orchestrator"
        self.assertTrue(deployed_plugin.exists())
        self.assertTrue((deployed_plugin / "plugin.json").exists())
        self.assertTrue((deployed_plugin / "hooks.json").exists())
        guard_script = deployed_plugin / "scripts" / "guard-git-operations.sh"
        self.assertTrue(guard_script.exists())
        self.assertTrue(os.access(guard_script, os.X_OK))

        # Run audit on applied settings and verify compliance
        res_audit = subprocess.run(
            BIN_CMD + [
                "audit",
                "--settings-path", str(self.settings_file),
                "--gemini-dir", str(self.gemini_dir),
                "--plugins-dir", str(self.plugins_dir),
                "--mcp-path", str(self.mcp_file),
                "--no-mcp",
                "--no-plugins",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_audit.returncode, 0, f"Audit failed: {res_audit.stderr}")
        self.assertIn("Trusted Workspaces: AgentVault, agentkit, and repokit whitelisted", res_audit.stdout)
        self.assertIn("Native File Access: AgentVault, agentkit, and repokit read permissions active", res_audit.stdout)

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
        agents_file = target_dir / "AGENTS.md"
        gemini_file = target_dir / "GEMINI.md"

        self.assertTrue(agents_file.exists())
        self.assertFalse(agents_file.is_symlink())
        self.assertTrue(gemini_file.exists())
        self.assertTrue(gemini_file.is_symlink())
        self.assertEqual(os.readlink(gemini_file), "AGENTS.md")

        content = agents_file.read_text(encoding="utf-8")
        self.assertIn("demo-service", content)
        self.assertIn("Go", content)
        self.assertIn("go test ./...", content)


class TestTemplateScaffolding(unittest.TestCase):
    def setUp(self):
        self.test_dir = tempfile.mkdtemp(prefix="agentkit_scaffold_test_")

    def tearDown(self):
        shutil.rmtree(self.test_dir, ignore_errors=True)

    def test_template_scaffolds_agents_primary_and_gemini_symlink(self):
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
        self.assertEqual(res.returncode, 0, f"Template command failed: {res.stderr}")
        agents_file = target_dir / "AGENTS.md"
        gemini_file = target_dir / "GEMINI.md"

        self.assertTrue(agents_file.exists(), "AGENTS.md must exist")
        self.assertFalse(agents_file.is_symlink(), "AGENTS.md must be a primary regular file, not a symlink")

        self.assertTrue(gemini_file.exists(), "GEMINI.md must exist")
        self.assertTrue(gemini_file.is_symlink(), "GEMINI.md must be a symlink pointing to AGENTS.md")
        self.assertEqual(os.readlink(gemini_file), "AGENTS.md")

        content = agents_file.read_text(encoding="utf-8")
        self.assertIn("demo-service", content)
        self.assertIn("Go", content)
        self.assertIn("go test ./...", content)

    def test_template_legacy_flag_scaffolds_gemini_primary(self):
        target_dir = Path(self.test_dir) / "scaffold_legacy_proj"
        res = subprocess.run(
            BIN_CMD + [
                "template",
                "--output", str(target_dir),
                "--name", "legacy-service",
                "--legacy",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0)

        agents_file = target_dir / "AGENTS.md"
        gemini_file = target_dir / "GEMINI.md"

        self.assertTrue(gemini_file.exists())
        self.assertFalse(gemini_file.is_symlink())
        self.assertTrue(agents_file.exists())
        self.assertTrue(agents_file.is_symlink())
        self.assertEqual(os.readlink(agents_file), "GEMINI.md")


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
        self.assertIn("repomix", catalog["mcpServers"])
        self.assertNotIn("memory", catalog["mcpServers"])
        self.assertNotIn("memory", catalog.get("_requirements", {}))
        self.assertNotIn("sqlite", catalog["mcpServers"])
        self.assertNotIn("sqlite", catalog.get("_requirements", {}))
        self.assertEqual(len(catalog["mcpServers"]), 3)

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

    def test_mcp_merger_resolves_server_vars_with_default(self):
        catalog = {
            "mcpServers": {
                "custom-service": {
                    "command": "test-service",
                    "args": ["--path", "${TEST_DATA_DIR:-$HOME/data}"]
                }
            }
        }
        with unittest.mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("TEST_DATA_DIR", None)
            merged, added = agy_std.merge_mcp_servers({}, catalog)
            self.assertIn("custom-service", merged["mcpServers"])
            srv = merged["mcpServers"]["custom-service"]
            expected_path = str(Path.home() / "data")
            self.assertEqual(srv["args"], ["--path", expected_path])

    def test_mcp_merger_resolves_server_vars_with_env(self):
        catalog = {
            "mcpServers": {
                "custom-service": {
                    "command": "test-service",
                    "args": ["--path", "${TEST_DATA_DIR:-$HOME/data}"]
                }
            }
        }
        custom_path = "/tmp/custom_data_dir"
        with unittest.mock.patch.dict(os.environ, {"TEST_DATA_DIR": custom_path}):
            merged, added = agy_std.merge_mcp_servers({}, catalog)
            self.assertIn("custom-service", merged["mcpServers"])
            srv = merged["mcpServers"]["custom-service"]
            self.assertEqual(srv["args"], ["--path", custom_path])

    def test_mcp_merger_heals_unresolved_placeholders(self):
        catalog = {
            "mcpServers": {
                "custom-service": {
                    "command": "custom-cmd",
                    "args": ["--dir", "${CUSTOM_DIR:-$HOME/data}"]
                }
            }
        }
        existing = {
            "mcpServers": {
                "custom-service": {
                    "command": "custom-cmd",
                    "args": ["--dir", "${CUSTOM_DIR:-$HOME/data}"]
                }
            }
        }
        with unittest.mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("CUSTOM_DIR", None)
            merged, added = agy_std.merge_mcp_servers(existing, catalog)
            self.assertNotIn("custom-service", added)
            expected_dir = str(Path.home() / "data")
            self.assertEqual(merged["mcpServers"]["custom-service"]["args"], ["--dir", expected_dir])

    def test_resolve_mcp_server_vars(self):
        # 1. Default fallback syntax ${VAR:-default}
        srv_default = {"command": "test", "args": ["--mode", "${TEST_MODE:-production}"]}
        with unittest.mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("TEST_MODE", None)
            res = agy_std.resolve_mcp_server_vars(srv_default)
            self.assertEqual(res["args"], ["--mode", "production"])

        # 2. Env value used for ${VAR:-default} when set
        with unittest.mock.patch.dict(os.environ, {"TEST_MODE": "staging"}):
            res = agy_std.resolve_mcp_server_vars(srv_default)
            self.assertEqual(res["args"], ["--mode", "staging"])

        # 3. Path with $HOME expansion in default
        srv_home = {"command": "test", "args": ["--path", "${DATA_DIR:-$HOME/store}"]}
        with unittest.mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("DATA_DIR", None)
            res = agy_std.resolve_mcp_server_vars(srv_home)
            self.assertEqual(res["args"], ["--path", str(Path.home() / "store")])

        # 4. Bash wrapper with safe fallback expansion
        srv_bash = {
            "command": "bash",
            "args": ["-c", 'exec my-server --path "${DATA_PATH:-$HOME/data.db}"']
        }
        with unittest.mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("DATA_PATH", None)
            res = agy_std.resolve_mcp_server_vars(srv_bash)
            expected_arg = f'exec my-server --path "{Path.home() / "data.db"}"'
            self.assertEqual(res["args"], ["-c", expected_arg])

        with unittest.mock.patch.dict(os.environ, {"DATA_PATH": "/tmp/custom.db"}):
            res = agy_std.resolve_mcp_server_vars(srv_bash)
            self.assertEqual(res["args"], ["-c", 'exec my-server --path "/tmp/custom.db"'])

        # 5. Unknown var without default is preserved
        srv_preserve = {"command": "test", "args": ["--token", "${UNKNOWN_VAR}"]}
        with unittest.mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("UNKNOWN_VAR", None)
            res = agy_std.resolve_mcp_server_vars(srv_preserve)
            self.assertEqual(res["args"], ["--token", "${UNKNOWN_VAR}"])

        # 6. Optional env dict resolution
        srv_env = {"command": "test", "args": [], "env": {"VAR": "${CUSTOM_ENV:-fallback}"}}
        res_no_env = agy_std.resolve_mcp_server_vars(srv_env, resolve_env=False)
        self.assertEqual(res_no_env["env"]["VAR"], "${CUSTOM_ENV:-fallback}")
        res_with_env = agy_std.resolve_mcp_server_vars(srv_env, resolve_env=True)
        self.assertEqual(res_with_env["env"]["VAR"], "fallback")

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
        self.plugin_dir = REPO_ROOT / "harness" / "plugins" / "autonomous-orchestrator"
        self.guard_script = self.plugin_dir / "scripts" / "guard-git-operations.sh"
        self.verify_script = self.plugin_dir / "scripts" / "verify-clean-completion.sh"
        self.headroom_script = self.plugin_dir / "scripts" / "headroom-wrap.py"
        self.filter_wrap_script = self.plugin_dir / "scripts" / "filter-and-wrap.py"
        self.guard_delegation_script = self.plugin_dir / "scripts" / "guard-subagent-delegation.sh"
        self.compliance_script = self.plugin_dir / "scripts" / "guard-agentkit-compliance.sh"

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
        self.assertTrue(self.compliance_script.exists())
        self.assertTrue(os.access(self.guard_script, os.X_OK))
        self.assertTrue(os.access(self.verify_script, os.X_OK))
        self.assertTrue(os.access(self.headroom_script, os.X_OK))
        self.assertTrue(os.access(self.filter_wrap_script, os.X_OK))
        self.assertTrue(os.access(self.guard_delegation_script, os.X_OK))
        self.assertTrue(os.access(self.compliance_script, os.X_OK))

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
            env=dict(os.environ, AGY_VERIFY_RUNNING="0"),
            cwd=tempfile.gettempdir(),
            capture_output=True,
            text=True,
        )
        data = json.loads(res_cancel.stdout)
        self.assertEqual(data["decision"], "stop")

        # 2. Dry run check
        res_dry = subprocess.run(
            [str(self.verify_script), "--dry-run"],
            env=dict(os.environ, AGY_VERIFY_RUNNING="0"),
            cwd=tempfile.gettempdir(),
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_dry.returncode, 0)
        data_dry = json.loads(res_dry.stdout)
        self.assertEqual(data_dry.get("status"), "dry-run")
        self.assertEqual(data_dry.get("decision"), "stop")

        # 3. Recursion circuit breaker check
        env_recursion = os.environ.copy()
        env_recursion["AGY_VERIFY_RUNNING"] = "1"
        res_recursion = subprocess.run(
            [str(self.verify_script)],
            env=env_recursion,
            cwd=tempfile.gettempdir(),
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_recursion.returncode, 0)
        data_recursion = json.loads(res_recursion.stdout)
        self.assertEqual(data_recursion.get("decision"), "stop")
        self.assertIn("Recursion circuit breaker", data_recursion.get("reason", ""))

        # 4. Clean git repository resolution via workspacePaths from /tmp
        with tempfile.TemporaryDirectory() as clean_repo:
            subprocess.run(["git", "init"], cwd=clean_repo, capture_output=True, check=True)
            subprocess.run(["git", "config", "user.email", "test@test.com"], cwd=clean_repo, check=True)
            subprocess.run(["git", "config", "user.name", "Test"], cwd=clean_repo, check=True)
            (Path(clean_repo) / "README.md").write_text("# Test\n")
            subprocess.run(["git", "add", "README.md"], cwd=clean_repo, check=True)
            subprocess.run(["git", "commit", "-m", "chore: init"], cwd=clean_repo, check=True)

            res_ws = subprocess.run(
                [str(self.verify_script)],
                input=json.dumps({
                    "terminationReason": "NO_TOOL_CALL",
                    "workspacePaths": [clean_repo],
                }),
                env=dict(os.environ, AGY_VERIFY_RUNNING="0"),
                cwd=tempfile.gettempdir(),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_ws.returncode, 0)
            data_ws = json.loads(res_ws.stdout)
            self.assertEqual(data_ws.get("decision"), "stop")

        # 5. Read-only turn gate allows stop even if repo has untracked files
        with tempfile.TemporaryDirectory() as dirty_repo:
            subprocess.run(["git", "init"], cwd=dirty_repo, capture_output=True, check=True)
            (Path(dirty_repo) / "dirty.txt").write_text("uncommitted")

            with tempfile.NamedTemporaryFile("w", delete=False) as tf:
                tf.write(json.dumps({"type": "USER_INPUT", "content": "What does file.txt say?"}) + "\n")
                tf.write(json.dumps({
                    "type": "PLANNER_RESPONSE",
                    "tool_calls": [{"name": "read_file", "args": {"Path": "file.txt"}}],
                }) + "\n")
                transcript_ro = tf.name

            try:
                res_ro = subprocess.run(
                    [str(self.verify_script)],
                    input=json.dumps({
                        "terminationReason": "NO_TOOL_CALL",
                        "workspacePaths": [dirty_repo],
                        "transcriptPath": transcript_ro,
                    }),
                    env=dict(os.environ, AGY_VERIFY_RUNNING="0"),
                    cwd=dirty_repo,
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(res_ro.returncode, 0)
                data_ro = json.loads(res_ro.stdout)
                self.assertEqual(data_ro.get("decision"), "stop")
            finally:
                os.unlink(transcript_ro)

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

        # 5. Verify root session with "Fix the agentkit plugins" in USER_INPUT is STILL denied
        with tempfile.NamedTemporaryFile("w", delete=False) as tf:
            tf.write(json.dumps({"type": "USER_INPUT", "content": "Fix the agentkit plugins"}) + "\n")
            transcript_agentkit = tf.name

        try:
            payload_agentkit_regex = {
                "toolCall": {
                    "name": "edit_file",
                    "args": {"Path": str(Path.home() / "Projects/repo/src/app.py")},
                },
                "modelName": "gemini-4-argon",
                "transcriptPath": transcript_agentkit,
            }
            res_regex = subprocess.run(
                [str(self.guard_delegation_script)],
                input=json.dumps(payload_agentkit_regex),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_regex.returncode, 0)
            data_regex = json.loads(res_regex.stdout)
            self.assertEqual(data_regex.get("decision"), "deny")
        finally:
            os.unlink(transcript_agentkit)

        # 6. Verify root session with invoke_subagent tool call in same turn is STILL denied
        with tempfile.NamedTemporaryFile("w", delete=False) as tf:
            tf.write(json.dumps({"type": "USER_INPUT", "content": "Implement feature X"}) + "\n")
            tf.write(json.dumps({"type": "PLANNER_RESPONSE", "tool_calls": [{"name": "invoke_subagent"}]}) + "\n")
            transcript_delegated = tf.name

        try:
            payload_post_delegation = {
                "toolCall": {
                    "name": "edit_file",
                    "args": {"Path": str(Path.home() / "Projects/repo/src/app.py")},
                },
                "modelName": "gemini-4-argon",
                "transcriptPath": transcript_delegated,
            }
            res_post_del = subprocess.run(
                [str(self.guard_delegation_script)],
                input=json.dumps(payload_post_delegation),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_post_del.returncode, 0)
            data_post_del = json.loads(res_post_del.stdout)
            self.assertEqual(data_post_del.get("decision"), "deny")
        finally:
            os.unlink(transcript_delegated)

        # 7. Verify root session editing file in Obsidian AgentVault is allowed
        vault_payload = {
            "toolCall": {
                "name": "edit_file",
                "args": {"Path": str(Path.home() / "Documents/AgentVault/Sessions/handoff.md")},
            },
            "modelName": "gemini-4-argon",
        }
        res_vault = subprocess.run(
            [str(self.guard_delegation_script)],
            input=json.dumps(vault_payload),
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_vault.returncode, 0)
        data_vault = json.loads(res_vault.stdout)
        self.assertEqual(data_vault.get("decision"), "allow")

    def test_guard_agentkit_compliance_hook(self):
        # 1. Verify --dry-run exits 0 with operational message
        res_dry = subprocess.run(
            [str(self.compliance_script), "--dry-run"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_dry.returncode, 0)
        data_dry = json.loads(res_dry.stdout)
        self.assertEqual(data_dry.get("status"), "dry-run")
        self.assertIn("operational", data_dry.get("message", ""))

        # 2. Verify AGENTKIT_SKIP_SESSION_AUDIT=1 bypasses audit and returns {}
        res_skip = subprocess.run(
            [str(self.compliance_script), "--test"],
            env=dict(os.environ, AGENTKIT_SKIP_SESSION_AUDIT="1"),
            capture_output=True,
            text=True,
        )
        self.assertEqual(res_skip.returncode, 0)
        self.assertEqual(json.loads(res_skip.stdout), {})

        # 3. Behavior when audit succeeds (mocked exit 0 returns {})
        with tempfile.TemporaryDirectory() as tmpdir:
            mock_bin = Path(tmpdir) / "agentkit"
            mock_bin.write_text("#!/bin/sh\nexit 0\n")
            mock_bin.chmod(0o755)

            res_ok = subprocess.run(
                [str(self.compliance_script), "--test"],
                env=dict(
                    os.environ,
                    AGENTKIT_BIN=str(mock_bin),
                    AGENTKIT_SKIP_SESSION_AUDIT="0",
                ),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_ok.returncode, 0)
            self.assertEqual(json.loads(res_ok.stdout), {})

        # 4. Behavior when audit fails without auto-apply (returns injectSteps alerting user)
        with tempfile.TemporaryDirectory() as tmpdir:
            mock_bin = Path(tmpdir) / "agentkit"
            mock_bin.write_text("#!/bin/sh\nexit 1\n")
            mock_bin.chmod(0o755)

            res_fail = subprocess.run(
                [str(self.compliance_script), "--test"],
                env=dict(
                    os.environ,
                    AGENTKIT_BIN=str(mock_bin),
                    AGENTKIT_SKIP_SESSION_AUDIT="0",
                    AGENTKIT_AUTO_APPLY="0",
                ),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_fail.returncode, 0)
            data_fail = json.loads(res_fail.stdout)
            self.assertIn("injectSteps", data_fail)
            self.assertEqual(len(data_fail["injectSteps"]), 1)
            msg = data_fail["injectSteps"][0]["systemMessage"]["systemMessage"]
            self.assertIn("out of sync", msg)
            self.assertIn("agentkit apply", msg)

        # 5. Behavior when audit fails with AGENTKIT_AUTO_APPLY=1 (triggers apply and returns auto-heal notice)
        with tempfile.TemporaryDirectory() as tmpdir:
            mock_bin = Path(tmpdir) / "agentkit"
            marker_file = Path(tmpdir) / "applied.marker"
            mock_bin.write_text(f"""#!/bin/sh
if [ "$1" = "audit" ]; then
    exit 1
elif [ "$1" = "apply" ]; then
    touch "{marker_file}"
    exit 0
fi
exit 0
""")
            mock_bin.chmod(0o755)

            res_auto = subprocess.run(
                [str(self.compliance_script), "--test"],
                env=dict(
                    os.environ,
                    AGENTKIT_BIN=str(mock_bin),
                    AGENTKIT_SKIP_SESSION_AUDIT="0",
                    AGENTKIT_AUTO_APPLY="1",
                ),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_auto.returncode, 0)
            self.assertTrue(marker_file.exists(), "agentkit apply should have been called")
            data_auto = json.loads(res_auto.stdout)
            self.assertIn("injectSteps", data_auto)
            self.assertEqual(len(data_auto["injectSteps"]), 1)
            msg = data_auto["injectSteps"][0]["systemMessage"]["systemMessage"]
            self.assertIn("auto-heal", msg)
            self.assertIn("automatically synchronized", msg)

        # 6. Fallback when agentkit binary is not found (returns {} and exits 0)
        with tempfile.TemporaryDirectory() as tmpdir:
            isolated_script = Path(tmpdir) / "guard.sh"
            shutil.copy2(self.compliance_script, isolated_script)
            isolated_script.chmod(0o755)
            res_nobin = subprocess.run(
                [str(isolated_script), "--test"],
                env=dict(
                    os.environ,
                    AGENTKIT_BIN="",
                    PATH="/usr/bin:/bin",
                    HOME=tmpdir,
                    AGENTKIT_SKIP_SESSION_AUDIT="0",
                ),
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_nobin.returncode, 0)
            self.assertEqual(json.loads(res_nobin.stdout), {})



class TestSkillManagement(unittest.TestCase):
    def test_skill_list(self):
        res = subprocess.run(
            BIN_CMD + ["skill", "list"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0, f"skill list failed: {res.stderr}")
        self.assertIn("timeseries-ml", res.stdout)
        self.assertIn("feature-engineering", res.stdout)

    def test_skill_info(self):
        res = subprocess.run(
            BIN_CMD + ["skill", "info", "timeseries-ml"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0, f"skill info failed: {res.stderr}")
        self.assertIn("timeseries-ml", res.stdout)

    def test_skill_install_symlink(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target_dir = Path(tmpdir) / "timeseries-ml"
            res = subprocess.run(
                BIN_CMD + ["skill", "install", "timeseries-ml", "--target", str(target_dir)],
                capture_output=True,
                text=True,
            )
            self.assertEqual(res.returncode, 0, f"skill install symlink failed: {res.stderr}")
            self.assertTrue((target_dir / "SKILL.md").exists())

    def test_skill_install_copy(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target_dir = Path(tmpdir) / "feature-engineering"
            res = subprocess.run(
                BIN_CMD + ["skill", "install", "feature-engineering", "--copy", "--target", str(target_dir)],
                capture_output=True,
                text=True,
            )
            self.assertEqual(res.returncode, 0, f"skill install copy failed: {res.stderr}")
            self.assertTrue((target_dir / "SKILL.md").exists())
            self.assertFalse((target_dir / "SKILL.md").is_symlink())

    def test_apply_creates_global_skills_registry(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            mock_gemini_dir = Path(tmpdir) / "gemini"
            mock_settings = Path(tmpdir) / "settings.json"
            mock_settings.write_text("{}", encoding="utf-8")

            res = subprocess.run(
                BIN_CMD + [
                    "apply",
                    "--settings-path", str(mock_settings),
                    "--gemini-dir", str(mock_gemini_dir),
                    "--no-mcp",
                    "--no-plugins",
                    "--no-backup",
                ],
                capture_output=True,
                text=True,
            )
            self.assertEqual(res.returncode, 0, f"apply failed: {res.stderr}")
            skills_json_path = mock_gemini_dir / "config" / "skills.json"
            self.assertTrue(skills_json_path.exists(), f"Expected {skills_json_path} to exist")

            data = json.loads(skills_json_path.read_text(encoding="utf-8"))
            self.assertIn("entries", data)
            entry_paths = [e.get("path") for e in data["entries"] if isinstance(e, dict)]
            expected_path = str((REPO_ROOT / "project" / "skills").resolve())
            self.assertIn(expected_path, entry_paths)


class TestToolManagement(unittest.TestCase):
    def test_tool_list(self):
        res = subprocess.run(
            BIN_CMD + ["tool", "list"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(res.returncode, 0, f"tool list failed: {res.stderr}")
        self.assertIn("cv_splitter.py", res.stdout)


if __name__ == "__main__":
    unittest.main()
