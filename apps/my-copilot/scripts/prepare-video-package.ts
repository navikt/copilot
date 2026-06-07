import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { arg, parseTags, required } from "./video-cli-common.ts";

type ProbeResult = {
  durationSec: number;
  width: number;
  height: number;
};

function optionalNumber(name: string): number | undefined {
  const value = arg(name);
  if (value === undefined) return undefined;
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    throw new Error(`Invalid value for --${name}; expected a number`);
  }
  return parsed;
}

function optionalInteger(name: string, defaultValue: number): number {
  const value = arg(name);
  if (value === undefined) return defaultValue;
  const parsed = Number(value);
  if (!Number.isInteger(parsed)) {
    throw new Error(`Invalid value for --${name}; expected an integer`);
  }
  return parsed;
}

function optionalPositiveInteger(name: string): number | undefined {
  const value = arg(name);
  if (value === undefined) return undefined;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`Invalid value for --${name}; expected a positive integer`);
  }
  return parsed;
}

function optionalString(name: string, defaultValue: string): string {
  return arg(name) ?? defaultValue;
}

function requireTool(tool: string) {
  try {
    execFileSync(tool, ["-version"], { stdio: "ignore" });
  } catch {
    throw new Error(`Missing required tool: ${tool}`);
  }
}

function slugifyTitle(id: string): string {
  return id
    .split(/[-_]+/g)
    .filter(Boolean)
    .map((part) => part[0].toUpperCase() + part.slice(1))
    .join(" ");
}

function toAbsolutePath(value: string): string {
  return path.isAbsolute(value) ? value : path.resolve(process.cwd(), value);
}

function probeVideo(inputFile: string): ProbeResult {
  const raw = execFileSync(
    "ffprobe",
    [
      "-v",
      "error",
      "-select_streams",
      "v:0",
      "-show_entries",
      "stream=width,height:format=duration",
      "-of",
      "json",
      inputFile,
    ],
    { encoding: "utf8" }
  );
  const parsed = JSON.parse(raw) as {
    streams?: Array<{ width?: number; height?: number }>;
    format?: { duration?: string };
  };
  const stream = parsed.streams?.[0];
  const durationSec = Math.max(1, Math.round(Number(parsed.format?.duration ?? "0")));
  const width = stream?.width ?? 0;
  const height = stream?.height ?? 0;
  if (!width || !height) {
    throw new Error("Could not determine video dimensions from input");
  }
  return { durationSec, width, height };
}

function derivePosterAt(durationSec: number, explicit?: number): number {
  if (explicit !== undefined) {
    return Math.max(0, explicit);
  }
  if (durationSec <= 2) {
    return 0;
  }
  return Math.min(Math.max(1, Math.round(durationSec * 0.1)), durationSec - 1);
}

function hlsFilter(): string {
  return "scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2";
}

function quote(value: string): string {
  return JSON.stringify(value);
}

function writeFile(filePath: string, content: string) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, content);
}

