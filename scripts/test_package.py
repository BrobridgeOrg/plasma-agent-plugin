import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch
import zipfile

from package_plugin import ROOT, build


SKILL_NAMES = {'plasma-create-view', 'plasma-create-bi', 'plasma-mcp-setup', 'plasma-knowledge-query',
               'plasma-document-analysis', 'plasma-fill-gaps', 'plasma-report', 'plasma-remap',
               'plasma-code-notes', 'plasma-guide'}


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

    def test_the_sql_path_proposes_what_it_reads_and_fills_once_confirmed(self):
        """A person confirms what is read, in business words; the compiler's
        reading lists their answers, and a compile with holes has no SQL."""
        text = self.compiled()
        for word in ('read_schema', 'answers', '確認'):
            self.assertIn(word, text)
        self.assertNotIn('{{h', text)
        self.assertNotIn('{{h', self.reference('advanced.md'))

    def test_fills_come_after_the_person_confirms_and_name_their_compile(self):
        """What the agent reads is proposed before anything is sent, and a
        reply with holes has no SQL to copy names from."""
        text = self.compiled()
        ask = text.index('把每個洞的 `question` 問使用者')
        confirm = text.index('使用者確認之後才補')
        send = text.index('送出同一個請求並帶 `fills`')
        self.assertLess(ask, confirm)
        self.assertLess(confirm, send)
        self.assertIn('`compile_id`', text[send:send + 200])
        self.assertNotIn('照編譯出的 SQL', text)

    def test_a_refused_query_is_told_apart_from_a_wrong_fragment(self):
        """Validation can fail away from the fragments, and an answer may be
        the faithful query rather than the optimized one."""
        text = self.compiled()
        self.assertIn('錯誤不在片段', text)
        self.assertIn('`optimized`', text)

    def test_a_question_is_read_as_the_knowledge_it_needs(self):
        """The agent looks for what the question needs before it fills a
        request; what fully matches is used, what does not is asked."""
        text = self.compiled()
        need = text.index('需要哪些知識')
        match = text.index('完整符合')
        near = text.index('不完全符合')
        request = text.index('把問題拆成請求的各部分')
        self.assertLess(need, match)
        self.assertLess(match, near)
        self.assertLess(near, request)
        self.assertIn('不自己決定', text)

    def test_a_near_concept_leads_to_the_concepts_it_reads(self):
        """A concept that does not fully match is still an index: what it
        reads may, and a new concept is built of that."""
        text = self.compiled()
        self.assertIn('`dependencies`', text)
        self.assertIn('索引', text)

    def test_a_new_concept_is_built_of_blocks_and_returns_to_the_commit_area(self):
        text = self.compiled()
        define = text.index('`define`')
        fragment = text.index('寫成一段 SQL')
        self.assertLess(define, fragment, 'a fragment is the last resort')
        self.assertIn('Commit 區', text[define:])
        self.assertIn('session.', text)

    def test_a_leaf_with_no_column_offers_candidates_before_it_binds(self):
        text = self.compiled()
        leaf = text.index('`expects.leaf`')
        search = text.index('`candidates`', leaf)
        choose = text.index('候選', search)
        bind = text.index('`bind`', choose)
        self.assertLess(leaf, search)
        self.assertLess(search, choose)
        self.assertLess(choose, bind)

    def test_a_changed_request_compiles_again_before_it_fills(self):
        """define and bind are the request: holes of the compile before them
        are no holes of this one."""
        text = self.compiled()
        change = text.index('改變後')
        self.assertIn('compile_id', text[change:change + 120])
        self.assertIn('不帶 `fills`', text[change:change + 120])

    def test_a_concept_with_no_definition_is_filled_under_its_own_name(self):
        """A published concept that reads the hole keeps reading it: a new name
        would leave the hole where it is."""
        text = self.compiled()
        self.assertIn('用那個概念自己的名稱', text)

    def test_a_reading_of_blocks_is_shown_with_what_it_warns(self):
        self.assertIn('不一定是同一筆', self.compiled())

    def test_a_search_cut_short_is_narrowed(self):
        self.assertIn('縮小', self.compiled())

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


