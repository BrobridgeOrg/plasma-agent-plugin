import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch
import zipfile

from package_plugin import ROOT, build


SKILL_NAMES = {'plasma-create-view', 'plasma-create-bi', 'plasma-mcp-setup', 'plasma-knowledge-lookup',
               'plasma-document-analysis', 'plasma-fill-gaps', 'plasma-report', 'plasma-storage-lookup'}


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


class AdvancedSkillsTest(unittest.TestCase):
    """The document analysis and gap skills come from the Leibniz experiment;
    only how they connect changes: through the Plasma gateway, a one-time
    upload URL, and the commit page run_status names."""

    def skill(self, name):
        return (ROOT / 'skills' / name / 'SKILL.md').read_text(encoding='utf-8')

    def test_frontmatter_names_each_skill(self):
        for name in ('plasma-document-analysis', 'plasma-fill-gaps'):
            text = self.skill(name)
            self.assertTrue(text.startswith('---\nname: ' + name + '\n'), name)
            self.assertIn('description:', text.split('---')[1], name)

    def test_report_skills_let_the_program_write_the_sql(self):
        """Phase 3: the program compiles the report; the agent asks the
        person what each hole asks, keeps the answer, fills the hole and runs
        the complete query through Plasma."""
        for name in ('plasma-report', 'plasma-storage-lookup'):
            text = self.skill(name)
            self.assertTrue(text.startswith('---\nname: ' + name + '\n'), name)
            self.assertIn('compile_query', text, name)
        report = self.skill('plasma-report')
        for word in ('reading', 'holes', 'question', 'submit_answer', 'fills', 'complete', 'run_query', 'release', 'as_of'):
            self.assertIn(word, report, word)
        self.assertIn('known_at', self.skill('plasma-storage-lookup'))

    def test_report_skills_read_with_the_read_group(self):
        """The report and storage agents use the read group's tools
        (read_knowledge, compile_query), not the document analysis's
        lookup_knowledge; an answer that cannot be kept still fills the hole
        of this report."""
        for name in ('plasma-report', 'plasma-storage-lookup'):
            text = self.skill(name)
            self.assertIn('read_knowledge', text, name)
            self.assertNotIn('lookup_knowledge', text, name)
        self.assertIn('沒有 `submit_answer`', self.skill('plasma-report'))

    def test_report_skill_corrects_a_refused_request(self):
        """A refused request is corrected and sent again: the suggested
        concepts, select for a value that is no measure, and a paged reply
        read to its end (Q04-Q30 2026-10-28)."""
        report = self.skill('plasma-report')
        for word in ('suggestions', 'is not a measure', 'read_tool_reply', 'has_more', 'aggregate'):
            self.assertIn(word, report, word)

    def test_report_skill_asks_what_the_knowledge_lacks(self):
        """A grouping or condition no published concept is goes in the
        request as ask; the person is asked in plain words and answers a
        column; the fill reads expects.rows; a failed validation is fixed,
        not asked (Q06 2026-10-28)."""
        report = self.skill('plasma-report')
        for word in ('"ask"', 'expects.rows', 'identity', 'validation', '母體'):
            self.assertIn(word, report, word)

    def test_report_skill_asks_by_the_attributes_own_name(self):
        """ask names the attribute alone; the question adds the entity
        it belongs to (Q06c 2026-10-28: 「發票明細」的「發票明細所屬工廠」)."""
        self.assertIn('只寫屬性本身', self.skill('plasma-report'))

    def test_no_experiment_connection_is_left(self):
        for name in ('plasma-document-analysis', 'plasma-fill-gaps'):
            text = self.skill(name)
            for stale in ('127.0.0.1', 'leibniz-', 'MCP 網址加上', '上傳面板', 'ssh -L'):
                self.assertNotIn(stale, text, f'{name} still says {stale!r}')
            self.assertIn('commit_page', text, name)

    def test_skills_teach_units_nodes_and_completeness(self):
        """1G: a value concept states its unit with the source quote, a
        computation or metric is a node, and gaps name units, measures and
        the root causes an incomplete concept points to."""
        analysis = self.skill('plasma-document-analysis')
        for want in ('arg_values', 'unit_quote', 'submit_node', 'op measure', 'submit_annotation'):
            self.assertIn(want, analysis)
        gaps = self.skill('plasma-fill-gaps')
        for want in ('unit_unknown', 'unit_mismatch', 'measure_incomplete', 'incomplete', 'causes'):
            self.assertIn(want, gaps)

    def test_setup_lists_knowledge_write(self):
        self.assertIn('`knowledge:write`', self.skill('plasma-mcp-setup'))
