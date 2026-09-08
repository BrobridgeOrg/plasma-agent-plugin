#!/usr/bin/env python3
"""Exercise installation and MCP/hook protocol behavior without Go on PATH."""
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
VERSION = (ROOT / 'VERSION').read_text().strip()
OS = {'Darwin': 'darwin', 'Linux': 'linux'}[platform.system()]
ARCH = {'arm64': 'arm64', 'aarch64': 'arm64', 'x86_64': 'amd64'}[platform.machine()]
ASSET = f'plasma-plugin-mcp_{VERSION}_{OS}_{ARCH}.tar.gz'
EVENT = json.dumps({'hook_event_name': 'PreToolUse', 'tool_name': 'mcp__plasma__sync_view'})


class LauncherTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='plasma launcher ')
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.release = self.base / 'release'
        self.release.mkdir()
        with tarfile.open(self.release / ASSET, 'w:gz') as archive:
            archive.add(ROOT / 'bin/plasma-plugin-mcp', arcname='plasma-plugin-mcp')
        digest = hashlib.sha256((self.release / ASSET).read_bytes()).hexdigest()
        (self.release / 'checksums.txt').write_text(f'{digest}  {ASSET}\n')
        self.path = self.base / 'path'
        self.path.mkdir()
        # Include only runtime utilities, intentionally excluding Go, gh, and curl.
        for name in ['bash', 'dirname', 'cat', 'uname', 'mkdir', 'mktemp', 'cp', 'awk',
                     'tar', 'gzip', 'chmod', 'mv', 'rm', 'shasum', 'sha256sum']:
            if source := shutil.which(name):
                (self.path / name).symlink_to(source)
        self.env = {k: v for k, v in os.environ.items()
                    if not k.startswith(('PLASMA_', 'OPHION_'))}
        self.env.update(PATH=str(self.path), PLASMA_PLUGIN_HOME=str(self.base / 'home'),
                        PLASMA_MCP_RELEASE_DIR=str(self.release))
        self.binary = self.base / f'home/bin/v{VERSION}/{OS}-{ARCH}/plasma-plugin-mcp'

    def launch(self, mode='hook', payload=EVENT):
        return subprocess.run(['/bin/bash', str(ROOT / 'bin/plasma-mcp.sh'), mode],
                              input=payload, text=True, capture_output=True,
                              env=self.env, timeout=30)

    def assert_hook(self, result):
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)['hookSpecificOutput']['permissionDecision'], 'ask')

    def test_first_install_and_offline_cache(self):
        first = self.launch()
        self.assert_hook(first)
        self.assertIn('Installing', first.stderr)
        self.assertTrue(self.binary.is_file())
        shutil.rmtree(self.release)
        cached = self.launch()
        self.assert_hook(cached)
        self.assertEqual(cached.stderr, '')

    def test_corrupt_download_is_not_installed(self):
        with (self.release / ASSET).open('ab') as archive:
            archive.write(b'corrupted')
        result = self.launch()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, '')
        self.assertIn('Checksum mismatch', result.stderr)
        self.assertFalse(self.binary.exists())
        self.assertEqual(list(self.binary.parent.glob('.download.*')), [])

    def test_preparation_does_not_prompt(self):
        for tool, args in [('run_query', {'sql': 'SELECT 1'}),
                           ('create_view', {'sync_mode': 'manual'}),
                           ('create_view', {})]:
            with self.subTest(tool=tool, args=args):
                payload = json.dumps({'hook_event_name': 'PreToolUse',
                                      'tool_name': 'mcp__plasma__' + tool,
                                      'tool_input': args})
                result = self.launch(payload=payload)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, '')

    def test_scheduled_sync_and_api_have_chinese_prompts(self):
        for tool, args, message in [
            ('create_view', {'sync_mode': 'scheduled'}, '立即開始'),
            ('create_access_entry', {'auth_type': 'none'}, '不需驗證即可讀取')
        ]:
            with self.subTest(tool=tool):
                payload = json.dumps({'hook_event_name': 'PreToolUse',
                                      'tool_name': 'mcp__plasma__' + tool,
                                      'tool_input': args})
                result = self.launch(payload=payload)
                self.assert_hook(result)
                reason = json.loads(result.stdout)['hookSpecificOutput']['permissionDecisionReason']
                self.assertIn(message, reason)

    def test_missing_checksum_is_rejected(self):
        (self.release / 'checksums.txt').write_text('')
        result = self.launch()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Missing or invalid checksum', result.stderr)
        self.assertFalse(self.binary.exists())

    def test_concurrent_first_launches(self):
        with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
            results = list(pool.map(lambda _: self.launch(), range(3)))
        for result in results:
            self.assert_hook(result)
        self.assertEqual(list(self.binary.parent.glob('.download.*')), [])

    def test_explicit_local_binary(self):
        self.env['PLASMA_MCP_BINARY'] = str(ROOT / 'bin/plasma-plugin-mcp')
        shutil.rmtree(self.release)
        self.assert_hook(self.launch())
        self.assertFalse(self.binary.exists())

    def test_public_download_and_cache(self):
        del self.env['PLASMA_MCP_RELEASE_DIR']
        self.env['TEST_RELEASE'] = str(self.release)
        curl = self.path / 'curl'
        curl.write_text('''#!/bin/bash
set -euo pipefail
for arg in "$@"; do
  case "$arg" in
    https://github.com/BrobridgeOrg/plasma-agent-plugin/releases/download/v*) url="$arg" ;;
  esac
  destination="$arg"
done
cp "$TEST_RELEASE/${url##*/}" "$destination"
''')
        curl.chmod(0o700)
        self.assert_hook(self.launch())
        curl.unlink()
        self.assert_hook(self.launch())

    def test_authenticated_download(self):
        del self.env['PLASMA_MCP_RELEASE_DIR']
        self.env['TEST_RELEASE'] = str(self.release)
        gh = self.path / 'gh'
        gh.write_text('''#!/bin/bash
set -euo pipefail
if [[ "$1 $2" == "auth status" ]]; then exit 0; fi
[[ "$1 $2" == "release download" ]]
[[ "$3" == "v''' + VERSION + '''" ]]
[[ "$4 $5" == "--repo BrobridgeOrg/plasma-agent-plugin" ]]
for arg in "$@"; do destination="$arg"; done
cp "$TEST_RELEASE/"* "$destination/"
echo "download progress" >&2
''')
        gh.chmod(0o700)
        self.assert_hook(self.launch())

    def test_download_failure_preserves_clean_stdout(self):
        del self.env['PLASMA_MCP_RELEASE_DIR']
        curl = self.path / 'curl'
        curl.write_text('#!/bin/bash\nexit 22\n')
        curl.chmod(0o700)
        result = self.launch()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, '')
        self.assertIn('gh auth login', result.stderr)
        self.assertFalse(self.binary.exists())

    def test_stdio_initialize_uses_release_version(self):
        self.env.update(PLASMA_URL='http://127.0.0.1:1', PLASMA_TOKEN='test-only')
        request = {'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {
            'protocolVersion': '2025-03-26', 'capabilities': {},
            'clientInfo': {'name': 'launcher-test', 'version': '1'}}}
        for mode in ['plasma', 'ophion']:
            with self.subTest(mode=mode):
                # Keep stdin open until the server responds, then close cleanly.
                with subprocess.Popen(['/bin/bash', str(ROOT / 'bin/plasma-mcp.sh'), mode],
                                      stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                      stderr=subprocess.PIPE, text=True, env=self.env) as proc:
                    try:
                        proc.stdin.write(json.dumps(request) + '\n')
                        proc.stdin.flush()
                        import select
                        ready, _, _ = select.select([proc.stdout], [], [], 15)
                        self.assertTrue(ready, 'MCP initialize timed out')
                        response = json.loads(proc.stdout.readline())
                        self.assertEqual(response['id'], 1)
                        self.assertEqual(response['result']['serverInfo']['version'], VERSION)
                    finally:
                        proc.stdin.close()
                        try:
                            proc.wait(timeout=5)
                        except subprocess.TimeoutExpired:
                            proc.kill()
                            proc.wait()


if __name__ == '__main__':
    unittest.main(verbosity=2)