class MissingReferenceTest(unittest.TestCase):
    def test_document_analysis_states_missing_references(self):
        """A cited document Ophion has is read with read_source; one it does
        not is asked for, saying what cannot be analysed and what is lost."""
        text = (ROOT / 'skills' / 'plasma-document-analysis' / 'SKILL.md').read_text()
        for want in ('read_source', '不補上，這些知識會遺失', '缺哪份文件、哪個段落'):
            self.assertIn(want, text)


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

    def test_the_guide_reads_the_state_and_hands_over(self):
        """Asked what Plasma can do, the agent reads the connection and the
        knowledge base first, lists what this workspace can do now and hands
        over to the skill that does it: the guide repeats no procedure."""
        guide = self.text('plasma-guide', 'SKILL.md')
        self.assertTrue(guide.startswith('---\nname: plasma-guide\n'))
        for word in ('`whoami`', '`overview`', '沒有 `overview` 時略過', 'references/advanced.md',
                     'references/standard.md', '只接受唯讀查詢', '對使用者只說這個系統有的功能'):
            self.assertIn(word, guide, word)
        pages = {name: self.text('plasma-guide', 'references', name + '.md') for name in ('advanced', 'standard')}
        for word in ('`rules`', 'knowledge:read', 'knowledge:write', 'query:run', 'views:write'):
            self.assertIn(word, pages['advanced'], word)
        linked = guide + ''.join(pages.values()).replace('../../', '../')
        for name in SKILL_NAMES - {'plasma-guide'}:
            self.assertIn(f'../{name}/SKILL.md', linked, name)
        for text in (guide, *pages.values()):
            for tool in ('compile_query(', 'create_view', 'request_upload'):
                self.assertNotIn(tool, text, f'{tool}: the procedure belongs to its own skill')

    def test_a_system_is_told_only_what_it_has(self):
        """One plugin serves two kinds of knowledge base (user 2026-11-13):
        the setup skill compares none, and the guide lists a system's own
        page, so a person is never told of what the other kind does."""
        setup = self.text('plasma-mcp-setup', 'SKILL.md')
        for word in ('進階模式', '一般模式', '兩種模式', '能做的事相同'):
            self.assertNotIn(word, setup, word)
        self.assertIn('../plasma-guide/SKILL.md', setup)
        guide = self.text('plasma-guide', 'SKILL.md')
        pages = {name: self.text('plasma-guide', 'references', name + '.md') for name in ('advanced', 'standard')}
        for name, text in (('guide', guide), *pages.items()):
            for word in ('進階模式', '一般模式', '兩種模式'):
                self.assertNotIn(word, text, f'{name}: {word}')
        for name in ('plasma-report', 'plasma-document-analysis', 'plasma-fill-gaps', 'plasma-remap', 'plasma-code-notes'):
            self.assertNotIn(name, pages['standard'], name)
            self.assertIn(name, pages['advanced'], name)
        for word in ('上傳', 'Commit'):
            self.assertNotIn(word, pages['standard'], word)

    def test_the_inventory_and_a_columns_notes_are_read_from_the_knowledge(self):
        """Which databases and tables there are is read from overview, and a
        table's or a column's notes and a calculated column's formula from
        read_schema: the published schema, not the engine's own catalog."""
        lookup = self.text('plasma-knowledge-query', 'references', 'advanced.md')
        for word in ('`overview`', '`database`', '`has_more`', '`rules`', '`next`', '`column`', '`notes`',
                     '`calculated`', '`sources`', '`unit`', '`concepts`', '`usage`', '從表或欄位問起時',
                     '工具清單沒有 `overview` 時'):
            self.assertIn(word, lookup, word)
        for text in (lookup, self.text('plasma-guide', 'SKILL.md')):
            self.assertNotIn('information_schema', text)

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


class ConceptOrDescriptionTest(unittest.TestCase):
    def test_document_analysis_judges_by_meaning_not_layout(self):
        """What a passage says, not its layout, makes it a concept or a
        description; how the agent reads is its own."""
        text = (ROOT / 'skills' / 'plasma-document-analysis' / 'SKILL.md').read_text()
        for want in ('看它說了什麼，不看它寫成句子、清單、表格還是標題', '怎麼讀、讀幾次、依什麼順序由你決定'):
            self.assertIn(want, text)
        self.assertNotIn('讀完全部內容才開始審查', text)


