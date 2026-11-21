---
applyTo: "**/*_test.go,**/*.test.ts,**/*.test.tsx,**/*.spec.ts,**/testdata/**"
excludeAgent: "cloud-agent"
---

# Review: tests and fixtures

Review-only guidance.

- **Right reason**: The test must fail because of the behaviour it names. Flag a test where an earlier check (auth, validation, a nil stub) rejects the input first, so the guard under test is never reached.
- **Mutation check**: For a new guard or security fix, ask for evidence that the test fails when that one guard is reverted. One mutation per guard.
- **No weakened assertions**: Flag removed assertions, widened expectations (`Contains` instead of equal, status range instead of exact code), `t.Skip`, and loosened timeouts in a change that fixes behaviour.
- **Stubs at one seam**: If every test stubs the same boundary, ask for one test that crosses it for real.
- **Controls**: A gate or assertion needs a case that makes it fail. A check that cannot fail reports green.
- **Isolation**: Tests must not touch the real home or config. In `cli/nav-pilot`, use `isolatedConfig(t)` and `testhome.Run`.
- **Fake data only**: No real names, emails, `NAVident`, tokens, or production numbers (not even aggregates) in fixtures or testdata. The repo is public.
