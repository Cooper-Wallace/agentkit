"""Unit tests for agentkit Obsidian vault CLI engine and integration workflows."""

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
CLI_PATH = REPO_ROOT / "bin" / "agentkit"


def run_cli(*args: str, env: dict = None) -> subprocess.CompletedProcess:
    """Execute agentkit CLI in a subprocess with optional environment variables."""
    full_env = os.environ.copy()
    # Default to an isolated test settings file unless specified
    if "AGENTKIT_SETTINGS_PATH" not in full_env:
        full_env["AGENTKIT_SETTINGS_PATH"] = str(Path(tempfile.gettempdir()) / "agentkit_test_vault_settings.json")
    if env:
        full_env.update(env)
    return subprocess.run(
        [sys.executable, str(CLI_PATH), *args],
        capture_output=True,
        text=True,
        timeout=60,
        env=full_env,
    )


class TestVaultPathResolution(unittest.TestCase):
    """Test resolution of vault path via flags, environment variables, and defaults."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_custom_path_flag_takes_precedence(self):
        custom_path = Path(self.temp_dir) / "CustomVault"
        env = {
            "AGENTKIT_VAULT_PATH": str(Path(self.temp_dir) / "EnvVault"),
            "AGENTKIT_AGENTVAULT_PATH": str(Path(self.temp_dir) / "AgentVaultEnvVault"),
            "OBSIDIAN_VAULT_PATH": str(Path(self.temp_dir) / "ObsidianEnvVault"),
        }
        res = run_cli("vault", "status", "--path", str(custom_path), "--json", env=env)
        # Vault doesn't exist yet, but status JSON reports resolved path
        data = json.loads(res.stdout)
        self.assertEqual(data["path"], str(custom_path.resolve()))

    def test_agentkit_vault_path_env_var(self):
        env_path = Path(self.temp_dir) / "AgentkitEnvVault"
        res = run_cli("vault", "status", "--json", env={"AGENTKIT_VAULT_PATH": str(env_path)})
        data = json.loads(res.stdout)
        self.assertEqual(data["path"], str(env_path.resolve()))

    def test_agentkit_agentvault_path_env_var(self):
        env_path = Path(self.temp_dir) / "AgentkitAgentVaultEnv"
        env = {"AGENTKIT_VAULT_PATH": "", "AGENTKIT_AGENTVAULT_PATH": str(env_path)}
        res = run_cli("vault", "status", "--json", env=env)
        data = json.loads(res.stdout)
        self.assertEqual(data["path"], str(env_path.resolve()))

    def test_obsidian_vault_path_env_var_fallback(self):
        env_path = Path(self.temp_dir) / "ObsidianEnvVault"
        env = {
            "AGENTKIT_VAULT_PATH": "",
            "AGENTKIT_AGENTVAULT_PATH": "",
            "OBSIDIAN_VAULT_PATH": str(env_path),
        }
        res = run_cli("vault", "status", "--json", env=env)
        data = json.loads(res.stdout)
        self.assertEqual(data["path"], str(env_path.resolve()))

    def test_default_path_resolution(self):
        env = {
            "AGENTKIT_VAULT_PATH": "",
            "AGENTKIT_AGENTVAULT_PATH": "",
            "OBSIDIAN_VAULT_PATH": "",
        }
        res = run_cli("vault", "status", "--json", env=env)
        data = json.loads(res.stdout)
        expected = (Path.home() / "Documents" / "AgentVault").resolve()
        self.assertEqual(data["path"], str(expected))


class TestVaultInit(unittest.TestCase):
    """Test `agentkit vault init` structure generation and frontmatter consistency."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.vault_path = Path(self.temp_dir) / "Vault"
        self.settings_path = Path(self.temp_dir) / "settings.json"

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_init_creates_complete_directory_tree_and_files(self):
        res = run_cli("vault", "init", "--path", str(self.vault_path), "--settings-path", str(self.settings_path))
        self.assertEqual(res.returncode, 0, res.stderr)
        self.assertIn("AgentVault initialized successfully", res.stdout)

        # 1. Directories
        self.assertTrue((self.vault_path / ".obsidian").is_dir())
        self.assertTrue((self.vault_path / "00-Inbox").is_dir())
        self.assertTrue((self.vault_path / "User-Input" / "pending").is_dir())
        self.assertTrue((self.vault_path / "User-Input" / "processed").is_dir())
        self.assertTrue((self.vault_path / "Projects").is_dir())
        self.assertTrue((self.vault_path / "Architecture" / "decisions").is_dir())
        self.assertTrue((self.vault_path / "Sessions").is_dir())

        # 2. Key notes
        expected_notes = [
            self.vault_path / "Dashboard.md",
            self.vault_path / "User-Input" / "README.md",
            self.vault_path / "Architecture" / "00-Overview.md",
            self.vault_path / "Architecture" / "contracts.md",
            self.vault_path / "Architecture" / "decisions" / "README.md",
            self.vault_path / "Sessions" / "README.md",
            self.vault_path / "Projects" / "agentkit.md",
            self.vault_path / "Projects" / "repokit.md",
            self.vault_path / "Projects" / "riced-kubuntu-os.md",
            self.vault_path / "Projects" / "workstation-setup.md",
            self.vault_path / "Projects" / "augurs-bazaar.md",
            self.vault_path / "Projects" / "augurs-saber.md",
        ]
        for note in expected_notes:
            self.assertTrue(note.is_file(), f"Expected note missing: {note}")

        # 3. Verify YAML frontmatter and Obsidian wikilinks/callouts in all notes
        for note in self.vault_path.rglob("*.md"):
            content = note.read_text(encoding="utf-8")
            self.assertTrue(
                content.startswith("---\n"),
                f"Note {note} missing YAML frontmatter opening",
            )
            self.assertIn("tags:", content, f"Note {note} missing tags")
            self.assertIn("type:", content, f"Note {note} missing type")
            self.assertIn("last_updated:", content, f"Note {note} missing last_updated")
            self.assertIn("updated_by: agentkit", content, f"Note {note} missing updated_by")
            # Verify callouts and wikilinks exist in major notes
            if note.name in ("Dashboard.md", "agentkit.md", "00-Overview.md"):
                self.assertRegex(content, r"> \[![a-zA-Z]+\]", f"Note {note} missing callout")
                self.assertIn("[[", content, f"Note {note} missing wikilinks")

        # 4. Verify trustedWorkspaces in settings
        self.assertTrue(self.settings_path.exists())
        settings_data = json.loads(self.settings_path.read_text(encoding="utf-8"))
        self.assertIn("trustedWorkspaces", settings_data)
        self.assertIn(str(self.vault_path.resolve()), settings_data["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "AgentVault"), settings_data["trustedWorkspaces"])

    def test_init_is_idempotent_and_preserves_customizations(self):
        # First init
        run_cli("vault", "init", "--path", str(self.vault_path), "--settings-path", str(self.settings_path))

        # Add custom line to agentkit.md
        agentkit_md = self.vault_path / "Projects" / "agentkit.md"
        custom_content = agentkit_md.read_text(encoding="utf-8") + "\n- [x] Custom completed task\n"
        agentkit_md.write_text(custom_content, encoding="utf-8")

        # Second init
        res = run_cli("vault", "init", "--path", str(self.vault_path), "--settings-path", str(self.settings_path))
        self.assertEqual(res.returncode, 0)

        # Custom line must be preserved
        persisted = agentkit_md.read_text(encoding="utf-8")
        self.assertIn("- [x] Custom completed task", persisted)

        # Settings trustedWorkspaces must not have duplicates
        settings_data = json.loads(self.settings_path.read_text(encoding="utf-8"))
        workspaces = settings_data.get("trustedWorkspaces", [])
        self.assertEqual(len(workspaces), len(set(workspaces)))
        self.assertIn(str(self.vault_path.resolve()), workspaces)
        self.assertIn(str(Path.home() / "AgentVault"), workspaces)

    def test_init_registers_trusted_workspaces_in_settings(self):
        custom_settings = Path(self.temp_dir) / "custom_settings.json"
        res = run_cli(
            "vault", "init",
            "--path", str(self.vault_path),
            "--settings-path", str(custom_settings),
        )
        self.assertEqual(res.returncode, 0, res.stderr)
        self.assertTrue(custom_settings.exists())
        data = json.loads(custom_settings.read_text(encoding="utf-8"))
        self.assertIn("trustedWorkspaces", data)
        self.assertIn(str(self.vault_path.resolve()), data["trustedWorkspaces"])
        self.assertIn(str(Path.home() / "AgentVault"), data["trustedWorkspaces"])


class TestVaultStatus(unittest.TestCase):
    """Test `agentkit vault status` reporting and note inventory."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.vault_path = Path(self.temp_dir) / "Vault"

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_status_uninitialized_vault(self):
        res = run_cli("vault", "status", "--path", str(self.vault_path))
        self.assertNotEqual(res.returncode, 0)
        self.assertIn("Obsidian AgentVault not found", res.stdout)

        res_json = run_cli("vault", "status", "--path", str(self.vault_path), "--json")
        self.assertNotEqual(res_json.returncode, 0)
        data = json.loads(res_json.stdout)
        self.assertFalse(data["exists"])
        self.assertEqual(data["error"], "Obsidian AgentVault does not exist or is not initialized.")

    def test_status_initialized_vault(self):
        run_cli("vault", "init", "--path", str(self.vault_path))

        # Human output
        res = run_cli("vault", "status", "--path", str(self.vault_path))
        self.assertEqual(res.returncode, 0)
        self.assertIn("Obsidian AgentVault Status", res.stdout)
        self.assertIn("Total Notes:", res.stdout)
        self.assertIn("Active Projects: 6", res.stdout)

        # JSON output
        res_json = run_cli("vault", "status", "--path", str(self.vault_path), "--json")
        self.assertEqual(res_json.returncode, 0)
        data = json.loads(res_json.stdout)
        self.assertTrue(data["exists"])
        self.assertGreaterEqual(data["total_notes"], 12)
        self.assertEqual(data["active_projects"], 6)
        self.assertEqual(data["pending_notes"], 0)
        self.assertIn("agentkit", data["projects"])
        self.assertIn("repokit", data["projects"])

    def test_status_detects_pending_notes(self):
        run_cli("vault", "init", "--path", str(self.vault_path))
        pending_file = self.vault_path / "User-Input" / "pending" / "feature-req.md"
        pending_file.write_text("# Feature Request\nAdd vault sync.\n", encoding="utf-8")

        res_json = run_cli("vault", "status", "--path", str(self.vault_path), "--json")
        data = json.loads(res_json.stdout)
        self.assertEqual(data["pending_notes"], 1)
        self.assertIn("feature-req.md", data["pending_files"])


class TestVaultIngest(unittest.TestCase):
    """Test `agentkit vault ingest` directive collection and processing."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.vault_path = Path(self.temp_dir) / "Vault"
        run_cli("vault", "init", "--path", str(self.vault_path))

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_ingest_pending_user_note(self):
        pending_dir = self.vault_path / "User-Input" / "pending"
        directive_file = pending_dir / "offline-task.md"
        directive_file.write_text(
            "---\ntags:\n  - directive\nproject: agentkit\n---\n# Offline Task\nImplement vault ingest.\n",
            encoding="utf-8",
        )

        res = run_cli("vault", "ingest", "--path", str(self.vault_path), "--json")
        self.assertEqual(res.returncode, 0, res.stderr)
        data = json.loads(res.stdout)
        self.assertEqual(data["ingested_count"], 1)
        item = data["items"][0]
        self.assertEqual(item["source"], "User-Input/pending/offline-task.md")
        self.assertEqual(item["project"], "agentkit")
        self.assertIn("Implement vault ingest.", item["content"])

        # Original note must be removed from pending
        self.assertFalse(directive_file.exists())

        # Processed note must exist in processed/ with audit stamp
        processed_dir = self.vault_path / "User-Input" / "processed"
        processed_files = list(processed_dir.glob("*offline-task.md"))
        self.assertEqual(len(processed_files), 1)
        processed_content = processed_files[0].read_text(encoding="utf-8")
        self.assertIn("> [!check] Ingested by agentkit on", processed_content)

    def test_ingest_from_project_note_section(self):
        agentkit_md = self.vault_path / "Projects" / "agentkit.md"
        content = agentkit_md.read_text(encoding="utf-8")
        # Add offline user input under the section
        notes_to_add = "\nPlease add automated backup rotation for vault notes.\n"
        updated_content = content + notes_to_add
        agentkit_md.write_text(updated_content, encoding="utf-8")

        res = run_cli(
            "vault",
            "ingest",
            "--path",
            str(self.vault_path),
            "--project",
            "agentkit",
            "--json",
        )
        self.assertEqual(res.returncode, 0)
        data = json.loads(res.stdout)
        self.assertEqual(data["ingested_count"], 1)
        self.assertEqual(data["items"][0]["source"], "Projects/agentkit.md")
        self.assertIn("automated backup rotation", data["items"][0]["content"])

        # Verify audit stamp in agentkit.md
        persisted = agentkit_md.read_text(encoding="utf-8")
        self.assertIn("> [!check] Ingested by agentkit on", persisted)

        # Re-running ingest should detect no new pending inputs
        res2 = run_cli(
            "vault",
            "ingest",
            "--path",
            str(self.vault_path),
            "--project",
            "agentkit",
            "--json",
        )
        data2 = json.loads(res2.stdout)
        self.assertEqual(data2["ingested_count"], 0)


class TestVaultSync(unittest.TestCase):
    """Test `agentkit vault sync` metadata and ADR synchronization."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.vault_path = Path(self.temp_dir) / "Vault"
        run_cli("vault", "init", "--path", str(self.vault_path))

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_sync_links_new_adrs(self):
        # Create a new ADR note in Architecture/decisions/
        adr_file = self.vault_path / "Architecture" / "decisions" / "0010-obsidian-vault-integration.md"
        adr_file.write_text(
            "---\ntags:\n  - architecture\n  - adr\ntype: reference\n---\n# ADR 0010\nObsidian Vault.",
            encoding="utf-8",
        )

        res = run_cli("vault", "sync", "--path", str(self.vault_path), "--project", "agentkit", "--json")
        self.assertEqual(res.returncode, 0, res.stderr)
        data = json.loads(res.stdout)
        self.assertEqual(data["synced"], ["agentkit"])

        # Check that agentkit.md now has a link to the new ADR
        agentkit_md = self.vault_path / "Projects" / "agentkit.md"
        content = agentkit_md.read_text(encoding="utf-8")
        self.assertIn("[[Architecture/decisions/0010-obsidian-vault-integration|0010-obsidian-vault-integration]]", content)


class TestVaultNote(unittest.TestCase):
    """Test `agentkit vault note` creation, wikilinks, and section appending."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.vault_path = Path(self.temp_dir) / "Vault"
        run_cli("vault", "init", "--path", str(self.vault_path))

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_create_structured_note(self):
        res = run_cli(
            "vault",
            "note",
            "--path",
            str(self.vault_path),
            "--project",
            "agentkit",
            "--title",
            "Subagent Memory Architecture",
            "--content",
            "Details on persistent cross-session knowledge graph.",
            "--tag",
            "architecture,memory",
            "--json",
        )
        self.assertEqual(res.returncode, 0, res.stderr)
        data = json.loads(res.stdout)
        self.assertTrue(data["success"])
        self.assertEqual(data["action"], "created")

        note_file = self.vault_path / "Projects" / "Subagent-Memory-Architecture.md"
        self.assertTrue(note_file.exists())
        content = note_file.read_text(encoding="utf-8")

        # Verify frontmatter
        self.assertIn("tags:", content)
        self.assertIn("agentkit", content)
        self.assertIn("memory", content)
        self.assertIn("architecture", content)
        self.assertIn("type: note", content)

        # Verify wikilinks and content
        self.assertIn("[[Projects/agentkit|agentkit]]", content)
        self.assertIn("Details on persistent cross-session knowledge graph.", content)

        # Verify backlink in Projects/agentkit.md
        agentkit_md = self.vault_path / "Projects" / "agentkit.md"
        agentkit_text = agentkit_md.read_text(encoding="utf-8")
        self.assertIn("[[Projects/Subagent-Memory-Architecture|Subagent Memory Architecture]]", agentkit_text)

    def test_append_to_existing_note(self):
        # Create note
        run_cli(
            "vault",
            "note",
            "--path",
            str(self.vault_path),
            "--project",
            "agentkit",
            "--title",
            "Subagent Memory Architecture",
            "--content",
            "Initial entry.",
        )
        # Append note
        res = run_cli(
            "vault",
            "note",
            "--path",
            str(self.vault_path),
            "--project",
            "agentkit",
            "--title",
            "Subagent Memory Architecture",
            "--content",
            "Follow-up entry.",
            "--json",
        )
        self.assertEqual(res.returncode, 0)
        data = json.loads(res.stdout)
        self.assertEqual(data["action"], "appended")

        note_file = self.vault_path / "Projects" / "Subagent-Memory-Architecture.md"
        content = note_file.read_text(encoding="utf-8")
        self.assertIn("Initial entry.", content)
        self.assertIn("Follow-up entry.", content)

    def test_append_section_to_project_note(self):
        res = run_cli(
            "vault",
            "note",
            "--path",
            str(self.vault_path),
            "--project",
            "agentkit",
            "--title",
            "Ignored for Section",
            "--section",
            "Sprint Retrospective",
            "--content",
            "Sprint 4 completed ahead of schedule.",
            "--json",
        )
        self.assertEqual(res.returncode, 0)
        data = json.loads(res.stdout)
        self.assertEqual(data["action"], "appended_section")

        agentkit_md = self.vault_path / "Projects" / "agentkit.md"
        content = agentkit_md.read_text(encoding="utf-8")
        self.assertIn("## Sprint Retrospective", content)
        self.assertIn("Sprint 4 completed ahead of schedule.", content)


if __name__ == "__main__":
    unittest.main()
