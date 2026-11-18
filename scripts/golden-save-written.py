#!/usr/bin/env python3
"""Print the files a golden call wrote, for transkripter/<arm>.<slug>.run<N>.files.txt.

  golden-save-written.py <workspace> < paths     # one ./relative/path per line

Each path is opened one component at a time from a directory fd with
O_NOFOLLOW, and read from the fd that was checked. A file or parent directory
swapped for a symlink before or during the read is refused, never followed,
and a path is never resolved twice (#2396).

Only the source types the fixtures write are copied, minus dotfiles and names
that suggest a secret. An allow list, because a deny list of secret file names
is never complete, and a skipped file costs only audit material. The publishing
scan in the plan still runs on what is copied.
"""

import os
import stat
import subprocess
import sys

COPY = (".kt", ".kts", ".go", ".ts", ".tsx", ".rs", ".toml", ".md", ".yaml", ".yml")
SECRET_WORDS = ("credential", "secret", "kubeconfig", "token", "password")
# Tests only: runs with the path after the parent directory is open and before
# the file is, to swap things under the reader.
HOOK = os.environ.get("GOLDEN_SAVE_WRITTEN_HOOK")


def read(root_fd, rel):
    parts = [p for p in rel.split("/") if p not in ("", ".")]
    if not parts or ".." in parts:
        raise PermissionError(rel)
    fd = os.dup(root_fd)
    try:
        for p in parts[:-1]:
            nxt = os.open(p, os.O_RDONLY | os.O_NOFOLLOW | os.O_DIRECTORY, dir_fd=fd)
            os.close(fd)
            fd = nxt
        if HOOK:
            subprocess.run([HOOK, rel], check=True)
        # O_NONBLOCK: a FIFO must not hang the harness; fstat refuses it below.
        f = os.open(parts[-1], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
    finally:
        os.close(fd)
    with os.fdopen(f, "rb") as fh:
        if not stat.S_ISREG(os.fstat(fh.fileno()).st_mode):
            raise PermissionError(rel)
        return fh.read()


def main():
    root_fd = os.open(sys.argv[1], os.O_RDONLY | os.O_NOFOLLOW | os.O_DIRECTORY)
    out = sys.stdout.buffer
    for rel in sys.stdin.read().splitlines():
        name = rel.rsplit("/", 1)[-1]
        if name.startswith(".") or not name.endswith(COPY) or any(w in name.lower() for w in SECRET_WORDS):
            out.write(f"=== {rel} (skipped: not a source file)\n".encode())
            continue
        try:
            data = read(root_fd, rel)
        except FileNotFoundError:
            out.write(f"=== {rel} (deleted)\n".encode())
        except OSError:
            out.write(f"=== {rel} (skipped: symlink or not a regular file)\n".encode())
        else:
            out.write(f"=== {rel}\n".encode() + data + b"\n")


if __name__ == "__main__":
    main()
