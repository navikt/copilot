import { execFileSync } from "node:child_process";
import fs from "node:fs";
import {
  arg,
  parseNonNegativeInteger,
  parseOptionalPositiveInteger,
  parsePositiveInteger,
  parseTags,
  required,
} from "./video-cli-common.ts";

type ManifestItem = {
  id: string;
  title: string;
  description: string;
  category: string;
  published_at: string;
  duration_sec: number;
  aspect_ratio: string;
  language: string;
  poster_object: string;
  hls_master_object: string;
  mp4_object?: string;
  captions_object: string;
  is_published: boolean;
  sort_order: number;
  metadata?: VideoMetadata;
};

type VideoMetadata = {
  series?: string;
  season?: number;
  episode?: number;
  tags?: string[];
};

type PublishEnvironment = "dev" | "prod";

function resolvePublishEnvironment(): PublishEnvironment {
  const value = arg("environment") ?? process.env.VIDEO_PUBLISH_ENV ?? process.env.NAIS_CLUSTER_NAME ?? "";
  switch (value.toLowerCase()) {
    case "dev":
    case "dev-gcp":
      return "dev";
    case "prod":
    case "prod-gcp":
      return "prod";
    default:
      throw new Error("Missing or invalid --environment (expected dev or prod)");
  }
}

function resolveEnvValue(baseName: string, environment: PublishEnvironment): string {
  const value = process.env[`${baseName}_${environment.toUpperCase()}`] ?? process.env[baseName];
  if (!value) {
    throw new Error(`Missing required environment variable ${baseName}_${environment.toUpperCase()}`);
  }
  return value;
}

function gsPath(bucket: string, objectPath: string): string {
  return `gs://${bucket}/${objectPath}`;
}

const VALID_OBJECT_PATH_RE = /^[a-zA-Z0-9][a-zA-Z0-9/_\-.]*$/;

function validateObjectPath(objectPath: string, label: string) {
  if (!VALID_OBJECT_PATH_RE.test(objectPath) || objectPath.includes("..") || objectPath.includes("//")) {
    throw new Error(`Invalid ${label}; object path contains unsupported characters`);
  }
}

function readManifest(manifestTarget: string): ManifestItem[] {
  const readWithGcloud = () => execFileSync("gcloud", ["storage", "cat", manifestTarget], { encoding: "utf8" }).trim();
  const readWithGsutil = () => execFileSync("gsutil", ["cat", manifestTarget], { encoding: "utf8" }).trim();
  try {
    const output = readWithGcloud();
    if (!output) {
      return [];
    }
    return JSON.parse(output) as ManifestItem[];
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (
      message.includes("No URLs matched") ||
      message.includes("matched no objects") ||
      message.includes("No such object")
    ) {
      return [];
    }
    try {
      const output = readWithGsutil();
      if (!output) {
        return [];
      }
      return JSON.parse(output) as ManifestItem[];
    } catch (fallbackError) {
      const fallbackMessage = fallbackError instanceof Error ? fallbackError.message : String(fallbackError);
      if (
        fallbackMessage.includes("No URLs matched") ||
        fallbackMessage.includes("matched no objects") ||
        fallbackMessage.includes("No such object")
      ) {
        return [];
      }
      throw fallbackError;
    }
  }
}

function writeManifest(manifestTarget: string, manifest: ManifestItem[]) {
  const payload = `${JSON.stringify(manifest, null, 2)}\n`;
  const cacheControl = "no-cache, max-age=0, must-revalidate";
  try {
    execFileSync(
      "gcloud",
      ["storage", "cp", "--content-type=application/json", `--cache-control=${cacheControl}`, "-", manifestTarget],
      {
        input: payload,
        stdio: ["pipe", "inherit", "inherit"],
      }
    );
  } catch {
    execFileSync(
      "gsutil",
      ["-h", "Content-Type:application/json", "-h", `Cache-Control:${cacheControl}`, "cp", "-", manifestTarget],
      {
        input: payload,
        stdio: ["pipe", "inherit", "inherit"],
      }
    );
  }
}

function upload(localFile: string, gsTarget: string) {
  try {
    execFileSync("gcloud", ["storage", "cp", localFile, gsTarget], { stdio: "inherit" });
  } catch {
    execFileSync("gsutil", ["cp", localFile, gsTarget], { stdio: "inherit" });
  }
}

function collectFiles(dir: string): string[] {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files: string[] = [];
  for (const entry of entries) {
    const fullPath = `${dir}/${entry.name}`;
    if (entry.isDirectory()) {
      files.push(...collectFiles(fullPath));
      continue;
    }
    if (entry.isFile()) {
      files.push(fullPath);
    }
  }
  return files;
}

