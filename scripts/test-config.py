#!/usr/bin/env python3
"""Exercise the setup helper that writes ~/.plasma-plugin/config.env."""
import json
import os
from pathlib import Path
import pty
import select
import stat
import subprocess
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / 'bin/plasma-config.sh'


class ConfigTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='plasma config ')
        self.addCleanup(self.tmp.cleanup)
        self.home = Path(self.tmp.name) / 'home'
        self.config = self.home / 'config.env'
        self.env = {k: v for k, v in os.environ.items()
                    if not k.startswith(('PLASMA_', 'OPHION_'))}
        self.env['PLASMA_PLUGIN_HOME'] = str(self.home)

    def run_config(self, *args, env=None):
        return subprocess.run(['/bin/bash', str(SCRIPT), *args], text=True,
                              capture_output=True, env={**self.env, **(env or {})},
                              timeout=30)

    def configure(self, **values):
        self.run_config('init')
        for key, value in values.items():
            self.assertEqual(self.run_config('set', key, value).returncode, 0)

    def complete(self):
        self.configure(PLASMA_URL='http://plasma.test', PLASMA_USERNAME='ops',
                       PLASMA_PASSWORD='secret', OPHION_URL='http://ophion.test',
                       OPHION_SERVICE_TOKEN='tok')

    def test_template_leaves_every_value_empty(self):
        # A template default would read back as a configured value, and check
        # would call a setup complete that nobody has filled in.
        for line in (ROOT / 'config.env.example').read_text().splitlines():
            if line.strip() and not line.startswith('#'):
                self.assertTrue(line.endswith('='), line)

    def test_init_creates_private_file_from_template(self):
        result = self.run_config('init')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), str(self.config))
        self.assertEqual(stat.S_IMODE(self.config.stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(self.home.stat().st_mode), 0o700)
        self.assertIn('PLASMA_URL', self.config.read_text())

    def test_init_keeps_an_existing_file(self):
        self.configure(PLASMA_URL='http://kept.test')
        self.run_config('init')
        self.assertIn('PLASMA_URL=http://kept.test', self.config.read_text())

    def test_set_replaces_rather_than_appends(self):
        self.configure(PLASMA_URL='http://first.test')
        self.run_config('set', 'PLASMA_URL', 'http://second.test')
        lines = [l for l in self.config.read_text().splitlines() if l.startswith('PLASMA_URL=')]
        self.assertEqual(lines, ['PLASMA_URL=http://second.test'])

    def test_set_collapses_a_hand_written_duplicate(self):
        self.run_config('init')
        with self.config.open('a') as handle:
            handle.write('PLASMA_URL=http://a.test\nPLASMA_URL=http://b.test\n')
        self.run_config('set', 'PLASMA_URL', 'http://c.test')
        lines = [l for l in self.config.read_text().splitlines() if l.startswith('PLASMA_URL=')]
        self.assertEqual(lines, ['PLASMA_URL=http://c.test'])

    def test_awkward_values_survive_a_round_trip(self):
        # The Go side trims the value and strips matching quotes; everything
        # else, '=' and '#' included, has to come back exactly as entered.
        self.configure(PLASMA_PASSWORD='p@ss w=rd#1')
        self.assertIn('PLASMA_PASSWORD=p@ss w=rd#1', self.config.read_text().splitlines())

    def test_unknown_key_is_refused(self):
        self.run_config('init')
        result = self.run_config('set', 'PLASMA_SECRET_SAUCE', 'x')
        self.assertEqual(result.returncode, 1)
        self.assertIn('unknown key', result.stderr)
        self.assertNotIn('SAUCE', self.config.read_text())

    def test_show_reports_values_but_never_secrets(self):
        self.complete()
        out = self.run_config('show').stdout
        self.assertIn('PLASMA_URL=http://plasma.test [file]', out)
        self.assertIn('PLASMA_PASSWORD=<set, 6 characters> [file]', out)
        self.assertNotIn('secret', out)
        self.assertIn('PLASMA_TOKEN=<unset>', out)

    def test_set_does_not_echo_a_secret(self):
        self.run_config('init')
        result = self.run_config('set', 'OPHION_SERVICE_TOKEN', 'super-secret')
        self.assertNotIn('super-secret', result.stdout + result.stderr)

    def test_check_names_what_is_missing(self):
        self.configure(PLASMA_URL='http://plasma.test', PLASMA_TOKEN='tok')
        result = self.run_config('check')
        self.assertEqual(result.returncode, 1)
        self.assertIn('status=incomplete', result.stdout)
        self.assertIn('OPHION_URL', result.stdout.rsplit('missing=', 1)[1])
        self.assertIn('OPHION_SERVICE_TOKEN', result.stdout.rsplit('missing=', 1)[1])

    def test_half_configured_password_auth_is_incomplete(self):
        self.configure(PLASMA_URL='http://plasma.test', PLASMA_USERNAME='ops',
                       OPHION_URL='http://ophion.test', OPHION_SERVICE_TOKEN='tok')
        self.assertIn('Plasma credentials', self.run_config('check').stdout)

    def test_check_passes_with_either_auth_mode(self):
        self.complete()
        self.assertEqual(self.run_config('check').returncode, 0)
        self.run_config('set', 'PLASMA_USERNAME', '')
        self.run_config('set', 'PLASMA_PASSWORD', '')
        self.assertEqual(self.run_config('check').returncode, 1)
        self.run_config('set', 'PLASMA_TOKEN', 'tok')
        self.assertEqual(self.run_config('check').returncode, 0)

    def test_environment_overrides_the_file(self):
        # The servers read the environment first, so the check has to as well.
        self.complete()
        out = self.run_config('show', env={'PLASMA_URL': 'http://override.test'}).stdout
        self.assertIn('PLASMA_URL=http://override.test [env]', out)

    def test_environment_alone_is_a_complete_setup(self):
        result = self.run_config('check', env={
            'PLASMA_URL': 'http://plasma.test', 'PLASMA_TOKEN': 'tok',
            'OPHION_URL': 'http://ophion.test', 'OPHION_SERVICE_TOKEN': 'tok'})
        self.assertEqual(result.returncode, 0, result.stdout)

    def test_session_start_asks_for_setup_only_while_incomplete(self):
        result = self.run_config('session-start')
        self.assertEqual(result.returncode, 0, result.stderr)
        context = json.loads(result.stdout)['hookSpecificOutput']
        self.assertEqual(context['hookEventName'], 'SessionStart')
        self.assertIn('plasma-plugin-setup', context['additionalContext'])
        self.assertIn('PLASMA_URL', context['additionalContext'])

        self.complete()
        quiet = self.run_config('session-start')
        self.assertEqual(quiet.returncode, 0, quiet.stderr)
        self.assertEqual(quiet.stdout, '')

    def test_session_start_stays_quiet_without_a_locatable_home(self):
        # A SessionStart hook that errors is noise at the top of every session.
        env = {k: v for k, v in self.env.items()
               if k not in ('HOME', 'PLASMA_PLUGIN_HOME')}
        result = subprocess.run(['/bin/bash', str(SCRIPT), 'session-start'], text=True,
                                capture_output=True, env=env, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, '')

    def test_set_secret_refuses_without_a_terminal(self):
        self.run_config('init')
        result = self.run_config('set-secret', 'PLASMA_PASSWORD')
        self.assertEqual(result.returncode, 1)
        self.assertIn('terminal', result.stderr)

    def test_set_secret_never_echoes_what_is_typed(self):
        # The skill promises a secret typed here stays off the screen and out
        # of the transcript, so the terminal must never see it.
        self.run_config('init')
        pid, fd = pty.fork()
        if pid == 0:  # the child replaces itself immediately
            os.execve('/bin/bash', ['/bin/bash', str(SCRIPT), 'set-secret', 'PLASMA_TOKEN'],
                      self.env)
        transcript = b''
        deadline = time.time() + 20
        while b'hidden' not in transcript and time.time() < deadline:
            if select.select([fd], [], [], 0.2)[0]:
                transcript += os.read(fd, 1024)
        self.assertIn(b'hidden', transcript, 'never reached the prompt')
        os.write(fd, b'sup3r-s3cret\n')
        try:
            while chunk := os.read(fd, 1024):
                transcript += chunk
        except OSError:
            pass
        self.assertEqual(os.waitstatus_to_exitcode(os.waitpid(pid, 0)[1]), 0)
        self.assertNotIn(b'sup3r-s3cret', transcript)
        self.assertIn('PLASMA_TOKEN=sup3r-s3cret', self.config.read_text().splitlines())

    def test_set_secret_refuses_a_non_secret_key(self):
        self.run_config('init')
        self.assertIn('not a secret', self.run_config('set-secret', 'PLASMA_URL').stderr)


if __name__ == '__main__':
    unittest.main(verbosity=2)
