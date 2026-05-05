# cplt demo — Copilot autopilot agent blocked by sandbox
#
# Story: Developer starts a Copilot coding session through cplt.
#        The agent works on the project, then attempts credential
#        theft and data exfiltration. cplt's sandbox blocks everything.
#
# Recording instructions:
#   1. cd to repo root (navikt/copilot)
#   2. chmod +x apps/my-copilot/public/cplt-demo-sim.sh
#   3. vhs apps/my-copilot/public/cplt-demo.tape
#
# Output: apps/my-copilot/public/demos/cplt-demo.gif

Output apps/my-copilot/public/demos/cplt-demo.gif
Output apps/my-copilot/public/demos/cplt-demo.webm

Set Shell "bash"
Set FontSize 16
Set Width 1024
Set Height 600
Set Theme "Catppuccin Mocha"
Set TypingSpeed 25ms
Set Padding 16

# Setup: alias cplt to the simulation script (hidden from recording)
Hide
Type "alias cplt='bash apps/my-copilot/public/cplt-demo-sim.sh'"
Enter
Sleep 300ms
Type "clear"
Enter
Sleep 300ms
Show

# Developer starts cplt — simulation script provides all output
Type "cplt"
Enter
Sleep 40s
