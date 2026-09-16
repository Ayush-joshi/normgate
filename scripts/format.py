#!/usr/bin/env python3
"""Check tracked source roots, never generated dependency caches (NG-F008)."""
import os
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
files = sorted(str(p) for folder in ("cmd", "internal", "schemas", "test", "policies") for p in (root / folder).rglob("*.go"))
command = [os.environ.get("GOFMT", "gofmt"), "-w" if "--write" in sys.argv else "-l", *files]
result = subprocess.run(command, capture_output=True, text=True, check=True)
if result.stdout:
    print("Run make fmt:\n" + result.stdout, file=sys.stderr)
    sys.exit(1)
