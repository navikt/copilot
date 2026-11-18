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
# The largest fixture source is a few KB and a whole fixture tree about 33 KB,
# so a real fix stays far below both. The limits only stop a runaway or hostile
# write from landing in docs/: per file, and per call across all its files.
PER_FILE = 256 * 1024
PER_CALL = 2 * 1024 * 1024


class Skip(Exception):
    pass




def read(root_fd, rel, limit):
    """Up to limit bytes of rel, and whether there was more."""
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
        st = os.fstat(fh.fileno())
        if not stat.S_ISREG(st.st_mode):
            raise PermissionError(rel)
        # A hard link is a regular file inside $WS whose content may live
        # anywhere on the same filesystem; a fixture never makes one.
        if st.st_nlink > 1:
            raise Skip("hard link")
        # Bounded read from the open fd: a file that grows meanwhile still
        # stops at the limit.
        data = fh.read(limit + 1)
        return data[:limit], len(data) > limit


def main():
    root_fd = os.open(sys.argv[1], os.O_RDONLY | os.O_NOFOLLOW | os.O_DIRECTORY)
    out = sys.stdout.buffer
    budget = PER_CALL
    for rel in sys.stdin.read().splitlines():
        name = rel.rsplit("/", 1)[-1]
        if name.startswith(".") or not name.endswith(COPY) or any(w in name.lower() for w in SECRET_WORDS):
            out.write(f"=== {rel} (skipped: not a source file)\n".encode())
            continue
        if budget <= 0:
            out.write(f"=== {rel} (skipped: {PER_CALL}-byte limit for this call reached)\n".encode())
            continue
        limit = min(PER_FILE, budget)
        try:
            data, more = read(root_fd, rel, limit)
        except Skip as e:
            out.write(f"=== {rel} (skipped: {e})\n".encode())
        except FileNotFoundError:
            out.write(f"=== {rel} (deleted)\n".encode())
        except OSError:
            out.write(f"=== {rel} (skipped: symlink or not a regular file)\n".encode())
        else:
            budget -= len(data)
            note = f" (truncated at {limit} bytes)" if more else ""
            out.write(f"=== {rel}{note}\n".encode() + data + b"\n")


if __name__ == "__main__":
    main()
