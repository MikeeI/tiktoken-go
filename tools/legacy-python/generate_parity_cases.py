#!/usr/bin/env python3
"""Generate deterministic upstream tiktoken token-id parity fixtures."""

from __future__ import annotations

import json
import sys
from typing import Any

import tiktoken

ALL = "all"
ENDOFTEXT = "<|endoftext|>"
FIM_PREFIX = "<|fim_prefix|>"
FIM_MIDDLE = "<|fim_middle|>"
FIM_SUFFIX = "<|fim_suffix|>"

CASES: list[dict[str, Any]] = [
    {
        "name": "cl100k_unicode_sentence",
        "encoding": "cl100k_base",
        "text": "hello world!你好，世界！",
        "allowed_special": [],
        "disallowed_special": [],
    },
    {
        "name": "cl100k_endoftext_allowed",
        "encoding": "cl100k_base",
        "text": f"hello {ENDOFTEXT}",
        "allowed_special": [ENDOFTEXT],
        "disallowed_special": [],
    },
    {
        "name": "cl100k_numbers_whitespace",
        "encoding": "cl100k_base",
        "text": "Numbers: 1234567890\n\tspaced   words\r\nfinal line",
        "allowed_special": [],
        "disallowed_special": [],
    },
    {
        "name": "cl100k_emoji_combining_rtl",
        "encoding": "cl100k_base",
        "text": "👩\u200d💻 café e\u0301 שלום مرحبا",
        "allowed_special": [],
        "disallowed_special": [],
    },
    {
        "name": "o200k_multilingual_emoji",
        "encoding": "o200k_base",
        "text": "Hello 世界 👋🏽 Здравей مرحبا नमस्ते",
        "allowed_special": [],
        "disallowed_special": [],
    },
    {
        "name": "o200k_long_single_piece",
        "encoding": "o200k_base",
        "text": "abcdefghijklmnopqrstuvwxyz" * 64,
        "allowed_special": [],
        "disallowed_special": [],
    },
    {
        "name": "p50k_base_code_snippet",
        "encoding": "p50k_base",
        "text": "def greet(name):\n    return f'hello {name}'\nprint(greet('world'))",
        "allowed_special": [],
        "disallowed_special": [],
    },
    {
        "name": "p50k_edit_fim_allowed",
        "encoding": "p50k_edit",
        "text": f"{FIM_PREFIX}def add(a, b):\n    return {FIM_SUFFIX}{FIM_MIDDLE}a + b",
        "allowed_special": [FIM_PREFIX, FIM_MIDDLE, FIM_SUFFIX],
        "disallowed_special": [],
    },
    {
        "name": "r50k_ascii_punctuation",
        "encoding": "r50k_base",
        "text": "ASCII punctuation: !?.,;:()[]{}<>+-=*/_`~|\\\"'",
        "allowed_special": [],
        "disallowed_special": [],
    },
]


def special_arg(values: list[str]) -> set[str] | str:
    if values == [ALL]:
        return ALL
    return set(values)


def main() -> None:
    output = []
    for case in CASES:
        encoding = tiktoken.get_encoding(case["encoding"])
        tokens = encoding.encode(
            case["text"],
            allowed_special=special_arg(case["allowed_special"]),
            disallowed_special=special_arg(case["disallowed_special"]),
        )
        output.append({**case, "tokens": tokens})

    json.dump(output, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
