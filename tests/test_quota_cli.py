"""Tests for quota-aware profiles and `agentkit quota` subcommands."""
import json
import subprocess
import sys
import unittest
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
CLI = REPO / 'bin' / 'agentkit'
SCRIPT = REPO / 'harness' / 'plugins' / 'quota-guardian' / 'scripts' / 'quota_status.py'


def run(*args):
    return subprocess.run([sys.executable, str(CLI), *args], capture_output=True, text=True, timeout=60)


class TestProfiles(unittest.TestCase):
    def setUp(self):
        self.cat = json.loads((REPO / 'config' / 'orchestration.json').read_text())

    def test_profile_list(self):
        r = run('profile', 'list')
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn('flash-high', r.stdout)
        self.assertNotIn('pro-flash', r.stdout)
        self.assertIn('complexity_tiers', r.stdout)
        self.assertIn('fallback', r.stdout)

    def test_default_and_primary_profiles(self):
        self.assertEqual(self.cat['default_profile'], 'argon-flash')
        self.assertIn('flash-high', self.cat['profiles'])
        self.assertNotIn('pro-flash', self.cat['profiles'])
        for name in ('argon-flash', 'opus-flash', 'sonnet-flash'):
            self.assertIn(name, self.cat['profiles'])

    def test_complexity_tiers_everywhere(self):
        for name, p in self.cat['profiles'].items():
            t = p['complexity_tiers']
            self.assertEqual(set(t), {'low', 'med', 'high'}, name)
            self.assertNotIn('thinking_levels', p)
            self.assertEqual(t['low']['model'], 'flash_lite', name)
            self.assertEqual(t['med']['model'], 'flash', name)
            self.assertEqual(t['high']['model'], 'flash', name)
            self.assertEqual(t['high']['escalate_to'], 'pro', name)
            for tier in t.values():
                self.assertTrue(tier['use'], name)
            self.assertTrue(p['worker_model'], name)
            self.assertIsInstance(p['quota_fallback'], list, name)

    def test_deprecated_alias(self):
        r = run('profile', 'show', 'pro-flash')
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn('deprecated', r.stderr)
        self.assertIn('flash-high', r.stdout)

    def test_apply_dry_run_alias(self):
        r = run('apply', '--dry-run', '--model-profile', 'pro-flash')
        self.assertIn('deprecated', r.stderr)


class TestQuotaCli(unittest.TestCase):
    def test_help_lists_quota(self):
        self.assertIn('quota', run('--help').stdout)

    def test_missing_plugin_message(self):
        code = "\n".join([
            "import importlib.machinery, importlib.util, sys, argparse, pathlib",
            f"l = importlib.machinery.SourceFileLoader('ak', {str(CLI)!r})",
            "m = importlib.util.module_from_spec(importlib.util.spec_from_loader('ak', l)); l.exec_module(m)",
            "m.QUOTA_SCRIPT = pathlib.Path('/nonexistent/quota_status.py')",
            "sys.exit(m.cmd_quota_status(argparse.Namespace(json=False)))",
        ])
        r = subprocess.run([sys.executable, '-c', code], capture_output=True, text=True, timeout=60)
        self.assertEqual(r.returncode, 1)
        self.assertIn('not found', r.stderr)

    @unittest.skipUnless(SCRIPT.exists(), 'quota-guardian script not present')
    def test_quota_status_json(self):
        self.assertIn(run('quota', 'status', '--json').returncode, (0, 1))

    @unittest.skipUnless(SCRIPT.exists(), 'quota-guardian script not present')
    def test_quota_route(self):
        self.assertIn(run('quota', 'route', '--complexity', 'low', '--json').returncode, (0, 1))


if __name__ == '__main__':
    unittest.main()
