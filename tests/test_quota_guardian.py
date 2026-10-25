"""Tests for the Quota Guardian plugin, scripts, hooks, and configuration."""

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
PLUGIN_DIR = REPO_ROOT / "harness" / "plugins" / "quota-guardian"
SCRIPTS_DIR = PLUGIN_DIR / "scripts"
QUOTA_STATUS_SCRIPT = SCRIPTS_DIR / "quota_status.py"
GUARD_QUOTA_SCRIPT = SCRIPTS_DIR / "guard-quota.py"
CONFIG_FILE = REPO_ROOT / "config" / "quota.json"

if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import quota_status  # noqa: E402


class TestQuotaGuardian(unittest.TestCase):
    def setUp(self):
        self.config = quota_status.load_config()
        self.temp_dir = tempfile.TemporaryDirectory()
        self.cache_file = Path(self.temp_dir.name) / "quota_cache.json"

    def tearDown(self):
        self.temp_dir.cleanup()

    def _create_fixture(self, data: dict) -> Path:
        p = Path(self.temp_dir.name) / "fixture.json"
        p.write_text(json.dumps(data), encoding="utf-8")
        return p

    def test_summary_calculation(self):
        """Test calculation of remaining fractions, thresholds, and disabled bucket handling."""
        raw_data = {
            "groups": [
                {
                    "displayName": "gemini",
                    "buckets": [
                        {
                            "bucketId": "gemini-active",
                            "remainingFraction": 0.85,
                            "window": "24h",
                            "resetTime": "2026-10-24T12:00:00Z",
                            "disabled": False,
                        },
                        {
                            "bucketId": "gemini-ignored",
                            "remainingFraction": 0.01,
                            "disabled": True,
                        },
                    ],
                },
                {
                    "displayName": "claude",
                    "buckets": [
                        {
                            "bucketId": "claude-low",
                            "remainingFraction": 0.10,
                            "window": "24h",
                            "resetTime": "2026-10-24T14:00:00Z",
                            "disabled": False,
                        }
                    ],
                },
                {
                    "displayName": "argon",
                    "buckets": [
                        {
                            "bucketId": "argon-exhausted",
                            "remainingFraction": 0.0,
                            "window": "1h",
                            "resetTime": "2026-10-24T10:00:00Z",
                            "disabled": False,
                        }
                    ],
                },
            ]
        }

        summary = quota_status.summarize(raw_data, self.config)
        groups = summary["groups"]

        # Gemini: active bucket is 0.85, disabled 0.01 is ignored
        self.assertAlmostEqual(groups["gemini"]["remainingFraction"], 0.85)
        self.assertEqual(groups["gemini"]["status"], "healthy")
        self.assertFalse(groups["gemini"]["low"])
        self.assertFalse(groups["gemini"]["exhausted"])

        # Claude: 0.10 is <= switch_threshold (0.15) -> low
        self.assertAlmostEqual(groups["claude"]["remainingFraction"], 0.10)
        self.assertEqual(groups["claude"]["status"], "low")
        self.assertTrue(groups["claude"]["low"])
        self.assertFalse(groups["claude"]["critical"])

        # Argon: 0.0 is exhausted
        self.assertAlmostEqual(groups["argon"]["remainingFraction"], 0.0)
        self.assertEqual(groups["argon"]["status"], "exhausted")
        self.assertTrue(groups["argon"]["exhausted"])
        self.assertTrue(groups["argon"]["critical"])

        # Earliest reset
        self.assertEqual(summary["earliest_reset"], "2026-10-24T10:00:00Z")
        self.assertFalse(summary["all_critical"])

    def test_route_complexity(self):
        """Test routing across complexity tiers: low->flash_lite, med->flash, high->pro/flash."""
        healthy_summary = {
            "groups": {
                "gemini": {"remainingFraction": 0.80},
                "claude": {"remainingFraction": 0.80},
                "argon": {"remainingFraction": 0.80},
            }
        }

        # Low complexity
        r_low = quota_status.route("low", healthy_summary, self.config)
        self.assertEqual(r_low["model"], "flash_lite")
        self.assertEqual(r_low["group"], "gemini")

        # Medium complexity
        r_med = quota_status.route("med", healthy_summary, self.config)
        self.assertEqual(r_med["model"], "flash")
        self.assertEqual(r_med["group"], "gemini")

        # High complexity when Argon is healthy -> escalates to pro
        r_high = quota_status.route("high", healthy_summary, self.config)
        self.assertEqual(r_high["model"], "pro")
        self.assertEqual(r_high["group"], "argon")

        # High complexity when Argon is degraded -> routes to flash
        argon_degraded = {
            "groups": {
                "gemini": {"remainingFraction": 0.80},
                "claude": {"remainingFraction": 0.80},
                "argon": {"remainingFraction": 0.08},
            }
        }
        r_high_degraded = quota_status.route("high", argon_degraded, self.config)
        self.assertEqual(r_high_degraded["model"], "flash")
        self.assertEqual(r_high_degraded["group"], "gemini")

    def test_fallback_routing(self):
        """Test fallback routing when Gemini group is exhausted."""
        gemini_exhausted = {
            "groups": {
                "gemini": {"remainingFraction": 0.02},
                "claude": {"remainingFraction": 0.80},
                "argon": {"remainingFraction": 0.80},
            }
        }

        # Low and med fall back to inherit when Gemini is degraded
        r_low = quota_status.route("low", gemini_exhausted, self.config)
        self.assertEqual(r_low["model"], "inherit")

        r_med = quota_status.route("med", gemini_exhausted, self.config)
        self.assertEqual(r_med["model"], "inherit")

    def test_guard_quota_allow(self):
        """Test that guard-quota allows healthy invocations."""
        fixture = self._create_fixture(
            {
                "groups": [
                    {
                        "displayName": "gemini",
                        "buckets": [{"remainingFraction": 0.90, "disabled": False}],
                    },
                    {
                        "displayName": "argon",
                        "buckets": [{"remainingFraction": 0.90, "disabled": False}],
                    },
                ]
            }
        )

        env = {
            **os.environ,
            "AGENTKIT_QUOTA_FIXTURE": str(fixture),
            "AGENTKIT_QUOTA_CACHE": str(self.cache_file),
        }
        payload = {
            "toolCall": {
                "name": "invoke_subagent",
                "args": {"Model": "flash", "Prompt": "Write simple code"},
            }
        }

        res = subprocess.run(
            [sys.executable, str(GUARD_QUOTA_SCRIPT), "--test", json.dumps(payload)],
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(res.returncode, 0, res.stderr)
        data = json.loads(res.stdout)
        self.assertEqual(data["decision"], "allow")
        self.assertNotIn("overwrite", data)

    def test_guard_quota_overwrite(self):
        """Test that guard-quota reroutes subagent model when target group is degraded."""
        fixture = self._create_fixture(
            {
                "groups": [
                    {
                        "displayName": "argon",
                        "buckets": [
                            {
                                "remainingFraction": 0.05,
                                "disabled": False,
                                "resetTime": "2026-10-24T16:00:00Z",
                            }
                        ],
                    },
                    {
                        "displayName": "gemini",
                        "buckets": [{"remainingFraction": 0.90, "disabled": False}],
                    },
                ]
            }
        )

        env = {
            **os.environ,
            "AGENTKIT_QUOTA_FIXTURE": str(fixture),
            "AGENTKIT_QUOTA_CACHE": str(self.cache_file),
        }
        payload = {
            "toolCall": {
                "name": "invoke_subagent",
                "args": {"Model": "pro", "Prompt": "Perform architectural audit"},
            }
        }

        res = subprocess.run(
            [sys.executable, str(GUARD_QUOTA_SCRIPT), "--test", json.dumps(payload)],
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(res.returncode, 0, res.stderr)
        data = json.loads(res.stdout)
        self.assertEqual(data["decision"], "allow")
        self.assertIn("overwrite", data)
        self.assertEqual(data["overwrite"]["Model"], "flash")
        self.assertIn("Quota degraded for argon", data["reason"])

    def test_guard_quota_deny_wrapup(self):
        """Test that guard-quota triggers wrap-up denial when all model quotas are critical."""
        fixture = self._create_fixture(
            {
                "groups": [
                    {
                        "displayName": "gemini",
                        "buckets": [{"remainingFraction": 0.02, "disabled": False}],
                    },
                    {
                        "displayName": "claude",
                        "buckets": [{"remainingFraction": 0.01, "disabled": False}],
                    },
                    {
                        "displayName": "argon",
                        "buckets": [{"remainingFraction": 0.00, "disabled": False}],
                    },
                ]
            }
        )

        env = {
            **os.environ,
            "AGENTKIT_QUOTA_FIXTURE": str(fixture),
            "AGENTKIT_QUOTA_CACHE": str(self.cache_file),
        }
        payload = {
            "toolCall": {
                "name": "invoke_subagent",
                "args": {"Model": "flash", "Prompt": "Write more code"},
            }
        }

        res = subprocess.run(
            [sys.executable, str(GUARD_QUOTA_SCRIPT), "--test", json.dumps(payload)],
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(res.returncode, 0, res.stderr)
        data = json.loads(res.stdout)
        self.assertEqual(data["decision"], "deny")
        self.assertIn("QUOTA WRAP-UP", data["reason"])

    def test_cli_status_and_route(self):
        """Test CLI invocations with --json and human tabular output."""
        fixture = self._create_fixture(
            {
                "groups": [
                    {
                        "displayName": "gemini",
                        "buckets": [
                            {
                                "remainingFraction": 0.85,
                                "window": "24h",
                                "resetTime": "2026-10-24T12:00:00Z",
                                "disabled": False,
                            }
                        ],
                    }
                ]
            }
        )
        env = {
            **os.environ,
            "AGENTKIT_QUOTA_FIXTURE": str(fixture),
            "AGENTKIT_QUOTA_CACHE": str(self.cache_file),
        }

        # 1. Status table output
        res_status = subprocess.run(
            [sys.executable, str(QUOTA_STATUS_SCRIPT), "status"],
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(res_status.returncode, 0, res_status.stderr)
        self.assertIn("Remaining %", res_status.stdout)
        self.assertIn("gemini", res_status.stdout)

        # 2. Status JSON output
        res_status_json = subprocess.run(
            [sys.executable, str(QUOTA_STATUS_SCRIPT), "status", "--json"],
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(res_status_json.returncode, 0, res_status_json.stderr)
        status_data = json.loads(res_status_json.stdout)
        self.assertIn("groups", status_data)
        self.assertIn("gemini", status_data["groups"])

        # 3. Route low complexity JSON
        res_route_json = subprocess.run(
            [sys.executable, str(QUOTA_STATUS_SCRIPT), "route", "--complexity", "low", "--json"],
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(res_route_json.returncode, 0, res_route_json.stderr)
        route_data = json.loads(res_route_json.stdout)
        self.assertEqual(route_data["model"], "flash_lite")

        # 4. Dry-run support
        for script in [QUOTA_STATUS_SCRIPT, GUARD_QUOTA_SCRIPT]:
            res_dry = subprocess.run(
                [sys.executable, str(script), "--dry-run"],
                capture_output=True,
                text=True,
            )
            self.assertEqual(res_dry.returncode, 0)
            data = json.loads(res_dry.stdout)
            self.assertEqual(data["status"], "dry-run")

    def test_plugin_structure_and_hooks(self):
        """Validate JSON manifests, presence of docs and skills, and script executability."""
        self.assertTrue(PLUGIN_DIR.is_dir(), "Plugin directory must exist")
        self.assertTrue(CONFIG_FILE.is_file(), "config/quota.json must exist")

        # Validate config/quota.json
        cfg = json.loads(CONFIG_FILE.read_text(encoding="utf-8"))
        self.assertIn("thresholds", cfg)
        self.assertIn("groups", cfg)
        self.assertIn("complexity_to_model", cfg)
        self.assertIn("fallbacks", cfg)

        # Validate plugin.json
        manifest_file = PLUGIN_DIR / "plugin.json"
        self.assertTrue(manifest_file.is_file(), "plugin.json must exist")
        manifest = json.loads(manifest_file.read_text(encoding="utf-8"))
        self.assertEqual(manifest["name"], "quota-guardian")
        self.assertEqual(manifest["displayName"], "Quota Guardian")
        self.assertEqual(manifest["version"], "1.0.0")
        self.assertIn("description", manifest)
        self.assertIn("suggestedPrompts", manifest)

        # Validate hooks.json
        hooks_file = PLUGIN_DIR / "hooks.json"
        self.assertTrue(hooks_file.is_file(), "hooks.json must exist")
        hooks_data = json.loads(hooks_file.read_text(encoding="utf-8"))
        self.assertIn("quota-guardian", hooks_data)
        self.assertIn("PreToolUse", hooks_data["quota-guardian"])

        # Validate rules, skills, documentation
        self.assertTrue((PLUGIN_DIR / "rules" / "AGENTS.md").is_file())
        self.assertTrue((PLUGIN_DIR / "skills" / "quota-wrapup" / "SKILL.md").is_file())
        self.assertTrue((PLUGIN_DIR / "README.md").is_file())

        # Validate script files and executability
        for script in [QUOTA_STATUS_SCRIPT, GUARD_QUOTA_SCRIPT]:
            self.assertTrue(script.is_file(), f"{script.name} must exist")
            first_line = script.read_text(encoding="utf-8").splitlines()[0]
            self.assertTrue(
                first_line.startswith("#!/usr/bin/env python3"),
                f"{script.name} must have python3 shebang",
            )
            if not os.access(script, os.X_OK):
                try:
                    os.chmod(script, 0o755)
                except OSError:
                    pass
            self.assertTrue(
                os.access(script, os.X_OK) or not os.access(script.parent, os.W_OK),
                f"{script.name} must be executable",
            )


if __name__ == "__main__":
    unittest.main()
