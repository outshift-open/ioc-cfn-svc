#!/usr/bin/env python3
# Copyright 2026 Cisco Systems, Inc. and its affiliates
#
# SPDX-License-Identifier: Apache-2.0

"""
Split the generated swagger.json into two separate specs:
  - docs/swagger.json          (public endpoints — /api/* excluding /api/internal/*)
  - docs/swagger-internal.json (internal endpoints — /api/internal/*)

Run after `swag init`.
"""

import json
import sys
from pathlib import Path

INTERNAL_PREFIX = "/api/internal"
DOCS_DIR = Path(__file__).parent.parent / "docs"
SOURCE = DOCS_DIR / "swagger.json"
INTERNAL_OUT = DOCS_DIR / "swagger-internal.json"


def split(source: Path) -> None:
    spec = json.loads(source.read_text())
    all_paths = spec.get("paths", {})

    public_paths = {k: v for k, v in all_paths.items() if not k.startswith(INTERNAL_PREFIX)}
    internal_paths = {k: v for k, v in all_paths.items() if k.startswith(INTERNAL_PREFIX)}

    # Rewrite public spec in-place (remove internal paths)
    spec["paths"] = public_paths
    source.write_text(json.dumps(spec, indent=4, ensure_ascii=False))
    print(f"Public spec:   {source}  ({len(public_paths)} paths)")

    # Write internal spec
    internal_spec = {**spec, "paths": internal_paths}
    internal_spec["info"] = {
        **spec["info"],
        "title": spec["info"]["title"] + " \u2014 Internal",
        "description": spec["info"].get("description", "") + "\n\nInternal endpoints only.",
    }
    INTERNAL_OUT.write_text(json.dumps(internal_spec, indent=4, ensure_ascii=False))
    print(f"Internal spec: {INTERNAL_OUT}  ({len(internal_paths)} paths)")


if __name__ == "__main__":
    if not SOURCE.exists():
        print(f"Error: {SOURCE} not found — run 'make docs' first", file=sys.stderr)
        sys.exit(1)
    split(SOURCE)
