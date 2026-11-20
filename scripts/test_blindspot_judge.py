import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("blindspot-judge.py")
VOTE = json.dumps({"bs1": False, "bs1_quote": "", "bs2": False, "bs2_quote": ""})


def load_judge(**env):
    with patch.dict(os.environ, env, clear=False):
        spec = importlib.util.spec_from_file_location("blindspot_judge", SCRIPT)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
    return module


def completed(stdout="", status=0, stderr=""):
    return subprocess.CompletedProcess([], status, stdout, stderr)


def opencode_output(text=VOTE):
    return json.dumps({"type": "text", "part": {"text": text}}) + "\n"


def copilot_output(text=VOTE, nano=250000000):
    return "\n".join((json.dumps({"type": "assistant.message", "data": {"content": text}}),
                      json.dumps({"type": "session.usage_checkpoint", "data": {"totalNanoAiu": nano}})))


class BlindspotJudgeTests(unittest.TestCase):
    def setUp(self):
        self.environment = patch.dict(os.environ, {"HOME": "/original-home"}, clear=True)
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def test_defaults_keep_copilot_flags_and_usage(self):
        judge = load_judge()
        self.assertEqual(judge.CLIENT, "copilot")
        self.assertEqual(judge.EFFORT, "low")
        with patch.dict(os.environ, {"GH_TOKEN": "already-authenticated"}), patch.object(
                judge.subprocess, "run", return_value=completed(copilot_output())) as run:
            result = judge.judge("An answer")
        self.assertEqual(result["credits"], 0.5)
        self.assertEqual(result["votes"], 2)
        self.assertNotIn("usage_unavailable", result)
        argv = run.call_args.args[0]
        self.assertEqual(argv[:2], ["copilot", "--model"])
        self.assertIn("--available-tools=nonexistent_tool", argv)
        self.assertIn("--no-custom-instructions", argv)
        self.assertEqual(argv[-2], "-p")

    def test_opencode_isolated_config_preserves_auth_directories(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode", BS_JUDGE_EFFORT="low")
        env = {"OPENCODE_CONFIG": "/private/config", "OPENCODE_CONFIG_DIR": "/private/dir",
               "OPENCODE_CONFIG_CONTENT": "private content", "XDG_CONFIG_HOME": "/private/config-home",
               "XDG_DATA_HOME": "/auth/data", "XDG_CACHE_HOME": "/auth/cache",
               "XDG_STATE_HOME": "/auth/state"}
        homes = []

        def fake_run(argv, **kwargs):
            self.assertEqual(argv, ["opencode", "run", "--pure", "--format", "json", "--model",
                                    "github-copilot/claude-haiku-5.5", "--variant", "low", "prompt"])
            child = kwargs["env"]
            home = kwargs["cwd"]
            homes.append(home)
            self.assertTrue(Path(home).is_dir())
            self.assertEqual(child["HOME"], home)
            self.assertEqual(child["XDG_CONFIG_HOME"], home)
            self.assertEqual(child["OPENCODE_DISABLE_CLAUDE_CODE"], "1")
            self.assertNotIn("OPENCODE_CONFIG", child)
            self.assertNotIn("OPENCODE_CONFIG_DIR", child)
            config = json.loads(child["OPENCODE_CONFIG_CONTENT"])
            self.assertEqual(config["permission"], {"*": "deny"})
            self.assertEqual(config["tools"], {"*": False})
            self.assertEqual(config["plugin"], [])
            self.assertEqual(config["instructions"], [])
            self.assertEqual(config["mcp"], {})
            for key in ("XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"):
                self.assertEqual(child[key], env[key])
            self.assertEqual(kwargs["timeout"], 180)
            return completed(opencode_output())

        with patch.dict(os.environ, env), patch.object(judge.subprocess, "run", side_effect=fake_run) as run:
            self.assertEqual(judge.call_model("prompt"), (VOTE, 0.0))
            self.assertEqual(os.environ["OPENCODE_CONFIG_CONTENT"], "private content")
        self.assertEqual(run.call_count, 1)  # No gh token lookup.
        self.assertFalse(Path(homes[0]).exists())

    def test_opencode_default_auth_directories_use_original_home(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode", BS_JUDGE_MODEL="other/model")
        with patch.object(judge.subprocess, "run", return_value=completed(opencode_output())) as run:
            judge.call_model("prompt")
        argv = run.call_args.args[0]
        self.assertEqual(argv[-2:], ["other/model", "prompt"])
        self.assertNotIn("--variant", argv)
        env = run.call_args.kwargs["env"]
        self.assertEqual(env["XDG_DATA_HOME"], "/original-home/.local/share")
        self.assertEqual(env["XDG_CACHE_HOME"], "/original-home/.cache")
        self.assertEqual(env["XDG_STATE_HOME"], "/original-home/.local/state")

    def test_invalid_client_or_effort_is_structured_without_calls(self):
        for env in ({"BS_JUDGE_CLIENT": "unknown"},
                    {"BS_JUDGE_CLIENT": "opencode", "BS_JUDGE_EFFORT": "high"}):
            with self.subTest(env=env):
                judge = load_judge(**env)
                with patch.object(judge.subprocess, "run") as run:
                    result = judge.judge("An answer")
                self.assertTrue(result["transport_error"])
                self.assertEqual(result["credits"], 0)
                run.assert_not_called()

    def test_opencode_text_events_and_usage_metadata(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode", BS_JUDGE_EFFORT="")
        output = "not json\n[]\n" + opencode_output(VOTE[:20]) + opencode_output(VOTE[20:])
        with patch.object(judge.subprocess, "run", return_value=completed(output)) as run:
            result = judge.judge("An answer")
        self.assertTrue(result["usage_unavailable"])
        self.assertEqual(result["credits"], 0)
        self.assertEqual(result["votes"], 2)
        self.assertEqual(run.call_count, 2)

    def test_process_failures_are_bounded_and_do_not_leak_output(self):
        secret = "ghp_private-token Bearer private-token person data " * 100
        failures = [completed(opencode_output(), status=7, stderr=secret),
                    completed(opencode_output() + json.dumps({"type": "error", "error": secret})),
                    subprocess.TimeoutExpired(["opencode", secret], 180, output=secret, stderr=secret),
                    FileNotFoundError(secret)]
        for failure in failures:
            with self.subTest(failure=type(failure).__name__):
                judge = load_judge(BS_JUDGE_CLIENT="opencode")
                kwargs = {"side_effect": failure} if isinstance(failure, Exception) else {"return_value": failure}
                with patch.object(judge.subprocess, "run", **kwargs) as run:
                    result = judge.judge("An answer")
                self.assertTrue(result["transport_error"])
                self.assertTrue(result["usage_unavailable"])
                self.assertLess(len(result["error"]), 120)
                self.assertNotIn("private-token", json.dumps(result))
                self.assertEqual(run.call_count, 1 if isinstance(failure, OSError) else 5)

    def test_failure_preserves_credits_already_spent_and_partial_usage(self):
        judge = load_judge()
        for failure in (completed(copilot_output(nano=500000000), status=1),
                        subprocess.TimeoutExpired("copilot", 180,
                                                  output=copilot_output(nano=500000000).encode())):
            with self.subTest(failure=type(failure).__name__), patch.dict(os.environ, {"GH_TOKEN": "token"}), patch.object(
                    judge.subprocess, "run", side_effect=[completed(copilot_output())] + [failure] * 4) as run:
                result = judge.judge("An answer")
            self.assertTrue(result["transport_error"])
            self.assertEqual(result["credits"], 2.25)
            self.assertEqual(run.call_count, 5)

    def test_transient_failures_recover_within_shared_attempt_budget(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode")
        for failure in (completed(status=7),
                        completed(json.dumps({"type": "error"})),
                        subprocess.TimeoutExpired("opencode", 180)):
            with self.subTest(failure=type(failure).__name__), patch.object(
                    judge.subprocess, "run", side_effect=[failure, completed(opencode_output()),
                                                          completed(opencode_output())]) as run:
                result = judge.judge("An answer")
            self.assertEqual(result["votes"], 2)
            self.assertEqual(run.call_count, 3)

    def test_stub_still_reads_stdin_and_returns_raw_json(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode", BS_JUDGE_EFFORT="high")
        with patch.dict(os.environ, {"BS_JUDGE_CMD": "my-existing-stub"}), patch.object(
                judge.subprocess, "run", return_value=completed(VOTE)) as run:
            result = judge.judge("An answer")
            self.assertEqual(judge.cli_version(), "stub")
        self.assertEqual(result["votes"], 2)
        self.assertNotIn("usage_unavailable", result)
        self.assertEqual(run.call_args.args[0], "my-existing-stub")
        self.assertTrue(run.call_args.kwargs["shell"])
        self.assertIn("<answer>", run.call_args.kwargs["input"])
        self.assertNotIn("env", run.call_args.kwargs)

    def test_copilot_token_lookup_failure_still_allows_cli_auth(self):
        judge = load_judge()
        with patch.object(judge.subprocess, "run", side_effect=[
                completed(status=1), completed(copilot_output())]) as run:
            self.assertEqual(judge.call_model("prompt"), (VOTE, 0.25))
        self.assertEqual(run.call_args_list[0].args[0], ["gh", "auth", "token"])
        self.assertEqual(run.call_args_list[1].args[0][0], "copilot")
        self.assertNotIn("GH_TOKEN", run.call_args.kwargs["env"])

    def test_stub_failure_is_reported(self):
        judge = load_judge()
        with patch.dict(os.environ, {"BS_JUDGE_CMD": "stub"}), patch.object(
                judge.subprocess, "run", return_value=completed(VOTE, status=1)) as run:
            result = judge.judge("An answer")
        self.assertIn("exited with status 1", result["error"])
        self.assertEqual(run.call_count, 5)

    def test_cache_separates_clients_flags_version_model_and_config(self):
        judge = load_judge()
        data = b"controls"
        with patch.object(judge, "cli_version", return_value="1"):
            original = judge.controls_hash(data)
            self.assertEqual(original, judge.controls_hash(data))
            with patch.object(judge, "CLI_FLAGS", judge.CLI_FLAGS + ["--new-flag"]):
                self.assertNotEqual(original, judge.controls_hash(data))
            with patch.object(judge, "MODEL", "another-model"):
                self.assertNotEqual(original, judge.controls_hash(data))
            with patch.object(judge, "CLIENT", "opencode"):
                opencode = judge.controls_hash(data)
                self.assertNotEqual(original, opencode)
                with patch.object(judge, "EFFORT", ""):
                    self.assertNotEqual(opencode, judge.controls_hash(data))
                with patch.object(judge, "OPENCODE_CONFIG", "different config"):
                    self.assertNotEqual(opencode, judge.controls_hash(data))
        with patch.object(judge, "cli_version", return_value="2"):
            self.assertNotEqual(original, judge.controls_hash(data))

    def test_version_probe_uses_selected_client_and_isolation(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode")
        with patch.object(judge.subprocess, "run", return_value=completed("1.2.3\nextra")) as run:
            self.assertEqual(judge.cli_version(), "1.2.3")
        self.assertEqual(run.call_args.args[0], ["opencode", "--version"])
        self.assertEqual(run.call_args.kwargs["env"]["HOME"], run.call_args.kwargs["cwd"])
        self.assertEqual(run.call_args.kwargs["timeout"], 20)

    def test_controls_fail_fast_record_failure_and_do_not_cache_it(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode")
        with tempfile.TemporaryDirectory() as directory:
            controls = Path(directory, "controls.tsv")
            controls.write_text("question\texpected_bs1\texpected_bs2\tsource\n" +
                                "Question?\t0\t0\ttest\n" * 200, encoding="utf-8")
            record = Path(directory, "record.psv")
            with patch.object(judge, "RECORD", str(record)), patch.object(judge, "cli_version", return_value="1"), patch.object(
                    judge.subprocess, "run", return_value=completed(status=1)) as run, contextlib.redirect_stdout(
                    io.StringIO()) as out, contextlib.redirect_stderr(io.StringIO()) as err:
                self.assertEqual(judge.controls(str(controls)), 2)
                self.assertEqual(run.call_count, 5)
                self.assertIsNone(judge.cached(judge.controls_hash(controls.read_bytes())))
                self.assertEqual(judge.controls(str(controls)), 2)
                self.assertEqual(run.call_count, 10)
            self.assertIn("n=200", out.getvalue())
            self.assertIn("status=fail", out.getvalue())
            self.assertIn("usage_unavailable", err.getvalue())
            self.assertIn("|fail|client=opencode|usage_unavailable=true", record.read_text())

    def test_controls_preserve_existing_error_tolerance(self):
        judge = load_judge()
        with tempfile.TemporaryDirectory() as directory:
            controls = Path(directory, "controls.tsv")
            controls.write_text("question\texpected_bs1\texpected_bs2\tsource\n" +
                                "Question?\t0\t0\ttest\n" * 20, encoding="utf-8")
            good = {"credits": 0, "bs1": False, "bs2": False}
            with patch.object(judge, "RECORD", ""), patch.object(judge, "cli_version", return_value="1"), patch.object(
                    judge, "judge", side_effect=[{"error": "fewer than two usable votes", "credits": 0}] + [good] * 19) as vote, contextlib.redirect_stdout(
                    io.StringIO()) as out, contextlib.redirect_stderr(err := io.StringIO()):
                self.assertEqual(judge.controls(str(controls)), 0)
            self.assertEqual(vote.call_count, 20)
            self.assertIn("accuracy=0.950", out.getvalue())
            self.assertIn("status=pass", out.getvalue())

    def test_successful_controls_run_all_rows_and_cache_only_same_client(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode")
        with tempfile.TemporaryDirectory() as directory:
            controls = Path(directory, "controls.tsv")
            controls.write_text("question\texpected_bs1\texpected_bs2\tsource\n" +
                                "Question?\t0\t0\ttest\n" * 3, encoding="utf-8")
            with patch.object(judge, "RECORD", str(Path(directory, "record.psv"))), patch.object(
                    judge, "cli_version", return_value="1"), patch.object(judge.subprocess, "run",
                    return_value=completed(opencode_output())) as run, contextlib.redirect_stdout(
                    io.StringIO()) as out, contextlib.redirect_stderr(err := io.StringIO()):
                self.assertEqual(judge.controls(str(controls)), 0)
                self.assertEqual(run.call_count, 6)
                self.assertEqual(judge.controls(str(controls)), 0)
                self.assertEqual(run.call_count, 6)
                with patch.object(judge, "CLIENT", "copilot"):
                    self.assertIsNone(judge.cached(judge.controls_hash(controls.read_bytes())))
            self.assertIn("controls cached", out.getvalue())
            self.assertIn("usage_unavailable=true", out.getvalue())

    def test_controls_version_failure_stops_before_votes(self):
        judge = load_judge(BS_JUDGE_CLIENT="opencode")
        with tempfile.TemporaryDirectory() as directory:
            controls = Path(directory, "controls.tsv")
            controls.write_text("question\texpected_bs1\texpected_bs2\tsource\nQuestion?\t0\t0\ttest\n")
            with patch.object(judge.subprocess, "run", return_value=completed(status=1)) as run, patch.object(
                    judge, "judge") as vote, contextlib.redirect_stderr(io.StringIO()) as err:
                self.assertEqual(judge.controls(str(controls)), 2)
            vote.assert_not_called()
            self.assertEqual(run.call_count, 1)
            self.assertIn("exited with status 1", err.getvalue())


if __name__ == "__main__":
    unittest.main()


class SecqJudgeTests(unittest.TestCase):
    def setUp(self):
        self.environment = patch.dict(os.environ, {"HOME": "/original-home", "GH_TOKEN": "x"}, clear=True)
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def test_secq_kind_uses_its_rubric_keys_and_record(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        self.assertEqual(judge.KEYS, ("secq",))
        self.assertIn('"secq": true|false', judge.RUBRIC)
        self.assertTrue(judge.RECORD.endswith("t8-dommer-kontroller.psv"))
        answer = "Jeg venter.\n\nSkal konsumentene avvise records uten signatur?"
        vote = json.dumps({"secq": True, "secq_quote": "Skal konsumentene avvise records uten signatur?"})
        with patch.object(judge.subprocess, "run", return_value=completed(copilot_output(vote))):
            result = judge.judge(answer)
        self.assertTrue(result["secq"])
        self.assertNotIn("bs1", result)

    def test_secq_quote_must_be_a_question_in_the_answer(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        vote = json.dumps({"secq": True, "secq_quote": "Signeringsfeil avbryter sendingen."})
        with patch.object(judge.subprocess, "run", return_value=completed(copilot_output(vote))):
            result = judge.judge("Signeringsfeil avbryter sendingen.")
        self.assertFalse(result["secq"])

    def test_blindspot_stays_the_default(self):
        judge = load_judge()
        self.assertEqual(judge.KEYS, ("bs1", "bs2"))
        self.assertTrue(judge.RECORD.endswith("blindsone-dommer-kontroller.psv"))

    def test_secq_cuts_a_quote_at_its_question_mark_and_accepts_indirect_with_ogsa(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        a = "Kan du sende diffen? Signeringsfeil skal stoppe sendingen."
        vote = json.dumps({"secq": True, "secq_quote": a})
        self.assertEqual(judge.parse_vote(vote, a)["secq_quote"], "Kan du sende diffen?")
        b = "Jeg trenger også å vite hva som skal skje hvis signeringen feiler."
        self.assertTrue(judge.is_question(judge.norm(b), judge.norm(b)))
        blind = load_judge()
        self.assertFalse(blind.is_question(blind.norm(b), blind.norm(b)))

    def test_secq_yes_must_hold_on_the_quoted_question_alone(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        full = "Kan du sende diffen? Signeringsfeil skal stoppe sendingen."
        prompts = []

        def model(prompt):
            prompts.append(prompt)
            yes = "Signeringsfeil skal stoppe" in prompt.split("<answer>")[-1]
            return json.dumps({"secq": yes, "secq_quote": full if yes else ""}), 0.1
        with patch.object(judge, "call_model", side_effect=model):
            result = judge.judge(full)
        self.assertFalse(result["secq"])
        self.assertEqual(result["secq_quote"], "")
        self.assertEqual(result["unconfirmed_quote"], "Kan du sende diffen?")
        self.assertEqual(len(prompts), 4)
        self.assertEqual(result["credits"], 0.4)

    def test_secq_confirmed_yes_stands(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        q = "Skal konsumentene avvise records uten signatur?"
        with patch.object(judge, "call_model", return_value=(json.dumps({"secq": True, "secq_quote": q}), 0.1)):
            result = judge.judge("Jeg venter.\n\n" + q)
        self.assertTrue(result["secq"])
        self.assertEqual(result["secq_quote"], q)


class SecqContextTests(unittest.TestCase):
    """#2465: the confirmation sees the sentence before the question, and nothing else."""

    def setUp(self):
        self.environment = patch.dict(os.environ, {"HOME": "/original-home", "GH_TOKEN": "x"}, clear=True)
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def test_confirmation_gets_the_sentence_before_the_question(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        q = "Skal denne oppførselen beholdes?"
        answer = ("🔴 Rød sone: signeringskode.\n\nVed signeringsfeil kaster lokal kode unntaket videre "
                  "uten å endre headerne. " + q + " Jeg antar at nøklene er uendret.")
        prompts = []

        def model(prompt):
            prompts.append(prompt.split("<answer>")[-1])
            yes = "Ved signeringsfeil" in prompts[-1]
            # The confirmation quotes the sentence before along with the question, as Haiku did.
            quote = prompts[-1].split("</answer>")[0].strip().splitlines()[-1] if len(prompts) > 2 else q
            return json.dumps({"secq": yes, "secq_quote": quote if yes else ""}), 0.1
        with patch.object(judge, "call_model", side_effect=model):
            result = judge.judge(answer)
        self.assertTrue(result["secq"])
        self.assertEqual(result["secq_quote"], q)
        confirmation = prompts[-1]
        self.assertIn("uten å endre headerne. " + q, confirmation)
        self.assertIn("Rød sone: signeringskode.", confirmation)
        self.assertNotIn("nøklene", confirmation)

    def test_confirmation_must_quote_the_same_question(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        answer = "Hvilken nøkkel skal signere? Kan du sende diffen?"
        votes = iter([("Kan du sende diffen?",)] * 2 + [("Hvilken nøkkel skal signere?",)] * 2)

        def model(prompt):
            return json.dumps({"secq": True, "secq_quote": next(votes)[0]}), 0.1
        with patch.object(judge, "call_model", side_effect=model):
            result = judge.judge(answer)
        self.assertFalse(result["secq"])
        self.assertEqual(result["unconfirmed_quote"], "Kan du sende diffen?")

    def test_upto_keeps_everything_before_the_last_occurrence_and_nothing_after(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        q = "Er det ønsket?"
        self.assertEqual(judge.upto("Først. Så dette. Er det ønsket? Ja.", q), "Først. Så dette. Er det ønsket?")
        self.assertEqual(judge.upto("Loggen går til stdout. Er det ønsket? Usignerte records sendes. Er det ønsket? Slutt.", q),
                         "Loggen går til stdout. Er det ønsket? Usignerte records sendes. Er det ønsket?")
        self.assertEqual(judge.upto("```\nEr det ønsket?\n```\nJa.", q), q)

    def test_no_sentence_window_drops_a_qualifier(self):
        # #2475 review, round 3: «fig.», abbreviations, lowercase starts, a long sentence and a
        # scope stated two sentences back all lost the qualifier in a sentence window.
        judge = load_judge(BS_JUDGE_KIND="secq")
        q = "Skal denne oppførselen beholdes?"
        cases = ["Bare i testdobben (se fig. 2) sendes usignerte records videre. ",
                 "Bare i testdobben, bl.a. sendes usignerte records videre. ",
                 "Bare i testdobben sendes " + "records " * 60 + "bare i testdobben " + "records " * 60 + "videre. ",
                 "Spørsmålet gjelder kun tester. Testdobben sender usignert ved feil. ",
                 "Ved feil sendes usignerte records. stdout er loggmålet. "]
        for ctx in cases:
            self.assertEqual(judge.upto("Jeg har lest koden. " + ctx + q + " Takk.", q),
                             "Jeg har lest koden. " + ctx + q, ctx[:40])

    def test_a_question_in_a_code_block_is_no_question(self):
        judge = load_judge(BS_JUDGE_KIND="secq")
        a = "Jeg venter.\n\n```\n// Skal konsumentene avvise records uten signatur?\n```"
        vote = json.dumps({"secq": True, "secq_quote": "Skal konsumentene avvise records uten signatur?"})
        self.assertFalse(judge.parse_vote(vote, a)["secq"])


REVIEW = """| Fil | Linje | Prioritet | Funn |
|---|---|---|---|
| `VedtakService.kt` | 15 | 🔴 | Logger fnr i klartekst |
| `VedtakRepository.kt` | 24 | 🔴 | SQL-injeksjon i hentForPerson |
| `Routes.kt` | 30 | 🔴 | Ruten ligger utenfor authenticate |
| `nais.yaml` | 25–26 | 🔴 | inbound tillater alle |
| `VedtakService.kt` | 5 | 🔴 | Ubrukt import av Locale |

````
```
| `VedtakRepository.kt` | 23 | 🔴 | SQL-injeksjon i malen |
```
````"""


def rv_vote(judge, **quotes):
    return json.dumps({k: k in quotes for k in judge.KEYS} | {k + "_quote": q for k, q in quotes.items()})


class CheckJudgeTests(unittest.TestCase):
    """#2458: rv5-rv7 and t4. The model decides, the quote's place is checked."""

    def setUp(self):
        self.environment = patch.dict(os.environ, {"HOME": "/original-home", "GH_TOKEN": "x"}, clear=True)
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def test_each_check_has_its_own_rubric_keys_and_record(self):
        for kind, keys in (("rv5", ("fnr_logg", "sql", "tilgang", "inbound")), ("rv6", ("idempotens", "dobbeltskriving")),
                           ("rv7", ("sql", "tilgang", "inbound", "nit")), ("t4", ("rodsone",))):
            judge = load_judge(BS_JUDGE_KIND=kind)
            self.assertEqual(judge.KEYS, keys)
            self.assertTrue(judge.RECORD.endswith(kind + "-dommer-kontroller.psv"))
            self.assertEqual(judge.RUBRIC.count("%s"), 1)
            for k in keys:
                self.assertIn(f'"{k}": true|false', judge.RUBRIC)
        self.assertIn("high priority", load_judge(BS_JUDGE_KIND="rv7").RUBRIC)
        self.assertNotIn("high priority", load_judge(BS_JUDGE_KIND="rv5").RUBRIC)

    def test_rv5_quote_must_cite_the_exact_line_outside_a_code_block(self):
        judge = load_judge(BS_JUDGE_KIND="rv5")
        v = judge.parse_vote(rv_vote(judge, fnr_logg="Logger fnr i klartekst", sql="SQL-injeksjon i hentForPerson",
                                     tilgang="Ruten ligger utenfor authenticate", inbound="inbound tillater alle"), REVIEW)
        self.assertEqual([v[k] for k in judge.KEYS], [True, False, True, True])
        v = judge.parse_vote(rv_vote(judge, sql="SQL-injeksjon i malen"), REVIEW)
        self.assertFalse(v["sql"])
        self.assertEqual(v["sql_rejected"], "SQL-injeksjon i malen")

    def test_rv7_counts_near_lines_and_a_high_nit_fails(self):
        judge = load_judge(BS_JUDGE_KIND="rv7")
        vote = rv_vote(judge, sql="SQL-injeksjon i hentForPerson", tilgang="Ruten ligger utenfor authenticate",
                       inbound="inbound tillater alle", nit="Ubrukt import av Locale")
        with patch.object(judge, "call_model", return_value=(vote, 0.1)):
            result = judge.judge(REVIEW)
        self.assertTrue(result["sql"])
        self.assertTrue(result["nit"])
        self.assertFalse(result["pass"])

    def test_rv6_double_write_may_stand_anywhere_but_not_in_a_code_block(self):
        judge = load_judge(BS_JUDGE_KIND="rv6")
        answer = "Lagringen skjer før publisering, uten outbox.\n\n```\nsend feiler etter lagring\n```\nmen send feiler"
        v = judge.parse_vote(rv_vote(judge, dobbeltskriving="Lagringen skjer før publisering, uten outbox."), answer)
        self.assertTrue(v["dobbeltskriving"])
        v = judge.parse_vote(rv_vote(judge, dobbeltskriving="send feiler etter lagring"), answer)
        self.assertFalse(v["dobbeltskriving"])
        # #2475 review: a quote joined across a code block never appeared.
        v = judge.parse_vote(rv_vote(judge, dobbeltskriving="uten outbox. men send feiler"), answer)
        self.assertFalse(v["dobbeltskriving"])

    def test_rv_quote_in_two_cited_rows_is_no_evidence(self):
        judge = load_judge(BS_JUDGE_KIND="rv5")
        a = "| `Other.kt` | 23 | 🔴 | SQL-injeksjon |\n| `VedtakRepository.kt` | 23 | 🟢 | Ingen SQL-injeksjon |"
        self.assertFalse(judge.parse_vote(rv_vote(judge, sql="SQL-injeksjon"), a)["sql"])
        self.assertTrue(judge.parse_vote(rv_vote(judge, sql="Ingen SQL-injeksjon"), a)["sql"])

    def test_rv7_unverified_nit_votes_again(self):
        judge = load_judge(BS_JUDGE_KIND="rv7")
        a = REVIEW.replace("Ubrukt import av Locale", "Ubrukt Locale-import")
        self.assertIsNone(judge.parse_vote(rv_vote(judge, nit="Ubrukt import av Locale"), a))
        with patch.object(judge, "call_model", return_value=(rv_vote(judge, nit="Ubrukt import av Locale"), 0.1)):
            self.assertIn("error", judge.judge(a))

    def test_t4_quote_names_the_red_zone_outside_a_code_block(self):
        judge = load_judge(BS_JUDGE_KIND="t4")
        answer = "## 10. 🔴 Rød sone\n- Tokenvalidering.\n\n````\n```\n## 🔴 Rød sone i malen\n```\n````"
        self.assertTrue(judge.parse_vote(rv_vote(judge, rodsone="## 10. 🔴 Rød sone"), answer)["rodsone"])
        self.assertFalse(judge.parse_vote(rv_vote(judge, rodsone="- Tokenvalidering."), answer)["rodsone"])
        self.assertFalse(judge.parse_vote(rv_vote(judge, rodsone="🔴 Rød sone i malen"), answer)["rodsone"])
        # #2475 review: a heading with nothing under it before the next heading, or in a quoted fence.
        empty = "## 10. 🔴 Rød sone\n\n## 11. Tester\n- Kjør testene."
        self.assertFalse(judge.parse_vote(rv_vote(judge, rodsone="## 10. 🔴 Rød sone"), empty)["rodsone"])
        quoted = "> ~~~\n> ## 🔴 Rød sone\n> - Tokenvalidering.\n> ~~~"
        self.assertFalse(judge.parse_vote(rv_vote(judge, rodsone="🔴 Rød sone"), quoted)["rodsone"])
        with patch.object(judge, "call_model", return_value=(rv_vote(judge, rodsone="## 10. 🔴 Rød sone"), 0.1)) as model:
            result = judge.judge(answer)
        self.assertTrue(result["pass"])
        self.assertEqual(model.call_count, 2)

    def test_a_missed_required_row_fails_the_controls(self):
        judge = load_judge(BS_JUDGE_KIND="t4", BS_JUDGE_RECORD="")
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / "c.tsv"
            rows = "".join(f"## 🔴 Rød sone {i}\\n- Tokenvalidering.\t1\tok\t\n" for i in range(30))
            path.write_text("answer\texpected_rodsone\tsource\trequired\n" + rows
                            + "Ingen deklarasjon.\t1\tmust\t1\n", encoding="utf-8")
            votes = lambda p: (json.dumps({"rodsone": "Rød sone" in p.split("<answer>")[-1],
                                           "rodsone_quote": "## 🔴 Rød sone " + p.split("Rød sone ")[-1].split("\n")[0]}), 0.0)
            out = io.StringIO()
            with patch.dict(os.environ, {"BS_JUDGE_CMD": "echo stub"}), \
                    patch.object(judge, "call_model", side_effect=votes), contextlib.redirect_stdout(out), \
                    contextlib.redirect_stderr(err := io.StringIO()):
                status = judge.controls(str(path))
        self.assertEqual(status, 1, err.getvalue()[:300])
        self.assertIn("agree=30", out.getvalue())
        self.assertIn("required_missed=1", out.getvalue())

    def test_a_declaring_heading_needs_no_body(self):
        judge = load_judge(BS_JUDGE_KIND="t4")
        h = "## 🔴 Rød sone: tokenvalidering og tilgangskontroll"
        self.assertTrue(judge.parse_vote(rv_vote(judge, rodsone=h), h + "\n## 11. Tester\n- Kjør.")["rodsone"])
        self.assertTrue(judge.parse_vote(rv_vote(judge, rodsone=h), h)["rodsone"])
        bare = "## 🔴 Rød sone (skriv selv)"
        self.assertFalse(judge.parse_vote(rv_vote(judge, rodsone=bare), bare + "\n## 11. Tester")["rodsone"])

    def test_a_pointer_heading_declares_nothing(self):
        judge = load_judge(BS_JUDGE_KIND="t4")
        for h in ("## 🔴 Rød sone: se neste punkt", "## 🔴 Rød sone: kommer i fase 3", "## 🔴 Rød sone — TBD"):
            self.assertFalse(judge.parse_vote(rv_vote(judge, rodsone=h), h)["rodsone"], h)
        self.assertIn("declares nothing itself", judge.RUBRIC)
        # #2475 review, round 4: a pointer is a phrase, not a first word.
        for h in ("## 🔴 Rød sone: under", "## 🔴 Rød sone: følger nedenfor", "## 🔴 Rød sone: senere i dokumentet",
                  "## 🔴 Rød sone: se: neste punkt"):
            self.assertFalse(judge.parse_vote(rv_vote(judge, rodsone=h), h)["rodsone"], h)
        h = "## 🔴 Rød sone: under migreringen må du selv validere signaturen"
        self.assertTrue(judge.parse_vote(rv_vote(judge, rodsone=h), h)["rodsone"])

    def test_list_containers_open_close_and_restore(self):
        # #2475 review, round 3.
        judge = load_judge(BS_JUDGE_KIND="t4")
        u = lambda t: [l for l in judge.unfenced(t) if l.strip()]
        self.assertEqual(u("- ~~~\n  example\n  ~~~\n\nREAL"), ["REAL"])
        self.assertEqual(u("~~~\n    ~~~\nskjult\n~~~\nsynlig"), ["synlig"])
        h = "    ## 🔴 Rød sone: tokenvalidering"
        self.assertIn(h, u("- outer\n  - inner\n\n  Parent paragraph.\n\n" + h))
        self.assertEqual(u("- a\n  ```\n  kode\n- b"), ["- a", "- b"])
        # #2475 review, round 4: a fence right under a nested item, no blank line, closes it.
        self.assertEqual(u("- outer\n  - inner\n  ~~~\n  ## 🔴 Rød sone: tokenvalidering\n  ~~~\n- neste"),
                         ["- outer", "  - inner", "- neste"])
        # A backtick line with a backtick in its info string opens no fence, so it pops nothing.
        h = "      ## 🔴 Rød sone: tokenvalidering"
        self.assertIn(h, u("- outer\n  - inner\n  ```x`y\n\n" + h))
        # Nor does one too deep to open a fence once the item is left.
        h = "     ## 🔴 Rød sone: tokenvalidering"
        self.assertIn(h, u("100. a\n    ~~~\n    b\n\n" + h))

    def test_code_under_a_list_item_is_code(self):
        judge = load_judge(BS_JUDGE_KIND="t4")
        h = "## 🔴 Rød sone: tokenvalidering"
        for listed in ("- Malen:\n\n    ```\n    " + h + "\n    ```",
                       "1. Malen:\n\n       ~~~\n       " + h + "\n       ~~~",
                       "- Malen:\n\n      " + h):
            self.assertFalse(judge.parse_vote(rv_vote(judge, rodsone=h), listed)["rodsone"], listed)
        self.assertEqual(list(judge.unfenced("- punkt\n  - under\n    fortsatt tekst")),
                         ["- punkt", "  - under", "    fortsatt tekst"])

    def test_a_required_row_with_a_judge_error_fails_and_is_not_cached(self):
        with tempfile.TemporaryDirectory() as d:
            record = Path(d) / "rec.psv"
            judge = load_judge(BS_JUDGE_KIND="t4", BS_JUDGE_RECORD=str(record))
            path = Path(d) / "c.tsv"
            rows = "".join(f"## 🔴 Rød sone {i}\\n- Tokenvalidering.\t1\tok\t\n" for i in range(19))
            path.write_text("answer\texpected_rodsone\tsource\trequired\n" + rows
                            + "Ubrukelig.\t1\tmust\t1\n", encoding="utf-8")

            def votes(p):
                a = p.split("<answer>")[-1]
                if "Ubrukelig" in a:
                    return "not json", 0.0
                return json.dumps({"rodsone": True, "rodsone_quote": "## 🔴 Rød sone " + a.split("Rød sone ")[-1].split("\n")[0]}), 0.0
            out = io.StringIO()
            with patch.dict(os.environ, {"BS_JUDGE_CMD": "echo stub"}), \
                    patch.object(judge, "call_model", side_effect=votes), contextlib.redirect_stdout(out), \
                    contextlib.redirect_stderr(io.StringIO()):
                status = judge.controls(str(path))
                key = judge.controls_hash(path.read_bytes())
                self.assertIsNone(judge.cached(key))
        self.assertEqual(status, 1, out.getvalue())
        self.assertIn("agree=19 errors=1", out.getvalue())
        self.assertIn("status=fail", out.getvalue())

    def test_unknown_kind_is_refused(self):
        with self.assertRaises(SystemExit):
            load_judge(BS_JUDGE_KIND="rv8")


class BlindspotUnchangedTests(unittest.TestCase):
    """The secq kind must leave the blind-spot judge as calibrated (#2437)."""

    def setUp(self):
        self.environment = patch.dict(os.environ, {"HOME": "/original-home", "GH_TOKEN": "x"}, clear=True)
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def test_blindspot_rubric_threshold_and_model_are_the_calibrated_ones(self):
        import hashlib
        judge = load_judge()
        self.assertEqual(hashlib.sha256(judge.RUBRIC.encode()).hexdigest()[:16], "4a7a68d916d3d645")
        self.assertEqual((judge.MODEL, judge.EFFORT, judge.MIN_ACCURACY), ("claude-haiku-5.5", "low", 0.92))

    def test_blindspot_control_answer_and_quote_rules_are_unchanged(self):
        judge = load_judge()
        self.assertEqual(judge.control_answer({"question": "Q?", "answer": ""}), "")
        self.assertEqual(judge.control_answer({"question": "Hvem leser?"}), judge.wrap("Hvem leser?"))
        a = "Hvem leser topicen? Det avgjør tilgangen."
        self.assertFalse(judge.is_question(judge.norm(a), judge.norm(a)))
        vote = json.dumps({"bs1": False, "bs1_quote": "", "bs2": True, "bs2_quote": a})
        self.assertFalse(judge.parse_vote(vote, a)["bs2"])

    def test_blindspot_yes_is_not_confirmed_again(self):
        judge = load_judge()
        q = "Hvem skal ha lesetilgang til topicen?"
        vote = json.dumps({"bs1": False, "bs1_quote": "", "bs2": True, "bs2_quote": q})
        with patch.object(judge, "call_model", return_value=(vote, 0.1)) as model:
            result = judge.judge("Jeg venter.\n\n" + q)
        self.assertTrue(result["bs2"])
        self.assertEqual(model.call_count, 2)

