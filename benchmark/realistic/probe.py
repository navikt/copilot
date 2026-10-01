#!/usr/bin/env python3
"""Check a network-disabled tool container without starting an agent."""

import json
import tempfile
from pathlib import Path

from check import NODE_IMAGE, ROOT, docker_limits, execute
from prepare import prepare

SCRIPT = r"""
const fs = require('node:fs');
const net = require('node:net');
const assert = require('node:assert/strict');

assert.equal(process.env.HOME, '/tmp');
fs.writeFileSync('/tmp/home-probe', 'ok');
fs.writeFileSync('/work/.isolation-probe', 'ok');
assert.equal(fs.readFileSync('/work/.isolation-probe', 'utf8'), 'ok');
fs.unlinkSync('/work/.isolation-probe');
assert.equal(fs.existsSync('/work/../evaluator/fixture/prompt.md'), false);
assert.equal(fs.existsSync('/work/controls'), false);

const socket = net.connect({host: '1.1.1.1', port: 443});
socket.setTimeout(3000);
socket.on('connect', () => { console.error('External connection succeeded'); process.exit(1); });
socket.on('timeout', () => { console.error('Network probe timed out'); process.exit(1); });
socket.on('error', error => {
  assert.ok(['ENETUNREACH', 'EHOSTUNREACH', 'EPERM', 'EACCES'].includes(error.code), error.message);
  console.log(JSON.stringify({workspace_write: true, evaluator_hidden: true, external_network_blocked: true}));
});
"""


def probe() -> dict:
    with tempfile.TemporaryDirectory(prefix=".nav-benchmark-", dir=ROOT) as temporary:
        attempt = Path(temporary) / "attempt"
        prepare("typescript/incident", attempt, "gpt-6-sol", "medium")
        command = docker_limits("512m", "64") + [
            "--mount", f"type=bind,src={attempt / 'workspace'},dst=/work",
            "--workdir", "/work", NODE_IMAGE, "node", "-e", SCRIPT,
        ]
        result = execute(command, 15)
        if result.returncode or not result.stdout.strip():
            raise RuntimeError(f"Tool-container isolation failed: {(result.stdout + result.stderr)[-1200:]}")
        return json.loads(result.stdout)


if __name__ == "__main__":
    print(json.dumps(probe(), sort_keys=True))
