# Comment rule A/B, seeded variant (#2605, #2629)

Fixture variant for the second run. `seed.py` adds stale comments to the prepared workspace after `prepare.py`, before the agent starts. Each comment describes behaviour the task removes: four in `kotlin/status-feature`, three in `typescript/feature`. Both tasks also add exported names: Kotlin functions are public by default, and the TypeScript task adds an exported `POST` handler. The source fixtures and evaluator tests are unchanged, so `check.py` scores the task exactly as before.

Results follow in this file after the run.
