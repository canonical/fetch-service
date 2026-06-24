#!/usr/bin/env python3
# Copyright 2026 Canonical Ltd.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License version 3 as
# published by the Free Software Foundation.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

"""
Generate a git upload-pack .raw testdata file from a local directory.

Usage: python3 generate_raw.py <repo_dir> <output.raw>

The directory should already contain the files you want in the pack.
This script will run `git init` and commit everything in the directory,
then produce a .raw file in the git v2 pkt-line + sideband format that
the fetch-service test helpers (git.UnpackObjects / git.Checkout) expect.
"""

import os
import subprocess
import sys
from pathlib import Path


def pkt(data: str | bytes) -> bytes:
    if isinstance(data, str):
        data = data.encode()
    return f"{len(data) + 4:04x}".encode() + data


def sideband_pkt(sb: int, data: bytes) -> bytes:
    return f"{len(data) + 5:04x}".encode() + bytes([sb]) + data


def main(repo_dir: Path, output_path: Path) -> None:
    env = {
        **os.environ,
        "GIT_AUTHOR_NAME": "Test",
        "GIT_AUTHOR_EMAIL": "test@test.com",
        "GIT_COMMITTER_NAME": "Test",
        "GIT_COMMITTER_EMAIL": "test@test.com",
    }

    subprocess.run(["git", "init", "-q"], cwd=repo_dir, env=env, check=True)
    subprocess.run(["git", "add", "."], cwd=repo_dir, env=env, check=True)
    subprocess.run(
        ["git", "commit", "-q", "-m", "feat: add files"],
        cwd=repo_dir, env=env, check=True,
    )

    sha = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=repo_dir
    ).decode().strip()
    print(f"Commit SHA: {sha}")

    obj_lines = subprocess.check_output(
        ["git", "rev-list", "--objects", "HEAD"], cwd=repo_dir
    ).decode().strip().splitlines()
    obj_shas = [line.split()[0] for line in obj_lines]

    pack_data = subprocess.check_output(
        ["git", "pack-objects", "--stdout"],
        input=("\n".join(obj_shas) + "\n").encode(),
        cwd=repo_dir, env=env,
    )
    print(f"Pack size: {len(pack_data)} bytes")

    raw  = pkt(b"shallow-info\n")
    raw += pkt(f"shallow {sha}".encode())
    raw += b"0001"           # delimiter
    raw += pkt(b"packfile\n")

    CHUNK = 65516
    for i in range(0, len(pack_data), CHUNK):
        raw += sideband_pkt(1, pack_data[i:i + CHUNK])

    raw += b"0000"

    output_path.write_bytes(raw)

    print(f"Written {len(raw)} bytes to {output_path}")
    print(f"Use this SHA in your test: {sha}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <repo_dir> <output.raw>")
        sys.exit(1)
    main(Path(sys.argv[1]), Path(sys.argv[2]))