class RevisionGuidanceTest(unittest.TestCase):
    def test_document_analysis_revises_and_deletes_existing_knowledge(self):
        """The agent changes existing knowledge with a new version or a
        deletion, and a person's correction in conversation is an answer
        document revising the rule (design 2026-11-03 §4.2, §4.3, §4.6)."""
        text = (ROOT / 'skills' / 'plasma-document-analysis' / 'SKILL.md').read_text()
        for want in ('revise_rule', 'delete_knowledge', 'base_version', 'version_conflicts', '不帶 `gap_fragment_id`'):
            self.assertIn(want, text)
        self.assertNotIn('帶 `correction` 照錄使用者的原話', text)

    def test_a_correction_in_conversation_is_confirmed_before_it_is_staged(self):
        """The new version's reading is shown and confirmed before
        finish_document; a read version rejected since is a conflict too
        (final review 16, 7)."""
        text = (ROOT / 'skills' / 'plasma-document-analysis' / 'SKILL.md').read_text()
        flow = text[text.index('使用者指出某一項不對時'):]
        self.assertLess(flow.index('`reading`'), flow.index('`finish_document`'))
        self.assertIn('使用者確認後才', flow)
        self.assertIn('read_rejected', text)
        self.assertIn('`staged`', text)

    def test_a_revision_rereads_only_what_changed(self):
        """A revised document: only git diff's changed lines are read again,
        each affected rule is revised, deleted or kept, and an unhandled one
        is decided before finishing (design 2026-11-03 §5)."""
        text = (ROOT / 'skills' / 'plasma-document-analysis' / 'SKILL.md').read_text()
        for want in ('affected_rules', 'keep_rule', 'affected_unhandled', '只重讀'):
            self.assertIn(want, text)


class StoredConceptsGuidanceTest(unittest.TestCase):
    """Document analysis follows the stored-concepts protocol (Ophion spec
    2026-11-04): column rows declare stored concepts, a fragment carries its
    statement mode, a kept existing definition stands with its reason."""

    def text(self):
        return (ROOT / 'skills' / 'plasma-document-analysis' / 'SKILL.md').read_text()

    def test_a_column_is_noted_not_declared(self):
        """Ophion design 2026-11-11: the concept layer names no column. What
        a source says of a column is the column's words and storage notes;
        a business concept carries the column as a clue; the program builds
        each column's stored concept from the schema."""
        text = self.text()
        self.assertIn('column_words', text)
        self.assertIn('概念宣告不帶表或欄位', text)
        self.assertIn('儲存概念由程式從 schema 建立', text)
        self.assertIn('record_clue', text)
        self.assertNotIn('記在宣告的 `column`', text)
        self.assertNotIn('橋接由程式產生', text)
        self.assertNotIn('就是概念，寫成規則', text)
        self.assertIn('看它說了什麼，不看它寫成句子、清單、表格還是標題', text)

    def test_a_fragment_carries_its_statement_mode(self):
        text = self.text()
        self.assertIn('begin_fragment', text)
        self.assertIn('陳述模式', text)

    def test_existing_definitions_and_rule_checks_are_handled_before_finishing(self):
        text = self.text()
        for want in ('existing_definition', 'rule_checks', '不擋發布'):
            self.assertIn(want, text)

    def test_the_person_is_told_about_versioning_conflicts(self):
        self.assertIn('版本規則', self.text())


class CodeNotesTest(unittest.TestCase):
    """A user's agent reads code and writes what it read as notes at a
    stable path, uploaded as an ordinary document (Ophion spec 2026-11-04
    §8 rulings 1 and 5)."""

    def text(self):
        return (ROOT / 'skills' / 'plasma-code-notes' / 'SKILL.md').read_text()

    def test_it_is_for_code_and_database_objects(self):
        head = self.text().split('---')[1]
        self.assertIn('使用者要把程式碼、DDL 或資料庫物件的知識交給 Plasma 知識庫時使用', head)

    def test_notes_live_at_a_stable_path_and_go_through_document_analysis(self):
        text = self.text()
        for want in ('user-notes/', '同一路徑', '../plasma-document-analysis/SKILL.md'):
            self.assertIn(want, text)
        self.assertNotIn('submit_answer', text.replace('不用 `submit_answer`', ''))

    def test_each_point_keeps_statement_snippet_source_and_mode_together(self):
        text = self.text()
        for want in ('```', '出處', '路徑:行', '模式', '同一段', '推得'):
            self.assertIn(want, text)

    def test_the_code_criteria_are_there(self):
        text = self.text()
        for want in ('iota', 'now()', '預期', '樂觀鎖', 'migration', '前端', 'feature flag', '測試', 'DeletedAt'):
            self.assertIn(want, text)
