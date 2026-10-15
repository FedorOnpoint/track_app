#!/usr/bin/env python3
"""User-scoped track_app client; tokens are never passed in argv or logged."""
import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid


def validate_base_url(value):
    parsed = urllib.parse.urlsplit(value)
    local = parsed.hostname in ("localhost", "127.0.0.1", "::1")
    if (not parsed.hostname or parsed.username or parsed.password or parsed.query
            or parsed.fragment or parsed.path not in ("", "/")
            or (parsed.scheme != "https" and not (parsed.scheme == "http" and local))):
        raise ValueError("Invalid origin")
    return value.rstrip("/")


def load_config(path):
    values = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            key, separator, value = line.partition("=")
            if not separator:
                raise ValueError("Invalid config")
            values[key.strip()] = value.strip()
    for name in ("TRACK_AGENT_BASE_URL", "TRACK_AGENT_TOKEN", "TRACK_AGENT_USER_ID"):
        if not values.get(name):
            raise ValueError("Incomplete config")
    values["TRACK_AGENT_BASE_URL"] = validate_base_url(values["TRACK_AGENT_BASE_URL"])
    values["TRACK_AGENT_USER_ID"] = str(uuid.UUID(values["TRACK_AGENT_USER_ID"]))
    return values


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request(config, path, payload=None):
    req = urllib.request.Request(
        config["TRACK_AGENT_BASE_URL"] + "/api/v1/agent/" + path,
        data=None if payload is None else json.dumps(payload, ensure_ascii=False).encode(),
        method="GET" if payload is None else "PUT",
        headers={"Authorization": "Bearer " + config["TRACK_AGENT_TOKEN"],
                 "Content-Type": "application/json"},
    )
    try:
        with urllib.request.build_opener(NoRedirect()).open(req, timeout=20) as response:
            return json.load(response)
    except urllib.error.HTTPError as exc:
        raise RuntimeError(f"API returned HTTP {exc.code}") from None
    except (urllib.error.URLError, TimeoutError):
        raise RuntimeError("Connection failed or timed out; update outcome may be unknown") from None


def topics(config):
    result = request(config, "topics")
    if result.get("user_id") != config["TRACK_AGENT_USER_ID"]:
        raise ValueError("Account mismatch")
    return result


def initialize(args):
    user_id = str(uuid.UUID(args.user_id))
    if uuid.UUID(user_id).int == 0 or not 1 <= args.days <= 365:
        raise ValueError("Invalid user or expiry")
    base = validate_base_url(args.base_url)
    client_path = args.config
    server_path = client_path.with_name(client_path.name + ".server")
    if client_path.exists() or server_path.exists():
        raise ValueError("Config already exists; use another --config path for rotation")
    token = secrets.token_hex(32)
    expiry = (datetime.now(timezone.utc) + timedelta(days=args.days)).isoformat(timespec="seconds").replace("+00:00", "Z")
    client = f"TRACK_AGENT_BASE_URL={base}\nTRACK_AGENT_TOKEN={token}\nTRACK_AGENT_USER_ID={user_id}\n"
    server = f"AGENT_API_TOKEN_SHA256={hashlib.sha256(token.encode()).hexdigest()}\nAGENT_USER_ID={user_id}\nAGENT_TOKEN_EXPIRES_AT={expiry}\n"
    for path, content in ((client_path, client), (server_path, server)):
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as file:
            file.write(content)
    print(f"Created {client_path} and {server_path}; expires {expiry}. Token not displayed.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, default=Path(".env.agent"))
    commands = parser.add_subparsers(dest="command", required=True)
    init = commands.add_parser("init")
    init.add_argument("--user-id", required=True)
    init.add_argument("--base-url", required=True)
    init.add_argument("--days", type=int, default=90)
    commands.add_parser("topics")
    update = commands.add_parser("update")
    update.add_argument("slug")
    update.add_argument("update_id")
    update.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    if args.command == "init":
        initialize(args)
        return
    config = load_config(args.config)
    if args.command == "topics":
        result = topics(config)
    else:
        if not all(re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}", x) for x in (args.slug, args.update_id)):
            raise ValueError("Invalid slug or update_id")
        payload = json.load(sys.stdin)
        if not isinstance(payload, dict) or not payload:
            raise ValueError("Expected a nonempty JSON object")
        if args.dry_run:
            result = {"dry_run": True, "user_id": config["TRACK_AGENT_USER_ID"],
                      "slug": args.slug, "update_id": args.update_id, "payload": payload}
        else:
            catalog = topics(config)
            if not any(topic["slug"] == args.slug for topic in catalog["data"]):
                raise ValueError("Unknown topic")
            result = request(config, f"topics/{args.slug}/updates/{args.update_id}", payload)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, RuntimeError) as error:
        # Never echo configuration values or arbitrary server error bodies.
        print(str(error) if isinstance(error, RuntimeError) else
              "Invalid input/configuration or inaccessible file; verify account, URL and config path.", file=sys.stderr)
        sys.exit(1)
