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
                    io.StringIO()) as out, contextlib.redirect_stderr(io.StringIO()):
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
                    io.StringIO()) as out, contextlib.redirect_stderr(io.StringIO()):
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
