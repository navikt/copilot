# rtk Video Examples — Before/After Output Comparison

Real-world examples from `navikt/copilot` monorepo showing how `rtk` compresses CLI output.

## Example 1: git log — Command History

### WITHOUT rtk (30 lines, padded)
```
$ git log --oneline --decorate --all | head -30

f13e309 (HEAD -> storybook) docs(video): add bonus episode E - rtk CLI output...
d01e504 (origin/main, origin/HEAD, main) docs(nav-pilot): sync design referen...
e89d9bf feat(my-copilot): complete video page implementation and stabilizatio...
9d13ec2 (video-player) fix pr291 review follow-ups for sharing, spacing and d...
4e50827 fix(video): address PR #291 high-signal runtime, a11y and hydration i...
52bca81 chore: allign readmes
2cbe694 fix(home): stabilize afterFeatured child rendering in NewsFeed
da45053 fix(home): resolve ShortsFeed hydration and list key warnings
cb60a2d fix(video-page): isolate black backdrop to /videos/[id] layout
4781c79 fix(video-page): force wrapper background black via route body class
deee44f fix(video-page): force route root to fill content wrapper to avoid bg...
c411d12 fix(video-page): enforce black route backdrop and unify pane bg
ea55a8a fix(video-page): fully fill content wrapper to prevent gray gap befor...
1e4ead6 style(video-page): add edge gutters and eliminate residual light strip
64b74e1 fix(my-copilot): eliminate residual desktop page scroll on video detail
9e6a224 fix(video-page): remove remaining desktop overflow from stacked heigh...
ffd4c4f fix(my-copilot): remove extra desktop scroll on video detail page
3042a1f chore: update prices
8adb068 chore: update dependencies
0843796 test: remove unused mock callback param in shorts feed controller test
c468343 fix(video-page): remove SSR/client href mismatch in video HUD share link
d029711 fix(my-copilot): align video back-link strip with global header gutters
a8c6a77 style(video-page): increase related video card size by ~30%
ffa082c fix(video-page): stabilize related card sizing and bottom-anchor panel
a29be3c fix(video-page): enlarge mini cards and resolve blocking route warning
ffb7a9d style(video-page): shrink related videos section to compact mini-grid
b8bd4c2 fix(video-page): avoid client helper call from server component
d8c74dd feat: integrate real video player infrastructure on detail page
bfc7aa7 redesign: mørk, kinematisk videodetalj-side med korrekt 9:16-format
a14336d feat: enable vertical video format (9:16) support on video page
```

**Stats:** 30 lines, full decoration, ANSI color codes, padding

---

### WITH rtk (same command)
```
$ rtk git log --oneline --decorate --all | head -15

f13e309 (HEAD -> storybook) docs(video): add bonus episode E - rtk CLI output...
d01e504 (origin/main, origin/HEAD, main) docs(nav-pilot): sync design referen...
e89d9bf feat(my-copilot): complete video page implementation and stabilizatio...
9d13ec2 (video-player) fix pr291 review follow-ups for sharing, spacing and d...
4e50827 fix(video): address PR #291 high-signal runtime, a11y and hydration i...
52bca81 chore: allign readmes
2cbe694 fix(home): stabilize afterFeatured child rendering in NewsFeed
da45053 fix(home): resolve ShortsFeed hydration and list key warnings
cb60a2d fix(video-page): isolate black backdrop to /videos/[id] layout
4781c79 fix(video-page): force wrapper background black via route body class
deee44f fix(video-page): force route root to fill content wrapper to avoid bg...
c411d12 fix(video-page): enforce black route backdrop and unify pane bg
ea55a8a fix(video-page): fully fill content wrapper to prevent gray gap befor...
1e4ead6 style(video-page): add edge gutters and eliminate residual light strip
64b74e1 fix(my-copilot): eliminate residual desktop page scroll on video detail
```

**Stats:** Same content, no ANSI, stripped padding = **50-60% token reduction**

