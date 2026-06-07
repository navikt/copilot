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
  overlay?: VideoOverlayComponent[];
};

type VideoOverlayComponent = {
  kind: string;
  anchor: string;
  labels: string[];
  highlight_index?: number;
  monospace?: boolean;
};

type VideoPackage = {
  id?: string;
  title?: string;
  description?: string;
  category?: string;
  language?: string;
  duration_sec?: number;
  sort_order?: number;
  published_at?: string;
  poster_file?: string;
  hls_file?: string;
  mp4_file?: string;
  captions_file?: string;
  hls_segments_dir?: string;
  publish_metadata?: {
    series?: string;
    season?: number;
    episode?: number;
    tags?: string[];
  };
  overlay?: Array<{
    kind: string;
    anchor: string;
    labels: string[];
    highlight_index?: number;
    highlightIndex?: number;
    monospace?: boolean;
  }>;
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

function parseVideoPackage(filePath: string): VideoPackage {
  if (!fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
    throw new Error(`--package-file does not exist or is not a file: ${filePath}`);
  }
  const raw = fs.readFileSync(filePath, "utf8");
  return JSON.parse(raw) as VideoPackage;
}

function toOptionalInteger(name: string, value: unknown): number | undefined {
  if (value === undefined || value === null || value === "") return undefined;
  const parsed = Number(value);
  if (!Number.isInteger(parsed)) {
    throw new Error(`Invalid value for ${name}; expected an integer`);
  }
  return parsed;
}

function normalizeOverlay(raw: VideoPackage["overlay"]): VideoOverlayComponent[] {
  if (!raw || raw.length === 0) return [];
  return raw.map((component, index) => {
    if (!component || typeof component !== "object") {
      throw new Error(`Invalid overlay component at index ${index}`);
    }
    if (typeof component.kind !== "string" || component.kind.trim() === "") {
      throw new Error(`Invalid overlay kind at index ${index}`);
    }
    if (typeof component.anchor !== "string" || component.anchor.trim() === "") {
      throw new Error(`Invalid overlay anchor at index ${index}`);
    }
    if (!Array.isArray(component.labels) || component.labels.some((label) => typeof label !== "string")) {
      throw new Error(`Invalid overlay labels at index ${index}`);
    }
    const highlightIndex =
      component.highlight_index !== undefined ? component.highlight_index : component.highlightIndex;
    const normalized: VideoOverlayComponent = {
      kind: component.kind,
      anchor: component.anchor,
      labels: component.labels,
      monospace: component.monospace,
    };
    if (highlightIndex !== undefined) {
      normalized.highlight_index = toOptionalInteger(`overlay[${index}].highlight_index`, highlightIndex);
    }
    return normalized;
  });
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

  const packageFile = arg("package-file");
  const videoPackage = packageFile ? parseVideoPackage(packageFile) : undefined;

  const id = arg("id") ?? videoPackage?.id ?? required("id");
  if (!/^[a-z0-9][a-z0-9-]{1,63}$/.test(id)) {
    throw new Error("Invalid --id; expected lowercase letters, numbers, and hyphens");
  }
  const title = arg("title") ?? videoPackage?.title ?? required("title");
  const description = arg("description") ?? videoPackage?.description ?? "";
  const category = arg("category") ?? videoPackage?.category ?? required("category");
  const language = arg("language") ?? videoPackage?.language ?? "nb";
  const durationRaw =
    arg("duration-sec") ?? (videoPackage?.duration_sec !== undefined ? String(videoPackage.duration_sec) : undefined);
  if (!durationRaw) {
    throw new Error("Missing required argument --duration-sec");
  }
  const durationSec = parsePositiveInteger("duration-sec", durationRaw);
  const sortRaw =
    arg("sort-order") ?? (videoPackage?.sort_order !== undefined ? String(videoPackage.sort_order) : "100");
  const sortOrder = parseNonNegativeInteger("sort-order", sortRaw);
  const publishedAt = arg("published-at") ?? videoPackage?.published_at ?? new Date().toISOString();

  const pkgSeries = (videoPackage?.publish_metadata?.series ?? "").trim();
  const series = (arg("series") ?? pkgSeries).trim();
  const season = parseOptionalPositiveInteger(
    "season",
    arg("season") ??
      (videoPackage?.publish_metadata?.season !== undefined ? String(videoPackage.publish_metadata.season) : undefined)
  );
  const episode = parseOptionalPositiveInteger(
    "episode",
    arg("episode") ??
      (videoPackage?.publish_metadata?.episode !== undefined
        ? String(videoPackage.publish_metadata.episode)
        : undefined)
  );
  const tags = parseTags(arg("tags") ?? videoPackage?.publish_metadata?.tags?.join(","));
  if ((season === undefined) !== (episode === undefined)) {
    throw new Error("When using season/episode, both --season and --episode must be set");
  }
  if ((season !== undefined || episode !== undefined) && !series) {
    throw new Error("--series is required when --season/--episode is set");
  }

  const posterFile = arg("poster-file") ?? videoPackage?.poster_file ?? required("poster-file");
  const hlsFile = arg("hls-file") ?? videoPackage?.hls_file ?? required("hls-file");
  const mp4File = arg("mp4-file") ?? videoPackage?.mp4_file;
  const captionsFile = arg("captions-file") ?? videoPackage?.captions_file;
  const hlsSegmentsDir = arg("hls-segments-dir") ?? videoPackage?.hls_segments_dir;

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
  const overlay = normalizeOverlay(videoPackage?.overlay);
  if (overlay.length > 0) metadata.overlay = overlay;
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