function main() {
  requireTool("ffmpeg");
  requireTool("ffprobe");

  const inputFile = toAbsolutePath(required("input"));
  if (!fs.existsSync(inputFile)) {
    throw new Error(`Input file does not exist: ${inputFile}`);
  }

  const id = required("id");
  if (!/^[a-z0-9][a-z0-9-]{1,63}$/.test(id)) {
    throw new Error("Invalid --id; expected lowercase letters, numbers, and hyphens");
  }

  const outputDir = toAbsolutePath(optionalString("output-dir", path.join("video-packages", id)));
  const title = optionalString("title", slugifyTitle(id));
  const category = optionalString("category", "copilot");
  const description = optionalString("description", "");
  const language = optionalString("language", "nb");
  const sortOrder = optionalInteger("sort-order", 100);
  const publishedAt = optionalString("published-at", new Date().toISOString());
  const durationOverride = optionalNumber("duration-sec");
  const posterTimeOverride = optionalNumber("poster-time");
  const series = (arg("series") ?? "").trim();
  const season = optionalPositiveInteger("season");
  const episode = optionalPositiveInteger("episode");
  const tags = parseTags(arg("tags"));
  if ((season === undefined) !== (episode === undefined)) {
    throw new Error("When using season/episode, both --season and --episode must be set");
  }
  if ((season !== undefined || episode !== undefined) && !series) {
    throw new Error("--series is required when --season/--episode is set");
  }

  const probe = probeVideo(inputFile);
  const durationSec = durationOverride !== undefined ? Math.max(1, Math.round(durationOverride)) : probe.durationSec;
  const posterTime = derivePosterAt(durationSec, posterTimeOverride);

  fs.rmSync(outputDir, { recursive: true, force: true });
  fs.mkdirSync(outputDir, { recursive: true });
  fs.mkdirSync(path.join(outputDir, "hls", "segments"), { recursive: true });

  const posterFile = path.join(outputDir, "poster.jpg");
  const hlsDir = path.join(outputDir, "hls");
  const hlsMasterFile = path.join(hlsDir, "master.m3u8");
  const mp4File = path.join(outputDir, "video.mp4");
  const metadataFile = path.join(outputDir, "video-package.json");
  const publishScript = path.join(outputDir, "publish.sh");

  fs.copyFileSync(inputFile, mp4File);

  execFileSync(
    "ffmpeg",
    ["-y", "-ss", String(posterTime), "-i", inputFile, "-frames:v", "1", "-vf", hlsFilter(), posterFile],
    { stdio: "inherit" }
  );

  execFileSync(
    "ffmpeg",
    [
      "-y",
      "-i",
      inputFile,
      "-map",
      "0:v:0",
      "-map",
      "0:a?",
      "-c:v",
      "libx264",
      "-profile:v",
      "main",
      "-pix_fmt",
      "yuv420p",
      "-preset",
      "veryfast",
      "-crf",
      "20",
      "-c:a",
      "aac",
      "-b:a",
      "128k",
      "-ar",
      "48000",
      "-ac",
      "2",
      "-vf",
      hlsFilter(),
      "-g",
      "48",
      "-keyint_min",
      "48",
      "-sc_threshold",
      "0",
      "-f",
      "hls",
      "-hls_time",
      "4",
      "-hls_playlist_type",
      "vod",
      "-hls_list_size",
      "0",
      "-hls_segment_filename",
      path.join(hlsDir, "segments", "segment_%03d.ts"),
      hlsMasterFile,
    ],
    { stdio: "inherit" }
  );

  const publishMetadata: {
    series?: string;
    season?: number;
    episode?: number;
    tags?: string[];
  } = {};
  if (series) publishMetadata.series = series;
  if (season !== undefined) publishMetadata.season = season;
  if (episode !== undefined) publishMetadata.episode = episode;
  if (tags.length > 0) publishMetadata.tags = tags;

  const metadata = {
    id,
    title,
    description,
    category,
    language,
    duration_sec: durationSec,
    sort_order: sortOrder,
    published_at: publishedAt,
    input_file: inputFile,
    output_dir: outputDir,
    poster_file: posterFile,
    hls_file: hlsMasterFile,
    mp4_file: mp4File,
    aspect_ratio: "9:16",
    dimensions: { width: probe.width, height: probe.height },
    publish_metadata: publishMetadata,
  };
  writeFile(metadataFile, `${JSON.stringify(metadata, null, 2)}\n`);

  const appRoot = path.resolve(import.meta.dirname, "..");
  const publishArgs = [
    `--id ${quote(id)}`,
    `--title ${quote(title)}`,
    `--category ${quote(category)}`,
    `--duration-sec ${quote(String(durationSec))}`,
    '--poster-file "${OUT_DIR}/poster.jpg"',
    '--hls-file "${OUT_DIR}/hls/master.m3u8"',
    '--hls-segments-dir "${OUT_DIR}/hls/segments"',
    '--mp4-file "${OUT_DIR}/video.mp4"',
    `--language ${quote(language)}`,
    `--sort-order ${quote(String(sortOrder))}`,
    `--published-at ${quote(publishedAt)}`,
  ];
  if (description) publishArgs.push(`--description ${quote(description)}`);
  if (series) publishArgs.push(`--series ${quote(series)}`);
  if (season !== undefined) publishArgs.push(`--season ${quote(String(season))}`);
  if (episode !== undefined) publishArgs.push(`--episode ${quote(String(episode))}`);
  if (tags.length > 0) publishArgs.push(`--tags ${quote(tags.join(","))}`);
  const publishArgsLines = publishArgs.map(
    (entry, index) => `  ${entry}${index < publishArgs.length - 1 ? " \\" : ""}`
  );

  const publishScriptLines = [
    "#!/usr/bin/env bash",
    "set -euo pipefail",
    "",
    `APP_ROOT=${quote(appRoot)}`,
    `OUT_DIR=${quote(outputDir)}`,
    `ID=${quote(id)}`,
    "",
    'VIDEO_BUCKET_PUBLIC="${VIDEO_BUCKET_PUBLIC:-${VIDEO_BUCKET_PUBLIC_DEV:-}}"',
    'if [[ -z "${VIDEO_BUCKET_PUBLIC:-}" ]]; then',
    '  echo "Set VIDEO_BUCKET_PUBLIC or VIDEO_BUCKET_PUBLIC_DEV before publishing" >&2',
    "  exit 1",
    "fi",
    "",
    "check_bucket_access() {",
    '  local bucket="$1"',
    "  local ok=0",
    "",
    "  if command -v gcloud >/dev/null 2>&1; then",
    '    if gcloud storage ls "gs://${bucket}" >/dev/null 2>&1; then',
    "      ok=1",
    "    fi",
    "  fi",
    "",
    '  if [[ "${ok}" -eq 0 ]] && command -v gsutil >/dev/null 2>&1; then',
    '    if gsutil ls "gs://${bucket}" >/dev/null 2>&1; then',
    "      ok=1",
    "    fi",
    "  fi",
    "",
    '  if [[ "${ok}" -eq 0 ]]; then',
    '    echo "Could not verify authenticated access to gs://${bucket} with gcloud or gsutil." >&2',
    '    echo "Authenticate first (for example: gcloud auth login and/or gcloud auth application-default login), then retry." >&2',
    "    exit 1",
    "  fi",
    "}",
    "",
    'check_bucket_access "${VIDEO_BUCKET_PUBLIC}"',
    "",
    'cd "${APP_ROOT}"',
    'VIDEO_BUCKET_PUBLIC="${VIDEO_BUCKET_PUBLIC}" VIDEO_PUBLISH_ENV=dev node --experimental-strip-types scripts/publish-video.ts \\',
    ...publishArgsLines,
  ];
  const publishScriptContent = `${publishScriptLines.join("\n")}\n`;
  writeFile(publishScript, publishScriptContent);
  fs.chmodSync(publishScript, 0o755);

  process.stdout.write(`Prepared video package in ${outputDir}\n`);
  process.stdout.write(`Metadata: ${metadataFile}\n`);
  process.stdout.write(`Publish script: ${publishScript}\n`);
  process.stdout.write(`HLS master: ${hlsMasterFile}\n`);
  process.stdout.write(`Poster: ${posterFile}\n`);
  process.stdout.write(`MP4 copy: ${mp4File}\n`);
}

main();
