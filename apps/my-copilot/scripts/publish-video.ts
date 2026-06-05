import { execFileSync } from "node:child_process";

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
};

type PublishEnvironment = "dev" | "prod";

function arg(name: string): string | undefined {
  const index = process.argv.indexOf(`--${name}`);
  if (index < 0) return undefined;
  return process.argv[index + 1];
}

function required(name: string): string {
  const value = arg(name);
  if (!value) {
    throw new Error(`Missing required argument --${name}`);
  }
  return value;
}

function parsePositiveInteger(name: string, value: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`Invalid value for --${name}; expected a positive integer`);
  }
  return parsed;
}

function parseNonNegativeInteger(name: string, value: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) {
    throw new Error(`Invalid value for --${name}; expected a non-negative integer`);
  }
  return parsed;
}

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

function readManifest(manifestTarget: string): ManifestItem[] {
  try {
    const output = execFileSync("gsutil", ["cat", manifestTarget], { encoding: "utf8" }).trim();
    if (!output) {
      return [];
    }
    return JSON.parse(output) as ManifestItem[];
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (message.includes("No URLs matched") || message.includes("matched no objects")) {
      return [];
    }
    throw error;
  }
}

function writeManifest(manifestTarget: string, manifest: ManifestItem[]) {
  const payload = `${JSON.stringify(manifest, null, 2)}\n`;
  execFileSync("gsutil", ["-h", "Content-Type:application/json", "cp", "-", manifestTarget], {
    input: payload,
    stdio: ["pipe", "inherit", "inherit"],
  });
}

function upload(localFile: string, gsTarget: string) {
  execFileSync("gsutil", ["cp", localFile, gsTarget], { stdio: "inherit" });
}

function main() {
  const environment = resolvePublishEnvironment();
  const bucketPublic = resolveEnvValue("VIDEO_BUCKET_PUBLIC", environment);
  const manifestTarget = gsPath(bucketPublic, "video_manifest.json");

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

  const posterFile = required("poster-file");
  const hlsFile = required("hls-file");
  const mp4File = arg("mp4-file");
  const captionsFile = arg("captions-file");

  const targetPrefix = `videos/${id}`;
  const posterObject = `${targetPrefix}/${posterFile.split("/").pop() ?? "poster.jpg"}`;
  const hlsObject = `${targetPrefix}/${hlsFile.split("/").pop() ?? "master.m3u8"}`;
  const mp4Object = mp4File ? `${targetPrefix}/${mp4File.split("/").pop() ?? "video.mp4"}` : "";
  const captionsObject = captionsFile ? `${targetPrefix}/${captionsFile.split("/").pop() ?? "captions.vtt"}` : "";

  upload(posterFile, gsPath(bucketPublic, posterObject));
  upload(hlsFile, gsPath(bucketPublic, hlsObject));
  if (mp4File) {
    upload(mp4File, gsPath(bucketPublic, mp4Object));
  }
  if (captionsFile) {
    upload(captionsFile, gsPath(bucketPublic, captionsObject));
  }

  const manifest = readManifest(manifestTarget);
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
