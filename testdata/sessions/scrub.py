#!/usr/bin/env python3
"""Replace secret-shaped values in a native session export.

Reads unsanitized files from --source and writes testdata-shaped trees to --dest.
The original pack is never written into the repository.
"""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path

JWT = re.compile(r"eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}")
SK_TOKEN = re.compile(r"(?i)\bsk-[A-Za-z0-9_-]{16,}")
EMAIL = re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b")
ENV_ASSIGN = re.compile(
    r"(?i)\b(OPENAI_API_KEY|ANTHROPIC_API_KEY|AMP_API_KEY|HF_TOKEN|GITHUB_TOKEN|"
    r"TRAICR_ADMIN_TOKEN|AWS_SECRET_ACCESS_KEY|CURSOR_API_KEY)\s*=\s*(?:\"[^\"]+\"|'[^']+'|\$\([^)]+\)|\S+)"
)
HOME = re.compile(r"/home/pael\b")
USERS_HOME = re.compile(r"/Users/pael\b")
HOME_SLUG = re.compile(r"(?<=[-_])home-pael(?=[-_])")
USER_WORD = re.compile(r"\b[Pp]ael\b")
HOSTS = (
    (re.compile(r"https://traicr\.esquie\.pael\.dev"), "https://traicr.example.test"),
    (re.compile(r"https://esquie\.pael\.dev"), "https://example.test"),
    (re.compile(r"\btraicr\.esquie\.pael\.dev\b"), "traicr.example.test"),
    (re.compile(r"\besquie\.pael\.dev\b"), "example.test"),
)
MACHINE_ID = re.compile(r"\bb525850e236c3d3fe1fc641dd5b3fd75\b")
WORD_MAELLE = re.compile(r"\b[Mm]aelle\b")
WORD_ESQUIE = re.compile(r"\b[Ee]squie\b")

KEEP_EMAIL_DOMAINS = {
    "ampcode.com",
    "cloudmanic.com",
    "example.com",
    "example.invalid",
    "example.test",
    "evil.test",
    "github.com",
    "test.com",
}

FAKE_JWT = "eyJhbGciOiJub25lIn0.eyJzdWIiOiJmaXh0dXJlLXVzZXIifQ.testdata"
FAKE_SK = "test-session-fixture-key"
FAKE_EMAIL = "user@example.test"


def replace_env(match: re.Match[str]) -> str:
    name = match.group(1)
    raw = match.group(0)
    _, _, value = raw.partition("=")
    value = value.strip()
    if value.startswith("$(") and value.endswith(")"):
        return raw
    if value[:1] in {"'", '"'}:
        return f"{name}={value[0]}test-session-fixture-key{value[0]}"
    return f"{name}=test-session-fixture-key"


def replace_email(match: re.Match[str]) -> str:
    domain = match.group(0).rsplit("@", 1)[1].lower()
    if domain in KEEP_EMAIL_DOMAINS:
        return match.group(0)
    return FAKE_EMAIL


def replace_word(match: re.Match[str], replacement: str) -> str:
    src = match.group(0)
    if src[0].isupper():
        return replacement[0].upper() + replacement[1:]
    return replacement


def scrub_text(text: str) -> str:
    text = JWT.sub(FAKE_JWT, text)
    text = SK_TOKEN.sub(FAKE_SK, text)
    text = ENV_ASSIGN.sub(replace_env, text)
    text = EMAIL.sub(replace_email, text)
    text = HOME.sub("/home/user", text)
    text = USERS_HOME.sub("/home/user", text)
    text = HOME_SLUG.sub("home-user", text)
    text = USER_WORD.sub("user", text)
    for pattern, replacement in HOSTS:
        text = pattern.sub(replacement, text)
    text = MACHINE_ID.sub("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", text)
    text = WORD_MAELLE.sub(lambda match: replace_word(match, "fixture-host"), text)
    text = WORD_ESQUIE.sub(lambda match: replace_word(match, "fixture-host"), text)
    return text


TRACES = (
    ("pi", "records.jsonl", "pi-herdr", "source/records.jsonl"),
    ("amp", "export.json", "amp-transcript-support", "source/export.json"),
    ("claude-code", "records.jsonl", "claude-cleanup", "source/records.jsonl"),
)


def validate(path: Path) -> None:
    if path.suffix == ".jsonl":
        for line_no, line in enumerate(path.read_text().splitlines(), 1):
            if line.strip():
                json.loads(line)
        return
    if path.suffix == ".json":
        json.loads(path.read_text())


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=Path("/tmp/destination/destination"))
    parser.add_argument("--dest", type=Path, default=Path(__file__).resolve().parent)
    args = parser.parse_args()
    for src_dir, src_name, dest_dir, dest_name in TRACES:
        source = args.source / src_dir / src_name
        dest = args.dest / dest_dir / dest_name
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(scrub_text(source.read_text()))
        validate(dest)
        print(f"wrote {dest.relative_to(args.dest)} ({dest.stat().st_size} bytes)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
