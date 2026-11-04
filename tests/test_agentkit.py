"""Unit tests for agentkit Enterprise Verification and Git-Backed AgentVault Sync."""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
CLI_PATH = REPO_ROOT / "bin" / "agentkit"


def run_cli(*args: str, env: dict = None) -> subprocess.CompletedProcess:
    """Execute agentkit CLI in a subprocess with isolated environment."""
    full_env = os.environ.copy()
    if "AGENTKIT_SETTINGS_PATH" not in full_env:
        full_env["AGENTKIT_SETTINGS_PATH"] = str(Path(tempfile.gettempdir()) / "agentkit_test_settings.json")
    if env:
        full_env.update(env)
    return subprocess.run(
        [sys.executable, str(CLI_PATH), *args],
        capture_output=True,
        text=True,
        timeout=60,
        env=full_env,
    )


class TestAgentVaultEnterpriseSync(unittest.TestCase):
    """Test git-backed AgentVault init, sync dry-run, and graceful degrade paths."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.vault_path = Path(self.temp_dir) / "Vault"

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_vault_init_creates_gitignore_ignoring_workspace_json(self):
        """Verify vault init creates .gitignore containing .obsidian/workspace.json."""
        res = run_cli("vault", "init", "--path", str(self.vault_path))
        self.assertEqual(res.returncode, 0, res.stderr)

        gitignore = self.vault_path / ".gitignore"
        self.assertTrue(gitignore.is_file(), f".gitignore not found at {gitignore}")

        content = gitignore.read_text(encoding="utf-8")
        self.assertIn(".obsidian/workspace.json", content)
        self.assertIn(".obsidian/workspace-mobile.json", content)
        self.assertIn(".obsidian/cache", content)
        self.assertIn(".trash/", content)
        self.assertIn(".DS_Store", content)

    def test_vault_sync_dry_run_non_mutating(self):
        """Verify vault sync --dry-run previews actions without mutating files."""
        run_cli("vault", "init", "--path", str(self.vault_path))

        # Add a new ADR to trigger pending sync
        adr_file = self.vault_path / "Architecture" / "decisions" / "0099-test-adr.md"
        adr_file.write_text(
            "---\ntags:\n  - adr\ntype: reference\n---\n# ADR 0099\nTest ADR.\n",
            encoding="utf-8",
        )

        agentkit_md = self.vault_path / "Projects" / "agentkit.md"
        initial_mtime = agentkit_md.stat().st_mtime
        initial_content = agentkit_md.read_text(encoding="utf-8")

        # Run sync with --dry-run
        res = run_cli("vault", "sync", "--path", str(self.vault_path), "--project", "agentkit", "--dry-run", "--json")
        self.assertEqual(res.returncode, 0, res.stderr)

        data = json.loads(res.stdout)
        self.assertTrue(data.get("dry_run"))
        self.assertIn("agentkit", data.get("synced", []))

        # File content and mtime must remain unchanged
        self.assertEqual(agentkit_md.read_text(encoding="utf-8"), initial_content)
        self.assertEqual(agentkit_md.stat().st_mtime, initial_mtime)

        # Test dry-run with .git present
        subprocess.run(["git", "init"], cwd=self.vault_path, capture_output=True, check=True)
        res_git_dry = run_cli("vault", "sync", "--path", str(self.vault_path), "--dry-run", "--json")
        self.assertEqual(res_git_dry.returncode, 0)
        data_git_dry = json.loads(res_git_dry.stdout)
        self.assertEqual(data_git_dry.get("git_sync"), "dry-run")
        self.assertTrue(data_git_dry.get("dry_run"))

    def test_vault_sync_git_local_only_degrade(self):
        """Verify vault sync commits locally and gracefully degrades when no remote origin exists."""
        run_cli("vault", "init", "--path", str(self.vault_path))

        # Initialize git repo in the vault with no remote
        subprocess.run(["git", "init"], cwd=self.vault_path, capture_output=True, check=True)
        subprocess.run(["git", "config", "user.email", "agent@test.com"], cwd=self.vault_path, check=True)
        subprocess.run(["git", "config", "user.name", "Agent"], cwd=self.vault_path, check=True)
        subprocess.run(["git", "add", "-A"], cwd=self.vault_path, check=True)
        subprocess.run(["git", "commit", "-m", "chore: initial commit"], cwd=self.vault_path, check=True)

        # Create a new note in Sessions/
        session_note = self.vault_path / "Sessions" / "handoff-2026-11-03-test.md"
        session_note.write_text("# Session Handoff\nTest.\n", encoding="utf-8")

        # Verify no remote origin is configured
        remote_check = subprocess.run(["git", "remote"], cwd=self.vault_path, capture_output=True, text=True)
        self.assertNotIn("origin", remote_check.stdout)

        # Execute vault sync in JSON mode
        res = run_cli("vault", "sync", "--path", str(self.vault_path), "--json")
        self.assertEqual(res.returncode, 0, res.stderr)

        data = json.loads(res.stdout)
        self.assertFalse(data.get("git_remote_synced"))
        self.assertEqual(data.get("git_status"), "local_only")

        # Verify commit was created locally
        log_res = subprocess.run(
            ["git", "log", "-n", "1", "--oneline"],
            cwd=self.vault_path,
            capture_output=True,
            text=True,
            check=True,
        )
        self.assertIn("chore(vault): sync project state and session notes", log_res.stdout)


if __name__ == "__main__":
    unittest.main()
