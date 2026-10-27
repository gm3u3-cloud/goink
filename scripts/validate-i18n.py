#!/usr/bin/env python3
"""Validate locale files: ru.json must cover en.json.

Checks:
  1. All leaf keys of en.json exist in ru.json (missing keys fall back to EN).
  2. Placeholder parity: for every shared string key, the set of {{...}}
     placeholders in en and ru must be identical (interpolation contract).
  3. Plural completeness: for every en key with an _one/_other suffix,
     ru.json must define all four Russian forms _one/_few/_many/_other.

Usage: python3 scripts/validate-i18n.py [locales_dir]
"""

import json
import re
import sys
from pathlib import Path

PLACEHOLDER_RE = re.compile(r"\{\{[^}]+\}\}")
PLURAL_SUFFIX_RE = re.compile(r"_(one|few|many|other)$")
PLURAL_SUFFIXES = ("one", "few", "many", "other")


def load(path: Path) -> dict:
    with path.open(encoding="utf-8") as f:
        return json.load(f)


def flatten(d: dict, prefix: str = "") -> dict:
    out = {}
    for k, v in d.items():
        key = f"{prefix}.{k}" if prefix else k
        if isinstance(v, dict):
            out.update(flatten(v, key))
        else:
            out[key] = v
    return out


def main() -> int:
    locales = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(
        "frontend/src/i18n/locales"
    )
    en = flatten(load(locales / "en.json"))
    ru = flatten(load(locales / "ru.json"))

    missing = [k for k in en if k not in ru]

    ph_mismatch = []
    for k, v in en.items():
        if k in ru and isinstance(ru[k], str) and isinstance(v, str):
            en_ph = set(PLACEHOLDER_RE.findall(v))
            ru_ph = set(PLACEHOLDER_RE.findall(ru[k]))
            if en_ph != ru_ph:
                ph_mismatch.append((k, sorted(en_ph), sorted(ru_ph)))

    plural_missing = []
    for k in en:
        if PLURAL_SUFFIX_RE.search(k):
            base = PLURAL_SUFFIX_RE.sub("", k)
            for suffix in PLURAL_SUFFIXES:
                full = f"{base}_{suffix}"
                if full not in ru:
                    plural_missing.append(full)
    plural_missing = sorted(set(plural_missing))

    print(f"en keys: {len(en)}, ru keys: {len(ru)}")
    print(f"missing keys in ru: {len(missing)}")
    for k in missing:
        print(f"  MISSING: {k}")
    print(f"placeholder mismatches: {len(ph_mismatch)}")
    for k, a, b in ph_mismatch:
        print(f"  {k}: en={a} ru={b}")
    print(f"plural forms missing in ru: {len(plural_missing)}")
    for k in plural_missing:
        print(f"  PLURAL MISSING: {k}")

    ok = not (missing or ph_mismatch or plural_missing)
    print("OK" if ok else "FAILED")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
