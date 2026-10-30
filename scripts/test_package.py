import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch
import zipfile

from package_plugin import ROOT, build


SKILL_NAMES = {'plasma-create-view', 'plasma-create-bi', 'plasma-mcp-setup', 'plasma-knowledge-query',
               'plasma-document-analysis', 'plasma-fill-gaps', 'plasma-report', 'plasma-remap'}


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




class AdvancedSkillsTest(unittest.TestCase):
    """The document analysis and gap skills come from the Leibniz experiment;
    only how they connect changes: through the Plasma gateway, a one-time
    upload URL, and the commit page run_status names."""

    def skill(self, name):
        return (ROOT / 'skills' / name / 'SKILL.md').read_text(encoding='utf-8')

    def reference(self, path):
        return (ROOT / 'skills' / 'plasma-knowledge-query' / 'references' / path).read_text(encoding='utf-8')

    def compiled(self):
        """The advanced sql-path the report and the view skills share."""
        return self.reference('sql-path/advanced.md')

    def test_frontmatter_names_each_skill(self):
        for name in ('plasma-document-analysis', 'plasma-fill-gaps'):
            text = self.skill(name)
            self.assertTrue(text.startswith('---\nname: ' + name + '\n'), name)
            self.assertIn('description:', text.split('---')[1], name)

    def test_report_skills_let_the_program_write_the_sql(self):
        """Phase 3: the program compiles the report; the agent asks the
        person what each hole asks, keeps the answer, fills the hole and runs
        the complete query through Plasma."""
        self.assertTrue(self.skill('plasma-report').startswith('---\nname: plasma-report\n'))
        self.assertIn('compile_query', self.skill('plasma-report'))
        self.assertIn('compile_query', self.reference('advanced.md'))
        compiled = self.compiled()
        for word in ('reading', 'holes', 'question', 'submit_answer', 'fills', 'complete', 'release', 'as_of'):
            self.assertIn(word, compiled, word)
        self.assertIn('run_query', self.skill('plasma-report'))
        self.assertIn('known_at', self.reference('advanced.md'))

    def test_report_skills_read_with_the_read_group(self):
        """The report and storage agents use the read group's tools
        (read_knowledge, compile_query), not the document analysis's
        lookup_knowledge; an answer that cannot be kept still fills the hole
        of this report."""
        for name, text in (('report', self.skill('plasma-report')), ('lookup', self.reference('advanced.md')),
                           ('sql-path', self.compiled())):
            self.assertIn('read_knowledge', text, name)
            self.assertNotIn('lookup_knowledge', text, name)
        self.assertIn('沒有 `submit_answer`', self.compiled())

    def test_report_skill_corrects_a_refused_request(self):
        """A refused request is corrected and sent again: the suggested
        concepts, select for a value that is no measure, and a paged reply
        read to its end (Q04-Q30 2026-10-28)."""
        report = self.compiled()
        for word in ('suggestions', 'is not a measure', 'read_tool_reply', 'has_more', 'aggregate'):
            self.assertIn(word, report, word)

    def test_report_skill_asks_what_the_knowledge_lacks(self):
        """A grouping or condition no published concept is goes in the
        request as ask; the person is asked in plain words and answers a
        column; the fill reads expects.rows; a failed validation is fixed,
        not asked (Q06 2026-10-28)."""
        report = self.compiled()
        for word in ('"ask"', 'expects.rows', 'identity', 'validation', '母體'):
            self.assertIn(word, report, word)

    def test_report_skill_asks_by_the_attributes_own_name(self):
        """ask names the attribute alone; the question adds the entity
        it belongs to (Q06c 2026-10-28: 「發票明細」的「發票明細所屬工廠」)."""
        self.assertIn('只寫屬性本身', self.compiled())

    def test_report_skill_lists_a_ratios_parts_in_one_query(self):
        """A ratio's numerator and denominator the question lists are
        named in columns of the same request (Q05 2026-10-29: three
        queries, one per column)."""
        self.assertIn('分子、分母', self.compiled())

    def test_report_skill_presents_around_the_compiled_query(self):
        """Presentation (a ROC year, a format, an order) wraps the compiled
        query unchanged in an outer SELECT and is told; another calendar is
        knowledge, not a bucket; a ROC year in a condition becomes the
        Gregorian period (2026-10-29)."""
        report = self.skill('plasma-report') + self.compiled()
        for word in ('外面包一層', '原封不動', '民國', '農曆', 'bucket'):
            self.assertIn(word, report, word)

    def test_document_analysis_lists_uploads_when_it_checks_them(self):
        """run_status lists every upload only with uploads=true (Ophion
        2026-10-29); the main agent asks for them when it checks states."""
        self.assertIn('`uploads` 為 true', self.skill('plasma-document-analysis'))

    def test_remap_skill_runs_mapping_after_a_schema_change(self):
        """A schema change is remapped with run_mapping before new documents
        (user ruling 2026-10-29); document analysis sends the agent there."""
        remap = self.skill('plasma-remap')
        for word in ('run_mapping', 'remap_needed', 'read_task', 'mapping_finish', 'commit_page', '不重新摘錄',
                     '最後的回報固定包含', '待補', '確認發布'):
            self.assertIn(word, remap, word)
        self.assertIn('plasma-remap', self.skill('plasma-document-analysis'))

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


