"""Unit tests for agentkit autonomous-orchestrator lifecycle plugins and verification gates."""

import json
import os
import shutil
import stat
import subprocess
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
VERIFY_SCRIPT = REPO_ROOT / "harness" / "plugins" / "autonomous-orchestrator" / "scripts" / "verify-clean-completion.sh"


class TestVerifyCleanCompletionPlugin(unittest.TestCase):
    """Test verify-clean-completion.sh stop gate verification and resolution logic."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.repo_dir = Path(self.temp_dir) / "test-repo"
        self.repo_dir.mkdir(parents=True, exist_ok=True)

        # Initialize clean git repository
        subprocess.run(["git", "init"], cwd=self.repo_dir, capture_output=True, check=True)
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=self.repo_dir, check=True)
        subprocess.run(["git", "config", "user.name", "Test Agent"], cwd=self.repo_dir, check=True)
        (self.repo_dir / "README.md").write_text("# Test\n", encoding="utf-8")
        subprocess.run(["git", "add", "README.md"], cwd=self.repo_dir, check=True)
        subprocess.run(["git", "commit", "-m", "chore: initial commit"], cwd=self.repo_dir, check=True)

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_bash_test_failure_blocks_completion(self):
        """Verify failing bash test in tests/*.sh blocks completion with decision: continue."""
        tests_dir = self.repo_dir / "tests"
        tests_dir.mkdir(parents=True, exist_ok=True)
        failing_sh = tests_dir / "test_failure.sh"
        failing_sh.write_text("#!/usr/bin/env bash\nexit 1\n", encoding="utf-8")
        failing_sh.chmod(failing_sh.stat().st_mode | stat.S_IEXEC)

        subprocess.run(["git", "add", "tests/test_failure.sh"], cwd=self.repo_dir, check=True)
        subprocess.run(["git", "commit", "-m", "test: add failing bash test"], cwd=self.repo_dir, check=True)

        env = os.environ.copy()
        env["AGY_VERIFY_RUNNING"] = "0"
        payload = json.dumps({
            "terminationReason": "NO_TOOL_CALL",
            "workspacePaths": [str(self.repo_dir)],
        })

        res = subprocess.run(
            [str(VERIFY_SCRIPT)],
            input=payload,
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(res.returncode, 0, res.stderr)
        data = json.loads(res.stdout)
        self.assertEqual(data.get("decision"), "continue")
        self.assertIn("QUALITY GATE BLOCKED", data.get("reason", ""))
        self.assertIn("bash tests", data.get("reason", ""))

        # Guarded safe path: passing bash test allows clean stop
        failing_sh.write_text("#!/usr/bin/env bash\nexit 0\n", encoding="utf-8")
        subprocess.run(["git", "add", "tests/test_failure.sh"], cwd=self.repo_dir, check=True)
        subprocess.run(["git", "commit", "-m", "test: fix bash test"], cwd=self.repo_dir, check=True)

        res_pass = subprocess.run(
            [str(VERIFY_SCRIPT)],
            input=payload,
            capture_output=True,
            text=True,
            env=env,
        )
        data_pass = json.loads(res_pass.stdout)
        self.assertEqual(data_pass.get("decision"), "stop")

    def test_venv_bin_pytest_resolution(self):
        """Verify .venv/bin/pytest is preferred and executed when present."""
        venv_bin = self.repo_dir / ".venv" / "bin"
        venv_bin.mkdir(parents=True, exist_ok=True)

        marker_file = self.repo_dir / ".pytest_called"
        dummy_pytest = venv_bin / "pytest"
        dummy_pytest.write_text(
            f"#!/usr/bin/env bash\necho 'called' > '{marker_file}'\nexit 0\n",
            encoding="utf-8",
        )
        dummy_pytest.chmod(dummy_pytest.stat().st_mode | stat.S_IEXEC)

        tests_dir = self.repo_dir / "tests"
        tests_dir.mkdir(parents=True, exist_ok=True)
        (tests_dir / "test_sample.py").write_text("# python test placeholder\n", encoding="utf-8")

        subprocess.run(["git", "add", "."], cwd=self.repo_dir, check=True)
        subprocess.run(["git", "commit", "-m", "chore: setup test env"], cwd=self.repo_dir, check=True)

        env = os.environ.copy()
        env["AGY_VERIFY_RUNNING"] = "0"
        payload = json.dumps({
            "terminationReason": "NO_TOOL_CALL",
            "workspacePaths": [str(self.repo_dir)],
        })

        res = subprocess.run(
            [str(VERIFY_SCRIPT)],
            input=payload,
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(res.returncode, 0, res.stderr)
        data = json.loads(res.stdout)
        self.assertEqual(data.get("decision"), "stop")
        self.assertTrue(marker_file.exists(), ".venv/bin/pytest was not executed")

        # Guarded failure check: failing .venv/bin/pytest blocks completion
        marker_file.unlink()
        dummy_pytest.write_text(
            f"#!/usr/bin/env bash\necho 'called again' > '{marker_file}'\nexit 1\n",
            encoding="utf-8",
        )
        subprocess.run(["git", "add", "."], cwd=self.repo_dir, check=True)
        subprocess.run(["git", "commit", "-m", "chore: fail pytest"], cwd=self.repo_dir, check=True)

        res_fail = subprocess.run(
            [str(VERIFY_SCRIPT)],
            input=payload,
            capture_output=True,
            text=True,
            env=env,
        )
        data_fail = json.loads(res_fail.stdout)
        self.assertEqual(data_fail.get("decision"), "continue")
        self.assertIn("pytest", data_fail.get("reason", ""))
        self.assertTrue(marker_file.exists())


if __name__ == "__main__":
    unittest.main()
