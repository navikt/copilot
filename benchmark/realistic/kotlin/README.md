# Offline Kotlin/Ktor pilot fixtures

Two independent Gradle projects: `cursor-debug` (symptom-only cursor pagination bug) and `status-feature` (a repository/service/route completion feature). Each `workspace/` is the complete agent-visible starter, including its public tests. Keep `prompt.md` and all of `controls/` outside the agent workspace. Do not copy the withheld tests into a benchmark attempt before the agent finishes.

Dependencies are pinned in each `build.gradle.kts`: Kotlin 2.1.20, Ktor 3.1.3 and Gradle 8.14.5 on JDK 21. The preflight uses the `gradle:8.14.5-jdk21` image pinned to digest `sha256:68e94b44fa717b894a559f555aaa491b855f94a0de689080f73e56da6c4a7479`. No application server or database is needed. Docker needs network access **only** to pull the image and warm the dependency cache; test runs use `--network none` and Gradle `--offline`.

`token-cache` (TokenX/Texas token cache, two bugs behind one symptom) is the third
project and adds `ktor-client-core`/`cio`/`mock` 3.1.3 and `kotlinx-serialization-json` 1.8.1.

The evaluator never bind-mounts host paths (Colima without host mounts sees an empty
directory). The Gradle cache is the Docker volume `nav-benchmark-gradle-cache`.
From the repository root, warm it for each starter:

```sh
docker volume create nav-benchmark-gradle-cache
for task in cursor-debug status-feature token-cache; do
  tar -C "benchmark/realistic/kotlin/$task/workspace" -c . | docker run --rm -i \
    -e GRADLE_USER_HOME=/cache --mount type=volume,src=nav-benchmark-gradle-cache,dst=/cache \
    gradle@sha256:68e94b44fa717b894a559f555aaa491b855f94a0de689080f73e56da6c4a7479 \
    sh -c 'mkdir /work && tar -x -C /work && cd /work && gradle test --no-daemon --console=plain'
done
```

Then verify all four fixtures and their baseline, known-good and invalid controls:

```sh
python3 -B benchmark/realistic/check.py
```

The check uses a fresh copy per variant and inserts withheld tests only while evaluating. Expected result: starters pass public tests; baseline and invalid controls fail acceptance, while the solutions pass acceptance and regression. These controls establish fixture discrimination, not task difficulty or agent success. They do not measure model quality, credit use or operational behavior.
