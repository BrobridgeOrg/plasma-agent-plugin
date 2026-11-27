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

    def test_the_sql_path_fills_from_facts_and_lists_what_it_inferred(self):
        """The agent fills a hole from what the knowledge and the schema
        say, and the person checks each inference (user ruling 2026-11-16);
        a compile with holes has no SQL."""
        text = self.compiled()
        for word in ('read_schema', 'answers', '核對', '代理推測：'):
            self.assertIn(word, text)
        self.assertNotIn('{{h', text)
        self.assertNotIn('{{h', self.reference('advanced.md'))

    def test_a_fill_rests_on_facts_and_names_its_compile(self):
        """Facts are looked for before a hole is filled, and a reply with
        holes has no SQL to copy names from."""
        text = self.compiled()
        facts = text.index('**先找事實。**')
        decide = text.index('**依事實決定怎麼補。**')
        send = text.index('以 `compile_query` 帶 `fills`')
        self.assertLess(facts, decide)
        self.assertLess(decide, send)
        self.assertIn('`compile_id`', text[send:send + 200])
        self.assertIn('不用把請求重送一次', text[send:send + 300])
        self.assertNotIn('照編譯出的 SQL', text)

    def test_what_the_facts_do_not_settle_is_asked(self):
        """Readings the facts cannot tell apart, a hole with no basis, and a
        relationship the data refuted are asked before they are filled (user
        ruling 2026-11-16)."""
        text = self.compiled()
        decide = text[text.index('**依事實決定怎麼補。**'):text.index('**補上。**')]
        for word in ('事實分不出來', '找不到任何依據', '`relation_refuted`', '一律先問'):
            self.assertIn(word, decide, word)
        refuted = text[text.index('- `relation_refuted`：'):]
        self.assertIn('使用者確認依它讀取後才補', refuted[:200])

    def test_inferences_are_listed_for_the_person_and_kept_only_once_checked(self):
        """A report gives the two lists with its result; a view is created
        after the person checked them; nothing inferred is written back to
        the knowledge before that (user ruling 2026-11-16)."""
        text = self.compiled()
        review = text[text.index('### 3.1 列給使用者核對'):text.index('## 4. 要建成 mview 時')]
        for word in ('**需要你核對的**', '**我自己決定的**', '核對完才建立', '還沒核對的推測不保存'):
            self.assertIn(word, review, word)
        checked = review.index('**說對的**')
        for tool in ('`submit_answer`', '`accept_relationship`', '`complete_relationship`'):
            self.assertNotIn(tool, text[text.index('## 3. 補洞'):text.index('### 3.1 列給使用者核對')], tool)
            self.assertGreater(review.index(tool), checked, tool)
        report = self.skill('plasma-report')
        for word in ('**需要你核對的**', '**我自己決定的**'):
            self.assertIn(word, report, word)
        self.assertIn('核對完才建立', (ROOT / 'skills' / 'plasma-create-view' / 'SKILL.md').read_text(encoding='utf-8'))

    def test_a_refused_query_is_told_apart_from_a_wrong_fragment(self):
        """Validation can fail away from the expressions, and an answer may be
        the faithful query rather than the optimized one."""
        text = self.compiled()
        self.assertIn('錯誤不在運算式', text)
        self.assertIn('`optimized`', text)

    def test_a_question_is_read_as_the_knowledge_it_needs(self):
        """The agent looks for what the question needs before it fills a
        request; what fully matches is used, what does not is read by the
        facts and listed as an inference."""
        text = self.compiled()
        need = text.index('需要哪些知識')
        match = text.index('完整符合')
        near = text.index('不完全符合')
        request = text.index('把問題拆成請求的各部分')
        self.assertLess(need, match)
        self.assertLess(match, near)
        self.assertLess(near, request)
        self.assertIn('記成一筆推測', text)

    def test_a_near_concept_leads_to_the_concepts_it_reads(self):
        """A concept that does not fully match is still an index: what it
        reads may, and a new concept is built of that."""
        text = self.compiled()
        self.assertIn('`dependencies`', text)
        self.assertIn('索引', text)

    def test_a_new_concept_is_built_of_blocks_and_returns_to_the_commit_area(self):
        text = self.compiled()
        define = text.index('`define`')
        fragment = text.index('寫**一個運算式**')
        self.assertLess(define, fragment, 'an expression is the last resort')
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
        self.assertIn('沒有 `keep_fills`', self.compiled())

    def test_a_stored_columns_derivation_is_read(self):
        """read_schema shows how a document says an existing column is
        computed, with what the data said of it; how it is submitted is the
        tool's own description (v0.37.0)."""
        lookup = self.reference('advanced.md')
        for word in ('`column_derivation`', '`expression`', '`verified`', '`unverified`', '`awaiting_choice`', '`refuted`',
                     '`reason`', '`unsupported_function`'):
            self.assertIn(word, lookup, word)

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
        column; the fill is one expression over the row t the hole names
        (expects.write, expects.t; mfg-full9: the rows frame was what failed);
        a failed validation is fixed, not asked (Q06 2026-10-28)."""
        report = self.compiled()
        for word in ('"ask"', 'expects.write', 'expects.t', 'validation', '母體'):
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
        """What the question asks of the compiled result (its names, a
        ratio, some columns or rows, a ROC year, a format, an order) is
        compile_query's outer, which reads report only, the compiled query
        unchanged, and is told; another calendar is knowledge, not a
        bucket; a ROC year in a condition becomes the Gregorian period
        (2026-10-29; Ophion outer 2026-11-27)."""
        report = self.skill('plasma-report') + self.compiled()
        for word in ('`outer`', 'FROM report', '原封不動', '民國', '農曆', 'bucket'):
            self.assertIn(word, report, word)
        self.assertNotIn('FROM (<編譯出的 sql', report, 'the wrap is the outer, not SQL copied around the compile')

    def test_search_is_the_first_of_two_levels(self):
        """A search lists names and short definitions only; the concept that
        looks like the one meant is expanded with definition or trace, where
        the notes on its columns and tables are (Ophion two-level search
        2026-11-27)."""
        for text in (self.compiled(), self.reference('advanced.md')):
            for word in ('第一層', '簡短定義', '`notes`'):
                self.assertIn(word, text, word)
            self.assertNotIn('另列這些字詞找到的附註', text, 'a search lists no note')
        self.assertIn('`compile_asks`', self.compiled())
        self.assertIn('`below`', self.compiled())

    def test_report_skill_reads_a_metric_whole_with_trace(self):
        """A search lists concepts and, finding none, the nearest names; a
        metric's knowledge is read whole by trace before the request is
        composed (Ophion agent reading load 2026-11-27)."""
        compiled = self.compiled()
        for word in ('`trace`', '`near`', '`concepts`'):
            self.assertIn(word, compiled, word)
        self.assertIn('`trace`', self.skill('plasma-report'))

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

    def test_the_three_layers_change_what_the_skills_say(self):
        """Ophion three layers (2026-11-19, 2026-11-20): a mapping is a
        record of a concept and its column, named by table and column; a
        consumer is told when the schema changed since the mapping; a
        member's several values are counted or chosen by the person; a
        compile_id names its request; a confirmation of a relationship is
        read at once."""
        remap = self.skill('plasma-remap')
        self.assertNotIn('`stored`', remap)
        self.assertNotIn('exp.derived.stored', remap)
        self.assertIn('對應紀錄', remap)
        self.assertIn('`schema_changed`', remap)
        self.assertIn('`schema_changed`', self.reference('advanced.md'))
        self.assertIn('plasma-remap', self.reference('advanced.md'))
        text = self.compiled()
        several = text[text.index('不只一個值'):]
        for word in ('`aggregate` 帶 `each` 為 true', '`select` 的 `each`', '只取哪一個'):
            self.assertIn(word, several[:300], word)
        self.assertIn('標示 `stored`', text)
        review = text[text.index('### 3.1 列給使用者核對'):]
        accepted = review.index('`accept_relationship`')
        self.assertIn('確認立即生效', review[accepted:accepted + 120])
        completed = review.index('`complete_relationship`')
        self.assertIn('發布後 Plasma 驗證', review[completed:completed + 120])

    def test_storage_batch2_changes_what_the_skills_say(self):
        """Ophion storage batch 2 (2026-11-24): a hole carries what the
        knowledge says of it; a row's value is no constant unless the person
        says so; fills accumulate per compile_id; a choice hole is chosen;
        group rules apply and are cancelled by scopes; definition gives the
        effective view and dimensions what to group by; what the person
        confirmed is kept with keep_fills or keep_agreed; mapping probes a
        column with profile_column when the task's protocol says so, and
        its conclusion waits as 資料推測; and the new pending items are
        closed. What a document states of groups, readings and tables is the
        product tools' to say (v0.37.0)."""
        text = self.compiled()
        for word in ('`gap_details`', '`related`', '`column_notes`', '`constant`', '`choose`', '`scopes`',
                     '`filled_from`', '送空字串 `""`', '`group_rules`', '`sensitive_columns`', '`effective`',
                     '`dimensions`', '`values.matches`'):
            self.assertIn(word, text, word)
        holes = text[text.index('## 3. 補洞'):text.index('### 3.1 列給使用者核對')]
        self.assertIn('每一列都是同一個值時', holes[holes.index('`constant`') - 120:])
        review = text[text.index('### 3.1 列給使用者核對'):text.index('## 4. 要建成 mview 時')]
        checked = review.index('**說對的**')
        for word in ('`keep_fills`', '`keep_agreed`', '`words`', '`lacked`', '`rests_on`', '`choice`'):
            self.assertGreater(review.index(word), checked, word)
        report = self.skill('plasma-report')
        for word in ('`keep_fills`', '`keep_agreed`', '`scopes`', '`column_notes`'):
            self.assertIn(word, report, word)
        lookup = self.reference('advanced.md')
        for word in ('`effective`', '`dimensions`'):
            self.assertIn(word, lookup, word)
        for name in ('plasma-document-analysis', 'plasma-remap'):
            text = self.skill(name)
            probe = text[text.index('協定說要抽樣探測某一欄'):]
            for word in ('`profile_column`', '`query:run`', '照協定交出結論', '資料推測'):
                self.assertIn(word, probe[:160], f'{name}: {word}')
        self.assertIn('`probe-conflict:`', self.skill('plasma-remap'))
        gaps = self.skill('plasma-fill-gaps')
        for word in ('`coverage:…`', '`answer-fill:…`', '`probe-conflict:…`', '`keep_fills`', '`keep_agreed`'):
            self.assertIn(word, gaps, word)
        notes = (ROOT / 'releases' / 'v0.36.0.md').read_text(encoding='utf-8')
        for word in ('feat/storage-batch2', 'feat/profile-column'):
            self.assertIn(word, notes, word)

    def test_no_experiment_connection_is_left(self):
        for name in ('plasma-document-analysis', 'plasma-fill-gaps'):
            text = self.skill(name)
            for stale in ('127.0.0.1', 'leibniz-', 'MCP 網址加上', '上傳面板', 'ssh -L'):
                self.assertNotIn(stale, text, f'{name} still says {stale!r}')
            self.assertIn('commit_page', text, name)

    def test_a_computation_gap_is_filled_where_it_was_declared(self):
        """A gap of a computation or a metric is filled on the document that
        declared it, after the source is looked up and before the person is
        asked; which fields it lacks is the item's own closes_with."""
        gaps = self.skill('plasma-fill-gaps')
        dag = gaps[gaps.index('（`dag:…`）'):]
        for want in ('先查原文有沒有寫', '宣告它的那份文件上補', '沒寫才照 `question` 問使用者', '根本原因'):
            self.assertIn(want, dag[:200], want)

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
        table's or a column's notes from read_schema: the published schema,
        not the engine's own catalog. read_schema has no calculated columns
        (Ophion 2262b5d5): what is computed from a column is a rule."""
        lookup = self.text('plasma-knowledge-query', 'references', 'advanced.md')
        for word in ('`overview`', '`database`', '`has_more`', '`rules`', '`next`', '`column`', '`notes`',
                     '`sources`', '`unit`', '`concepts`', '`usage`', '從表或欄位問起時',
                     '工具清單沒有 `overview` 時'):
            self.assertIn(word, lookup, word)
        self.assertNotIn('`calculated`', lookup)
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




class ReadingTest(unittest.TestCase):
    def test_how_the_document_is_read_is_the_agents(self):
        """How the agent reads is its own; what a passage is submitted as is
        the protocol's to say, and a line that states no knowledge stays
        uncovered without a record (Ophion D8)."""
        text = (ROOT / 'skills' / 'plasma-document-analysis' / 'SKILL.md').read_text()
        for want in ('怎麼讀、讀幾次、依什麼順序由你決定', '留著就是未覆蓋，不用記錄'):
            self.assertIn(want, text)
        self.assertNotIn('讀完全部內容才開始審查', text)


class RevisionGuidanceTest(unittest.TestCase):
    def text(self):
        return (ROOT / 'skills' / 'plasma-document-analysis' / 'SKILL.md').read_text()

    def test_a_correction_is_the_persons_words_and_its_reading_is_confirmed(self):
        """A person's correction of a product, in the commit area or
        published, is kept as an answer document that the new version
        cites; the new version's reading is shown to the person (Ophion
        read_translation, submit_answer)."""
        text = self.text()
        flow = text[text.index('使用者指出某一項不對時'):text.index('## 9. 交給使用者發布')]
        self.assertLess(flow.index('`submit_answer`'), flow.index('`reading`'))
        for want in ('照錄使用者的原話', '只改使用者說的部分', '貼給使用者確認', '請使用者決定', '`withdraw_mapping`'):
            self.assertIn(want, flow, want)

    def test_a_revision_rereads_what_changed_and_reconfirms_what_it_touched(self):
        """A revised document is its own upload: only the changed lines are
        read again, and each published product whose lines changed or went
        stays published until it is sent again, rewritten, kept or
        withdrawn (Ophion D13)."""
        text = self.text()
        revision = text[text.index('## 10. 文件改版'):]
        for want in ('`document_coverage`', '`diff`', '`to_reconfirm`', '自動沿用', '只重讀', '維持已發布',
                     '重送或保留', '改寫', '撤回', '請他決定'):
            self.assertIn(want, revision, want)
        self.assertIn('文件改版', (ROOT / 'skills' / 'plasma-code-notes' / 'SKILL.md').read_text())
        self.assertIn('`reconfirm:…`', (ROOT / 'skills' / 'plasma-fill-gaps' / 'SKILL.md').read_text())


# The tools Ophion removed with the stateless contract (Ophion
# docs/proposal-submission-contract.md D2, §4.4, §4.5): none is kept beside
# the new ones, so a skill that names one sends the agent to a tool that
# does not exist.
REMOVED_TOOLS = ('open_document', 'finish_document', 'resume_item', 'document_status',
                 'submit_rules', 'submit_node', 'submit_annotation', 'submit_backbone', 'submit_metadata',
                 'submit_group', 'record_clue', 'record_description', 'report_gap', 'declare_reference',
                 'identify_document', 'revise_rule', 'keep_rule', 'delete_knowledge',
                 'mapping_submit_mapping', 'mapping_submit_node', 'mapping_record_unmapped')

# One tool for each kind of product and of mapping (Ophion
# pkg/api/advanced/agentmcp/testdata/tools.golden.json at PR #260). Which
# kinds there are and what each takes is the MCP's to say: no skill names one.
KIND_TOOLS = ('submit_entity', 'submit_attribute', 'submit_relation', 'submit_condition', 'submit_metric',
              'submit_scope', 'submit_metric_group', 'submit_workspace_convention', 'submit_constant',
              'submit_code_meaning', 'submit_note', 'submit_column', 'submit_table',
              'submit_table_relationship', 'submit_column_derivation',
              'map_entity', 'map_rows', 'map_attribute', 'map_value', 'map_relation', 'map_computed',
              'map_unmapped')

# The other submit, mapping and withdraw tools of the same list, which the
# skills name where the flow calls them.
FLOW_TOOLS = ('submit_answer', 'withdraw_knowledge', 'withdraw_mapping', 'mapping_finish', 'mapping_read_source',
              'mapping_lookup_columns', 'mapping_lookup_relationships')

# Parameters of the product tools: how a product is written is read from the
# tool, not from a skill.
PRODUCT_PARAMETERS = ('source', 'statement', 'content_type', 'channel', 'lines', 'cells', 'quote', 'locator',
                      'unclear', 'resolutions', 'rule_check_answers', 'drop_changes', 'rule_id', 'base_version',
                      'declare', 'clues', 'value_type', 'unit', 'unit_quote', 'cardinality', 'through', 'realizes',
                      'basis', 'expected_document_revision', 'new_predicates', 'arg_values')

FLOW_SKILLS = ('plasma-document-analysis', 'plasma-fill-gaps', 'plasma-remap', 'plasma-code-notes')


def named(word, text):
    """Whether text names word as a whole identifier."""
    return re.search(r'(?<![A-Za-z0-9_])' + re.escape(word) + r'(?![A-Za-z0-9_])', text) is not None


class StatelessContractTest(unittest.TestCase):
    """The document flow follows Ophion's stateless contract (Ophion
    docs/proposal-submission-contract.md, PR #260): nothing is opened or
    finished, a product names its document by upload_id, and the skills
    tell the flow only (A9)."""

    def text(self, name='plasma-document-analysis'):
        return (ROOT / 'skills' / name / 'SKILL.md').read_text(encoding='utf-8')

    def shipped(self):
        """Every page an agent or a person installing the plugin reads."""
        pages = [*sorted((ROOT / 'skills').rglob('*.md')), ROOT / 'README.md', *sorted((ROOT / 'docs').glob('*.md')),
                 ROOT / '.opencode-plugin' / 'INSTALL.md']
        self.assertGreater(len(pages), 20)
        return {str(p.relative_to(ROOT)): p.read_text(encoding='utf-8') for p in pages}

    def test_no_page_names_a_removed_tool_or_a_work(self):
        """A9, A10: the removed tools and work_id are gone from every skill
        and the README; the release notes alone say which they were."""
        for path, text in self.shipped().items():
            for gone in (*REMOVED_TOOLS, 'work_id'):
                self.assertFalse(named(gone, text), f'{path} names {gone}')
        notes = (ROOT / 'releases' / 'v0.37.0.md').read_text(encoding='utf-8')
        for gone in (*REMOVED_TOOLS, 'work_id'):
            self.assertTrue(named(gone, notes), f'the release notes do not say {gone} is gone')

    def test_a_skill_names_only_tools_the_contract_has(self):
        """A submit, map, mapping or withdraw tool a skill names is one of
        the contract's list."""
        for path, text in self.shipped().items():
            for tool in re.findall(r'`((?:submit|map|mapping|withdraw)_[a-z_]+)`', text):
                self.assertIn(tool, (*KIND_TOOLS, *FLOW_TOOLS), f'{path} names {tool}')

    def test_what_a_product_is_is_read_from_the_mcp(self):
        """A9: a skill names no kind's tool and no parameter of a product;
        it sends the agent to the protocol and the tools' own descriptions."""
        for path, text in self.shipped().items():
            for tool in KIND_TOOLS:
                self.assertFalse(named(tool, text), f'{path} names {tool}')
        for name in FLOW_SKILLS:
            text = self.text(name)
            for parameter in PRODUCT_PARAMETERS:
                self.assertNotIn('`' + parameter + '`', text, f'{name} names the parameter {parameter}')
        for name in ('plasma-document-analysis', 'plasma-fill-gaps'):
            self.assertIn('`read_task`（`target` 為 `document`）的協定，與每個工具自己的說明', self.text(name), name)
        for name in ('plasma-document-analysis', 'plasma-remap'):
            self.assertIn('`read_task`（`target` 為 `task`）', self.text(name), name)
        for name in ('plasma-document-analysis', 'plasma-fill-gaps', 'plasma-remap'):
            self.assertIn('只寫流程，以及和使用者之間的事', self.text(name), name)

    def test_a_product_names_its_document_and_nothing_is_opened(self):
        """Every submission carries its document's upload_id and where the
        file states it; a refused one keeps nothing and is sent again; the
        sub-agents share nothing but each its own upload_id."""
        text = self.text()
        analyse = text[text.index('## 4. 分析一份文件'):text.index('## 5. 把問題問使用者')]
        for want in ('`read_document`', '`upload_id` 與出處', '當下進 Commit 區', '這一筆什麼都沒留下', '整筆重送',
                     '後提交的一方', '`document_coverage`'):
            self.assertIn(want, analyse, want)
        agents = text[text.index('## 3. 每份文件一個子代理'):text.index('## 4. 分析一份文件')]
        for want in ('`upload_id`', '同時', '一個子代理只處理一份'):
            self.assertIn(want, agents, want)

    def test_mapping_runs_when_the_person_says_so(self):
        """A7, D11: once the documents are analysed the person is asked
        whether to run the mapping, and told that a concept with no mapping
        is published as a gap; run_mapping is called after they agree."""
        text = self.text()
        mapping = text[text.index('## 7. 請使用者決定是否執行對應'):text.index('## 8. 請使用者確認翻譯')]
        ask = mapping.index('問使用者一次')
        agreed = mapping.index('使用者同意後')
        run = mapping.index('呼叫 `run_mapping`')
        self.assertLess(ask, agreed)
        self.assertLess(agreed, run)
        for want in ('先不執行也可以發布', '列為缺口', '`task.ready`', '`mapping_finish`', '`map_*`'):
            self.assertIn(want, mapping, want)
        publish = text[text.index('## 9. 交給使用者發布'):text.index('## 10. 文件改版')]
        self.assertIn('還沒執行對應', publish)

    def test_the_report_and_the_persons_rulings(self):
        """The agent reports what the document's products cover and what
        waits; a definition two documents keep both of, and a conflict of
        facts, wait for the person on the commit page, which publishes."""
        text = self.text()
        report = text[text.index('## 6. 回報'):text.index('## 7. 請使用者決定是否執行對應')]
        for want in ('覆蓋情況', '被擋下而沒能解決的提交', '等使用者在 Commit 頁裁決的衝突', '`pending_items`'):
            self.assertIn(want, report, want)
        publish = text[text.index('## 9. 交給使用者發布'):text.index('## 10. 文件改版')]
        for want in ('`setup.commit_page`', '發布由使用者在頁面上操作', '事實衝突', '裁決前只有牽涉的產物不發布',
                     '發布之後還能替同一份文件補產物'):
            self.assertIn(want, publish, want)

    def test_an_item_is_filled_on_the_document_that_states_it(self):
        """A pending item is completed by a product submitted on the
        document that states what it lacks, by its upload_id; the person's
        answer is a document of its own (Ophion pending_items)."""
        gaps = self.text('plasma-fill-gaps')
        fill = gaps[gaps.index('## 5. 補齊一個項目'):gaps.index('## 6. 使用者指正翻譯或對應')]
        for want in ('照這一項的 `closes_with`', '說明所缺內容的那份文件上提交缺的產物', '`upload_id`',
                     '再查一次 `pending_items`'):
            self.assertIn(want, fill, want)
        answer = gaps[gaps.index('## 4. 轉交使用者的回答'):gaps.index('## 5. 補齊一個項目')]
        for want in ('`submit_answer`', '使用者的原話', '`item_id`', '以行號引用回答'):
            self.assertIn(want, answer, want)

    def test_the_skills_name_no_tool_the_document_flow_dropped(self):
        # Ophion's document flow has no review, no fragment to open, end or
        # close, and no deferral tool: a submission carries its own source.
        for name in ('plasma-document-analysis', 'plasma-fill-gaps'):
            text = self.text(name)
            for gone in ('review_units', 'begin_fragment', 'close_fragment', 'defer_fragment', '`finish`', 'lost_products', 'retry_target', 'carried_excerpts', 'gap_fragment_id',
                         'fragment', 'resumed_for', 'still_declared', 'kept_in_fragment'):
                self.assertNotIn(gone, text, f'{name} names {gone}')

    def test_the_release_is_published_with_ophion(self):
        """D2: no old tool is kept, so the plugin and Ophion are released
        together and a client reads the tool list again."""
        notes = (ROOT / 'releases' / 'v0.37.0.md').read_text(encoding='utf-8')
        for want in ('不相容', 'Ophion PR #260', '同版一起發布', '重新取得工具清單'):
            self.assertIn(want, notes, want)


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

    def test_a_migration_that_rewrites_data_states_the_data(self):
        """History is a past design that no longer applies and nothing uses
        it (Ophion 2026-11-27); a migration that rewrites existing rows says
        what the data is now."""
        text = self.text()
        migration = text[text.index('- **migration**'):]
        self.assertIn('寫成資料現況', migration[:120])


