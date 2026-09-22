import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch
import zipfile

from package_plugin import ROOT, build


SKILL_NAMES = {'plasma-create-pview', 'plasma-mcp-setup', 'ophion-knowledge-lookup'}


def by_host(archives):
    """Index archives by the host segment of their filename."""
    return {path.name.rsplit('_', 1)[1].removesuffix('.zip'): path for path in archives}


class PackageTest(unittest.TestCase):
    def test_both_hosts_have_identical_skills_and_no_runtime(self):
        with tempfile.TemporaryDirectory() as directory, patch.dict('os.environ', {'GITHUB_REF_NAME': ''}):
            archives = build(output=directory)
            indexed = by_host(archives)
            self.assertEqual(set(indexed), {'chatgpt', 'claude', 'opencode'})
            skill_sets = []
            for archive, folder in ((indexed['chatgpt'], '.codex-plugin'),
                                    (indexed['claude'], '.claude-plugin')):
                with zipfile.ZipFile(archive) as zf:
                    names = zf.namelist()
                    manifest = f'plasma-plugin/{folder}/plugin.json'
                    self.assertIn(manifest, names)
                    skills = {n: zf.read(n) for n in names if '/skills/' in n}
                    self.assertEqual({Path(n).parent.name for n in skills if n.endswith('/SKILL.md')},
                                     SKILL_NAMES)
                    self.assertEqual(set(names), {manifest, 'plasma-plugin/.mcp.json', *skills})
                    self.assertEqual(json.loads(zf.read(manifest))['mcpServers'], './.mcp.json')
                    self.assertEqual(json.loads(zf.read('plasma-plugin/.mcp.json')), {
                        'mcpServers': {'plasma': {'type': 'http',
                            'url': 'https://plasma-mcp.bbg-x.top/mcp'}}})
                    # Links between skills must resolve within the actual archive.
                    for name, content in skills.items():
                        for target in re.findall(r'\]\(([^)]+\.md)\)', content.decode()):
                            from posixpath import normpath, join, dirname
                            self.assertIn(normpath(join(dirname(name), target)), names)
                    skill_sets.append(skills)
            self.assertEqual(*skill_sets)
            first = [p.read_bytes() for p in archives]
            self.assertEqual(first, [p.read_bytes() for p in build(output=directory)])

    def test_opencode_archive_carries_the_config_fragment_and_no_manifest(self):
        with tempfile.TemporaryDirectory() as directory, patch.dict('os.environ', {'GITHUB_REF_NAME': ''}):
            indexed = by_host(build(output=directory))
            self.assertIn('opencode', indexed)

            with zipfile.ZipFile(indexed['opencode']) as zf:
                names = set(zf.namelist())
                skills = {n: zf.read(n) for n in names if '/skills/' in n}
                self.assertEqual({Path(n).parent.name for n in skills if n.endswith('/SKILL.md')},
                                 SKILL_NAMES)
                # opencode reads neither manifest; shipping one would only be a
                # second place for the version to drift.
                self.assertFalse([n for n in names if n.endswith('plugin.json')])
                self.assertEqual(names, {'plasma-plugin/opencode.json',
                                         'plasma-plugin/INSTALL.md', *skills})

                config = json.loads(zf.read('plasma-plugin/opencode.json'))
                server = config['mcp']['plasma']
                self.assertEqual(server['type'], 'remote')
                # No headers: this host authenticates by OAuth, and a bearer
                # header would switch that off.
                self.assertNotIn('headers', server)
                # No scope either. The gateway's 401 challenge settles what a
                # client asks for, ahead of anything configured locally, so a
                # scope here would only look like a knob that works.
                self.assertNotIn('oauth', server)
                self.assertEqual(server['url'], 'https://plasma-mcp.bbg-x.top/mcp')

            # Same skills in every archive, whatever shape the archive is.
            with zipfile.ZipFile(indexed['claude']) as claude_zf:
                claude_skills = {n: claude_zf.read(n) for n in claude_zf.namelist() if '/skills/' in n}
            self.assertEqual(skills, claude_skills)

    def test_app_binding_is_only_in_linked_chatgpt_archive(self):
        with tempfile.TemporaryDirectory() as directory, patch.dict('os.environ', {'GITHUB_REF_NAME': ''}):
            indexed = by_host(build(output=directory, app_id='asdk_app_testfixture'))
            chatgpt, claude = indexed['chatgpt-linked'], indexed['claude']
            with zipfile.ZipFile(chatgpt) as zf:
                app = json.loads(zf.read('plasma-plugin/.app.json'))
                self.assertEqual(app['apps']['plasma'], {'id': 'asdk_app_testfixture', 'required': True})
                manifest = json.loads(zf.read('plasma-plugin/.codex-plugin/plugin.json'))
                self.assertEqual(manifest['apps'], './.app.json')
                self.assertEqual(manifest['mcpServers'], './.mcp.json')
            with zipfile.ZipFile(claude) as zf:
                self.assertNotIn('plasma-plugin/.app.json', zf.namelist())
            self.assertFalse((ROOT / '.app.json').exists())

    def test_bad_app_ids_and_wrong_release_tags_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory, patch.dict('os.environ', {'GITHUB_REF_NAME': ''}):
            for value in ('plugin_123', 'https://example.com/mcp', '', 'asdk_app_'):
                with self.assertRaises(ValueError):
                    build(output=directory, app_id=value)
            with patch.dict('os.environ', {'GITHUB_REF_NAME': 'v999.0.0'}):
                with self.assertRaises(ValueError):
                    build(output=directory)


if __name__ == '__main__':
    unittest.main()
