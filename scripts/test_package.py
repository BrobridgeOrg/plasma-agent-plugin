import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch
import zipfile

from package_plugin import ROOT, build


class PackageTest(unittest.TestCase):
    def test_both_hosts_have_identical_skills_and_no_runtime(self):
        with tempfile.TemporaryDirectory() as directory, patch.dict('os.environ', {'GITHUB_REF_NAME': ''}):
            archives = build(output=directory)
            skill_sets = []
            for archive, folder in zip(archives, ('.codex-plugin', '.claude-plugin')):
                with zipfile.ZipFile(archive) as zf:
                    names = zf.namelist()
                    manifest = f'plasma-plugin/{folder}/plugin.json'
                    self.assertIn(manifest, names)
                    skills = {n: zf.read(n) for n in names if '/skills/' in n}
                    self.assertEqual({Path(n).parent.name for n in skills if n.endswith('/SKILL.md')},
                                     {'plasma-create-view', 'plasma-mcp-setup', 'ophion-knowledge-lookup'})
                    self.assertEqual(set(names), {manifest, *skills})
                    # Links between skills must resolve within the actual archive.
                    for name, content in skills.items():
                        for target in re.findall(r'\]\(([^)]+\.md)\)', content.decode()):
                            from posixpath import normpath, join, dirname
                            self.assertIn(normpath(join(dirname(name), target)), names)
                    skill_sets.append(skills)
            self.assertEqual(*skill_sets)
            first = [p.read_bytes() for p in archives]
            self.assertEqual(first, [p.read_bytes() for p in build(output=directory)])

    def test_app_binding_is_only_in_linked_chatgpt_archive(self):
        with tempfile.TemporaryDirectory() as directory, patch.dict('os.environ', {'GITHUB_REF_NAME': ''}):
            chatgpt, claude = build(output=directory, app_id='asdk_app_testfixture')
            with zipfile.ZipFile(chatgpt) as zf:
                app = json.loads(zf.read('plasma-plugin/.app.json'))
                self.assertEqual(app['apps']['plasma'], {'id': 'asdk_app_testfixture', 'required': True})
                manifest = json.loads(zf.read('plasma-plugin/.codex-plugin/plugin.json'))
                self.assertEqual(manifest['apps'], './.app.json')
                self.assertNotIn('mcpServers', manifest)
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