function uploadDirectoryContents(localDir: string, bucket: string, gsTargetPrefix: string) {
  if (!fs.existsSync(localDir) || !fs.statSync(localDir).isDirectory()) {
    throw new Error(`--hls-segments-dir does not exist or is not a directory: ${localDir}`);
  }
  const files = collectFiles(localDir);
  for (const filePath of files) {
    const relativePath = filePath.slice(localDir.length + 1).replaceAll("\\", "/");
    validateObjectPath(`${gsTargetPrefix}/${relativePath}`, "hls-segment");
    upload(filePath, gsPath(bucket, `${gsTargetPrefix}/${relativePath}`));
  }
}

function ensurePublicRead(bucket: string) {
  const bucketTarget = `gs://${bucket}`;
  try {
    execFileSync(
      "gcloud",
      [
        "storage",
        "buckets",
        "add-iam-policy-binding",
        bucketTarget,
        "--member=allUsers",
        "--role=roles/storage.objectViewer",
      ],
      { stdio: "inherit" }
    );
  } catch {
    execFileSync("gsutil", ["iam", "ch", "allUsers:objectViewer", bucketTarget], { stdio: "inherit" });
  }
}

function main() {
  const environment = resolvePublishEnvironment();
  const bucketPublic = resolveEnvValue("VIDEO_BUCKET_PUBLIC", environment);
  const manifestTarget = gsPath(bucketPublic, "video_manifest.json");
  ensurePublicRead(bucketPublic);

  const id = required("id");
  if (!/^[a-z0-9][a-z0-9-]{1,63}$/.test(id)) {
    throw new Error("Invalid --id; expected lowercase letters, numbers, and hyphens");
  }
  const title = required("title");
  const description = arg("description") ?? "";
  const category = required("category");
  const language = arg("language") ?? "nb";
  const durationSec = parsePositiveInteger("duration-sec", required("duration-sec"));
  const sortOrder = parseNonNegativeInteger("sort-order", arg("sort-order") ?? "100");
  const publishedAt = arg("published-at") ?? new Date().toISOString();
  const series = (arg("series") ?? "").trim();
  const season = parseOptionalPositiveInteger("season", arg("season"));
  const episode = parseOptionalPositiveInteger("episode", arg("episode"));
  const tags = parseTags(arg("tags"));
  if ((season === undefined) !== (episode === undefined)) {
    throw new Error("When using season/episode, both --season and --episode must be set");
  }
  if ((season !== undefined || episode !== undefined) && !series) {
    throw new Error("--series is required when --season/--episode is set");
  }

  const posterFile = required("poster-file");
  const hlsFile = required("hls-file");
  const mp4File = arg("mp4-file");
  const captionsFile = arg("captions-file");
  const hlsSegmentsDir = arg("hls-segments-dir");

  const targetPrefix = `videos/${id}`;
  const posterObject = `${targetPrefix}/${posterFile.split("/").pop() ?? "poster.jpg"}`;
  const hlsObject = `${targetPrefix}/${hlsFile.split("/").pop() ?? "master.m3u8"}`;
  const mp4Object = mp4File ? `${targetPrefix}/${mp4File.split("/").pop() ?? "video.mp4"}` : "";
  const captionsObject = captionsFile ? `${targetPrefix}/${captionsFile.split("/").pop() ?? "captions.vtt"}` : "";

  validateObjectPath(posterObject, "poster-file");
  validateObjectPath(hlsObject, "hls-file");
  if (mp4Object) validateObjectPath(mp4Object, "mp4-file");
  if (captionsObject) validateObjectPath(captionsObject, "captions-file");

  upload(posterFile, gsPath(bucketPublic, posterObject));
  upload(hlsFile, gsPath(bucketPublic, hlsObject));
  if (mp4File) {
    upload(mp4File, gsPath(bucketPublic, mp4Object));
  }
  if (captionsFile) {
    upload(captionsFile, gsPath(bucketPublic, captionsObject));
  }
  if (hlsSegmentsDir) {
    // Upload segments to the same prefix as master.m3u8 references.
    uploadDirectoryContents(hlsSegmentsDir, bucketPublic, targetPrefix);
  }

  const manifest = readManifest(manifestTarget);
  const metadata: VideoMetadata = {};
  if (series) metadata.series = series;
  if (season !== undefined) metadata.season = season;
  if (episode !== undefined) metadata.episode = episode;
  if (tags.length > 0) metadata.tags = tags;
  const entry: ManifestItem = {
    id,
    title,
    description,
    category,
    published_at: publishedAt,
    duration_sec: durationSec,
    aspect_ratio: "9:16",
    language,
    poster_object: posterObject,
    hls_master_object: hlsObject,
    mp4_object: mp4Object || undefined,
    captions_object: captionsObject,
    is_published: true,
    sort_order: sortOrder,
    metadata: Object.keys(metadata).length > 0 ? metadata : undefined,
  };

  const existingIndex = manifest.findIndex((item) => item.id === id);
  if (existingIndex >= 0) {
    manifest[existingIndex] = entry;
  } else {
    manifest.push(entry);
  }

  writeManifest(manifestTarget, manifest);
  process.stdout.write(`Published video ${id} to ${bucketPublic} and updated manifest: ${manifestTarget}\n`);
}

main();
