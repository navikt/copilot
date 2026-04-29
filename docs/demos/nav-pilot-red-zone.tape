# nav-pilot rød/grønn sone demo
# Demonstrates how nav-pilot identifies core logic as 🔴 rød sone
# and delivers stubs instead of full implementations.
#
# Run: vhs docs/demos/nav-pilot-red-zone.tape
# Requires: vhs (brew install charmbracelet/tap/vhs)

Output docs/demos/nav-pilot-red-zone.gif
Output docs/demos/nav-pilot-red-zone.webm

Set Shell "bash"
Set FontSize 16
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set Padding 20

# Title frame
Type "# nav-pilot: Bevisst AI-bruk — rød/grønn sone i praksis"
Enter
Sleep 2s
Type "# Demo: Implementere §8-20 redusert arbeidsgiverperiode i helse-spleis"
Enter
Sleep 3s
Hide
Type "clear"
Enter
Show

# Simulate the copilot CLI prompt
Type "bash docs/demos/nav-pilot-red-zone-sim.sh"
Enter
Sleep 45s