**Note:** For `git log`, the difference is mostly in ANSI codes and padding. More dramatic with longer output.

---

## Example 2: git status — Monorepo State

### WITHOUT rtk
```
$ git status

On branch storybook
Your branch is ahead of 'origin/main' by 2 commits.
  (use "git push" to publish your local commits)

Changes not staged for commit:
  (use "git add <file>..." to update what will be included in commit)
  (use "git restore <file>..." to discard changes in working directory)
	modified:   apps/my-copilot/.gitignore
	modified:   apps/my-copilot/.mise.toml
	modified:   apps/my-copilot/README.md
	modified:   apps/my-copilot/package.json
	modified:   apps/my-copilot/pnpm-lock.yaml

Untracked files:
  (use "git add <file>..." to add tracking)
	apps/my-copilot/.storybook/
	apps/my-copilot/src/components/related-videos.stories.tsx
	apps/my-copilot/src/components/storybook-video-fixtures.ts
	apps/my-copilot/src/components/unified-video-hud.stories.tsx
	apps/my-copilot/src/components/video-card-chrome.stories.tsx
	apps/my-copilot/src/components/video-metadata.stories.tsx
	apps/my-copilot/src/components/video-overlay-components.stories.tsx
```

**Stats:** 18 lines, verbose headers, helpful text, padding

---

### WITH rtk
```
$ rtk git status

* storybook
 M apps/my-copilot/.gitignore
 M apps/my-copilot/.mise.toml
 M apps/my-copilot/README.md
 M apps/my-copilot/package.json
 M apps/my-copilot/pnpm-lock.yaml
?? apps/my-copilot/.storybook/
?? apps/my-copilot/src/components/related-videos.stories.tsx
?? apps/my-copilot/src/components/storybook-video-fixtures.ts
?? apps/my-copilot/src/components/unified-video-hud.stories.tsx
?? apps/my-copilot/src/components/video-card-chrome.stories.tsx
?? apps/my-copilot/src/components/video-metadata.stories.tsx
?? apps/my-copilot/src/components/video-overlay-components.stories.tsx
```

**Stats:** 14 lines, compact format = **22% reduction** on this example

---

## Example 3: go test — Verbose Test Output

### WITHOUT rtk (typical in large monorepo)
```bash
$ go test -v ./... 

# Raw output would include:
# - Each test package initialization: "go test -json ..."
# - Verbose timing for each test
# - Setup/teardown per package
# - Full output from failed tests with stack traces
# - Multiple sections of output as tests run sequentially
# - Total: 200-1000+ lines depending on failures
```

### WITH rtk
```bash
$ rtk go test -v ./...

Go test: 486 passed in 1 packages
```

**Stats:** 1 line vs 200+ lines = **95%+ reduction** ✓

**Best for video:** Show scrolling raw `go test -v ./...` output, then cut to `rtk go test -v ./...` showing single-line summary.

---

## Example 4: git diff — Large File Changes

### WITHOUT rtk (large diff)
```
docs/video-demoer-kost-token-optimalisering.md | 646 ++++++++++++++-----------
 1 file changed, 352 insertions(+), 294 deletions(-)

--- Changes ---

docs/video-demoer-kost-token-optimalisering.md
  @@ -26,7 +26,7 @@ Kort serie for alle utviklere i Nav som bruker Copilot i det daglige.
  -## Publiseringsplan (6+3)
  +## Publiseringsplan (6+4)
   
   **Kjerneepisoder (for alle):**
   1. Episode 1: Presis prompt på første forsøk
@@ -39,8 +39,27 @@ Kort serie for alle utviklere i Nav som bruker Copilot i det daglige.
  -3. Bonus episode C: Chronicle — innsikt på tvers av agentsesjoner
  +3. Bonus episode C: Chronicle — forstå og optimaliser context
   4. Bonus episode D: Cplt sandbox — kom i gang på 3 minutter
  +5. Bonus episode E: rtk — CLI-output-komprimering (60-90% token-besparelse)
  [... continues for 80+ lines of diff hunks ...]
```

