import argparse
import contextlib
import hashlib
import io
import json
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch
import uuid

import progress


class ClientTests(unittest.TestCase):
    def test_initialization_protects_secret_and_does_not_overwrite(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / ".env.agent"
            args = argparse.Namespace(config=path, user_id=str(uuid.uuid4()),
                                      base_url="https://example.com", days=90)
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                progress.initialize(args)
            config = progress.load_config(path)
            token = config["TRACK_AGENT_TOKEN"]
            self.assertEqual(len(token), 64)
            self.assertNotIn(token, output.getvalue())
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            server_path = path.with_name(path.name + ".server")
            self.assertEqual(stat.S_IMODE(server_path.stat().st_mode), 0o600)
            server = server_path.read_text()
            self.assertNotIn(token, server)
            self.assertIn(hashlib.sha256(token.encode()).hexdigest(), server)
            with self.assertRaises(ValueError):
                progress.initialize(args)
            self.assertEqual(progress.load_config(path)["TRACK_AGENT_TOKEN"], token)

    def test_rejects_unsafe_origins(self):
        for url in ("http://example.com", "https://user:password@example.com",
                    "https://example.com/path", "https://example.com?token=x", "file:///tmp/x"):
            with self.subTest(url=url), self.assertRaises(ValueError):
                progress.validate_base_url(url)
        self.assertEqual(progress.validate_base_url("http://127.0.0.1:8000/"), "http://127.0.0.1:8000")

    def test_account_mismatch_prevents_update(self):
        with patch.object(progress, "load_config", return_value={"TRACK_AGENT_USER_ID": "owner"}), \
             patch.object(progress, "request", return_value={"user_id": "other", "data": []}) as request, \
             patch("sys.argv", ["progress.py", "update", "go-maps", "session-1"]), \
             patch("sys.stdin", io.StringIO('{"status":"learning"}')):
            with self.assertRaises(ValueError):
                progress.main()
            self.assertEqual(request.call_count, 1)
            self.assertEqual(request.call_args.args[1], "topics")

    def test_dry_run_makes_no_requests(self):
        with patch.object(progress, "load_config", return_value={"TRACK_AGENT_USER_ID": "owner"}), \
             patch.object(progress, "request") as request, \
             patch("sys.argv", ["progress.py", "update", "go-maps", "session-1", "--dry-run"]), \
             patch("sys.stdin", io.StringIO('{"status":"learning"}')), \
             contextlib.redirect_stdout(io.StringIO()) as output:
            progress.main()
            request.assert_not_called()
            self.assertTrue(json.loads(output.getvalue())["dry_run"])

    def test_redirects_are_never_followed(self):
        self.assertIsNone(progress.NoRedirect().redirect_request(None, None, 302, "Found", {}, "https://other.example"))

    def test_successful_update_preserves_intent_id_and_body(self):
        config = {"TRACK_AGENT_USER_ID": "owner"}
        with patch.object(progress, "load_config", return_value=config), \
             patch.object(progress, "request", side_effect=[
                 {"user_id": "owner", "data": [{"slug": "go-maps"}]},
                 {"data": {"status": "learning"}},
             ]) as request, \
             patch("sys.argv", ["progress.py", "update", "go-maps", "session-1"]), \
             patch("sys.stdin", io.StringIO('{"status":"learning"}')), \
             contextlib.redirect_stdout(io.StringIO()):
            progress.main()
            self.assertEqual(request.call_args.args, (config, "topics/go-maps/updates/session-1", {"status": "learning"}))


if __name__ == "__main__":
    unittest.main()