class KnowledgeQueryTest(unittest.TestCase):
    """A knowledge base is simple or advanced for good (Ophion app.mode);
    the skills are the same capabilities, and only the path to the knowledge
    differs: knowledge-query holds both paths and the sql-path the view and
    report skills share."""

    def text(self, *parts):
        return (ROOT / 'skills').joinpath(*parts).read_text(encoding='utf-8')

    def test_entry_tells_the_mode_by_the_tools(self):
        entry = self.text('plasma-knowledge-query', 'SKILL.md')
        self.assertTrue(entry.startswith('---\nname: plasma-knowledge-query\n'))
        for word in ('read_knowledge', 'compile_query', 'search_knowledge', 'get_table_card',
                     'references/standard.md', 'references/advanced.md',
                     'references/sql-path/standard.md', 'references/sql-path/advanced.md'):
            self.assertIn(word, entry, word)

    def test_create_view_takes_its_sql_from_either_path(self):
        view = self.text('plasma-create-view', 'SKILL.md')
        for word in ('sql-path/standard.md', 'sql-path/advanced.md', 'compile_query', 'UNION ALL'):
            self.assertIn(word, view, word)
        self.assertNotIn('plasma-knowledge-lookup', view)

    def test_a_view_compiles_without_a_cutoff_or_window(self):
        """An mview syncs again and again: without as_of each sync reads every
        key's latest version (sqlcompile rows()); the period is a bucketed
        dimension the pview filters, not a window."""
        compiled = self.text('plasma-knowledge-query', 'references', 'sql-path', 'advanced.md')
        for word in ('要建成 mview 時', '不帶 `as_of`', '不帶 `window`', 'bucket', '不算 schema 變更'):
            self.assertIn(word, compiled, word)

    def test_advanced_only_skills_stop_in_simple_mode(self):
        """One plugin serves both modes, so an advanced-only skill can be
        triggered on a simple-mode system: it tells the mode by the tools
        before it checks knowledge:write, says what to do instead and stops,
        without asking for a reauthorization that cannot grant the scope."""
        for name in ('plasma-document-analysis', 'plasma-fill-gaps', 'plasma-remap', 'plasma-report'):
            text = self.text(name, 'SKILL.md')
            for word in ('本 skill 只用於進階模式', '核對 scopes 之前', '`compile_query`', '`search_knowledge`',
                         '結束本 skill', '不要求使用者重新授權', '../plasma-knowledge-query/SKILL.md'):
                self.assertIn(word, text, f'{name}: {word}')
        self.assertIn('請在 Plasma 介面上傳', self.text('plasma-document-analysis', 'SKILL.md'))

    def test_code_names_are_supplemented_in_either_mode(self):
        """A missing code name is offered as a candidate in both modes; the
        skills that use it link the mode-neutral page, not the simple-mode
        lookup, and the advanced page keeps the compiled SQL unchanged."""
        names = self.text('plasma-knowledge-query', 'references', 'code-names.md')
        for word in ('一般模式', '進階模式', 'VALUES', 'CASE', '原封不動', '不用 `r.*`'):
            self.assertIn(word, names, word)
        for name in ('plasma-create-view', 'plasma-create-bi'):
            text = self.text(name, 'SKILL.md')
            self.assertIn('references/code-names.md', text, name)
            self.assertNotIn('references/standard.md', text, name)

    def test_failures_are_told_before_the_mode_is_known(self):
        """With neither tool set the mode is unknown, so the whoami
        diagnosis lives in the entry, not in the simple-mode page."""
        entry = self.text('plasma-knowledge-query', 'SKILL.md')
        self.assertIn('## 工具缺少或失敗', entry)
        self.assertNotIn('## 工具缺少或失敗', self.text('plasma-knowledge-query', 'references', 'standard.md'))

    def test_skills_call_it_plasma_sql(self):
        """The skills say Plasma SQL; only the deepest page, the SQL rules,
        says it is Trino syntax (ANSI SQL with Trino's functions)."""
        rules = ROOT / 'skills' / 'plasma-knowledge-query' / 'references' / 'sql-path' / 'plasma-sql.md'
        self.assertIn('Trino', rules.read_text(encoding='utf-8'))
        for path in (ROOT / 'skills').rglob('*.md'):
            if path != rules:
                self.assertNotIn('trino', path.read_text(encoding='utf-8').lower(), path)

    def test_compiled_sql_explains_relation_holes(self):
        """A relation hole is asked with the relationship it carries and,
        once the person confirms, filled from it; the confirmation or the
        join columns are kept when the agent may write."""
        text = self.text('plasma-knowledge-query', 'references', 'sql-path', 'advanced.md')
        for word in ('relation_unverified', 'relation_refuted', '`relation`', 'accept_relationship', 'complete_relationship', 'join_unknown'):
            self.assertIn(word, text, word)
        version = (ROOT / 'VERSION').read_text(encoding='utf-8').strip()
        for manifest in ('.claude-plugin', '.codex-plugin'):
            data = json.loads((ROOT / manifest / 'plugin.json').read_text(encoding='utf-8'))
            self.assertEqual(version, data['version'], manifest)
        self.assertTrue((ROOT / 'releases' / f'v{version}.md').exists())

    def test_sql_rules_are_written_once(self):
        rules = self.text('plasma-knowledge-query', 'references', 'sql-path', 'plasma-sql.md')
        self.assertIn('approx_percentile', rules)
        for path in (('references', 'sql-path', 'standard.md'), ('references', 'sql-path', 'advanced.md')):
            text = self.text('plasma-knowledge-query', *path)
            self.assertIn('plasma-sql.md', text, path)
            self.assertNotIn('approx_percentile', text, path)


if __name__ == '__main__':
    unittest.main()