class HistoryAndTableWordsTest(unittest.TestCase):
    def test_a_table_says_what_it_stores_and_history_is_read_when_asked(self):
        """A table's description is a note of kind table_words; a past
        design that no longer applies is read with history true and is used
        by no query (Ophion 2026-11-27)."""
        lookup = (ROOT / 'skills' / 'plasma-knowledge-query' / 'references' / 'advanced.md').read_text(encoding='utf-8')
        self.assertIn('`table_words`', lookup)
        history = lookup[lookup.index('`history` 為 true'):]
        self.assertIn('不參與查詢與 SQL', history[:80])


# Words of the test corpora (user 2026-11-27: no tool description and no
# example in the plugin carries what a test corpus knows). The corpora are
# MIMIC-IV, the manufacturing benchmark, the HIS corpus and the shop corpus;
# the plugin cites nothing of BIRD/Spider-Ent, so none of its words is listed.
# 判準三問:
# 1. Neither derived nor tuned: the list is the corpora's own vocabulary,
#    read from their documents: table and column names, domain nouns and
#    values only a corpus has, among them every one the examples carried
#    before 0.37.1. A corpus added later adds its words here.
# 2. It assumes a corpus shows through its distinctive words. A corpus rule
#    restated in generic words passes it and is caught in review. The shop
#    corpus is itself e-commerce, so its plain table names and nouns are
#    listed too, and the examples use a small library (loans, members,
#    branches, fines), as Ophion's tool descriptions do. "orders" is also
#    a verb ("a rule orders rows"): the table names are matched only where
#    they stand as identifiers (quoted, dotted, after FROM or JOIN).
# 3. A prose-heavy corpus with no SQL and one concept a file shows as its
#    domain nouns rather than identifiers: list those nouns when it is
#    added; its sentences stay the reviewer's.
CORPUS_WORDS = (
    # MIMIC-IV
    'admission', 'icustay', 'patient', 'hadm_id', 'subject_id', 'stay_id', 'itemid', 'labevents',
    'chartevents', 'icd_code', 'seq_num', 'mortality', 'mimic', '住院', '入院', '病人', '病患', '人次',
    # manufacturing benchmark
    'plant_id', 'work_order', 'bom_header', 'bom_component', 'lot_id', 'lot_genealogy', 'ar_invoice',
    'payment_allocation', 'shipment_allocation', 'supplier', 'uom_conversion', 'manufacturing', 'otif',
    'erp2', 'as400', 'de01', 'tw01', 'vn01', '製令', '料號', '工單', '批號', '良率', '工廠', '過帳', 'posted',
    # HIS
    'informix', 'opd_', 'nhi_', 'reg_no', '掛號', '病歷', '門診', '健保', '檢驗', '醫囑', '護理', '批價', '疾病',
    # shop
    'category_code', 'product_categories', 'invoice_seq', 'code_system', 'customer_tier', 'order_status',
    'deleted_flag', 'price_periods', 'v_open_orders', 'placed_on', 'customer_id', 'order_id', 'product_id',
    'invoice_id', '金卡', '免運', '杯類', '訂單', '客戶', '商品', '發票',
)
# What a word cannot say: a corpus's database name, the shop's plain table
# names where they stand as identifiers, its money columns in minor units
# and MIMIC's shifted years (2100-2199).
SHOP_TABLES = r'(?:orders|customers|order_lines|products|invoices)'
CORPUS_PATTERNS = (r'\bshop\b', rf'(?:[`"\'.\[]|\bfrom\s+|\bjoin\s+){SHOP_TABLES}\b',
                   rf'\b{SHOP_TABLES}(?=[.`"\'\]])', r'_minor\b', r'\b21\d\d\b')


class NeutralExamplesTest(unittest.TestCase):
    def shipped(self):
        """Every text an agent reads from the plugin: the skills, the
        manifests, the opencode files and the README."""
        paths = [*sorted((ROOT / 'skills').rglob('*.md')), ROOT / 'README.md',
                 *sorted((ROOT / '.claude-plugin').glob('*.json')), ROOT / '.codex-plugin' / 'plugin.json',
                 *sorted((ROOT / '.opencode-plugin').iterdir())]
        return {str(p.relative_to(ROOT)): p.read_text(encoding='utf-8') for p in paths}

    def test_no_text_carries_a_test_corpus(self):
        for name, text in self.shipped().items():
            lower = text.lower()
            for word in CORPUS_WORDS:
                self.assertNotIn(word, lower, f'{name}: {word}')
            for pattern in CORPUS_PATTERNS:
                found = re.search(pattern, lower)
                self.assertIsNone(found, f'{name}: {found and found.group(0)}')


if __name__ == '__main__':
    unittest.main()