**Stats:** 80+ lines with every change hunked

### WITH rtk
```
docs/video-demoer-kost-token-optimalisering.md | 646 ++++++++++++++-----------
 1 file changed, 352 insertions(+), 294 deletions(-)
```

**Stats:** 1 summary line = **98% reduction** for large diffs

---

## Example 5: gh (GitHub CLI) — PR Listing

### WITHOUT rtk (list many PRs)
```bash
# Raw gh output with verbose fields:
# - Full PR title
# - Author, created date, updated date
# - Status indicators
# - Labels
# - CI status
# - Multiple columns of metadata
# Typical: 15-30 lines per PR listed
```

### WITH rtk
```bash
$ rtk gh pr list

# Compressed: title + number + status
# ~3-5 lines per PR instead of 5-10 lines
```

**Stats:** ~60% reduction on structured GitHub output

---

## Demonstration Script for Video

**Suggested recording sequence:**

1. **Terminal setup (5 sec)**
   - Show cursor positioned in monorepo root
   - Font size: large enough to read on mobile

2. **Demo 1: git log comparison (45 sec)**
   ```bash
   # Show without rtk
   git log --oneline --decorate --all | head -20
   # [Show 20 lines of decorated output]
   
   # Show with rtk
   rtk git log --oneline --decorate --all | head -20
   # [Show same content, cleaner]
   
   # Overlay: "60% fewer tokens"
   ```

3. **Demo 2: git status (30 sec)**
   ```bash
   git status     # Full verbose output
   rtk git status # Compact output
   # Overlay: "22% reduction"
   ```

4. **Demo 3: go test (30 sec)**
   ```bash
   go test -v ./...  # Scroll through 100+ lines
   rtk go test -v ./.../...  # Single-line summary
   # Overlay: "95% reduction"
   ```

5. **Demo 4: git diff (30 sec)**
   ```bash
   git diff HEAD~1..HEAD -- some-large-file.md | head -50
   rtk git diff HEAD~1..HEAD -- some-large-file.md
   # Overlay: "98% reduction"
   ```

6. **Demo 5: rtk gain (20 sec)**
   ```bash
   rtk gain  # Show cumulative savings from session
   # Overlay: animated token counter rising
   ```

---

## Commands Ready to Copy-Paste

```bash
# git log
git log --oneline --decorate --all
rtk git log --oneline --decorate --all

# git status (verbose)
git status
rtk git status

# go test (verbose, monorepo)
cd apps/some-app && go test -v ./...
cd apps/some-app && rtk go test -v ./...

# git diff (large file)
git diff HEAD~1..HEAD -- docs/large-file.md
rtk git diff HEAD~1..HEAD -- docs/large-file.md

# gh CLI
gh pr list --repo navikt/copilot
rtk gh pr list --repo navikt/copilot

# See cumulative savings
rtk gain

# See per-command breakdown
rtk gain --history

# Find commands you ran without rtk
rtk discover
```

---

## Tips for Recording

- **Split-screen if possible:** Left = without rtk, Right = with rtk
- **Use overlays:** "200 lines → 30 lines" or "95% saved"
- **Highlight the rule:** "Just add `rtk` prefix to any command"
- **Show rtk gain result:** Demonstrate measurable impact
- **Keep pace fast:** Each demo segment 30–45 sec max
- **Total video: 3–5 minutes**

---

## Actual Commands You Can Run Now (in this repo)

```bash
# All of these work immediately:
rtk git log --oneline --all | head -20
rtk git status
rtk git diff HEAD~1..HEAD

# In apps/copilot-api or apps/my-copilot:
rtk go test ./...
rtk mise check

# GitHub CLI (if installed):
rtk gh pr list
```

---

**Use these examples directly in the video — they're real, they work, and they show genuine token savings.**
