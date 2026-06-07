const WATCH_STATE_STORAGE_KEY = "my-copilot:shorts:watch-state:v1";
const WATCH_STATE_VERSION = 1;
const WATCHED_THRESHOLD_PCT = 80;
const MAX_WATCH_STATE_ENTRIES = 500;
const RETENTION_DAYS = 180;

export type WatchStatus = {
  watched: boolean;
  watchedAt?: string;
  progressPct: number;
  lastPositionSec: number;
  durationSec?: number;
  lastSeenAt: string;
};

export type WatchStateV1 = {
  version: 1;
  updatedAt: string;
  videos: Record<string, WatchStatus>;
};

export type WatchOrderingMode = "deprioritize" | "hide";

type UpsertProgressParams = {
  state: WatchStateV1;
  videoId: string;
  currentTimeSec: number;
  durationSec?: number;
  now?: Date;
};

type MarkWatchedParams = {
  state: WatchStateV1;
  videoId: string;
  durationSec?: number;
  now?: Date;
};

function defaultState(now: Date = new Date()): WatchStateV1 {
  return {
    version: WATCH_STATE_VERSION,
    updatedAt: now.toISOString(),
    videos: {},
  };
}

function clampPercent(value: number): number {
  return Math.max(0, Math.min(100, Math.round(value)));
}

function toSeconds(value: number | undefined): number | undefined {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) {
    return undefined;
  }
  return Math.floor(value);
}

function canUseStorage(): boolean {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

function parseISO(value: string | undefined): number {
  if (!value) return Number.NaN;
  return Date.parse(value);
}

function migrate(raw: unknown, now: Date = new Date()): WatchStateV1 {
  if (!raw || typeof raw !== "object") {
    return defaultState(now);
  }

  const candidate = raw as Partial<WatchStateV1>;
  if (candidate.version !== WATCH_STATE_VERSION || !candidate.videos || typeof candidate.videos !== "object") {
    return defaultState(now);
  }

  const next: WatchStateV1 = defaultState(now);
  const videos = candidate.videos as Record<string, WatchStatus>;

  for (const [videoId, status] of Object.entries(videos)) {
    if (!status || typeof status !== "object") continue;

    const lastSeenAt = typeof status.lastSeenAt === "string" ? status.lastSeenAt : now.toISOString();
    const watched = Boolean(status.watched);
    const progressPct = clampPercent(typeof status.progressPct === "number" ? status.progressPct : 0);
    const lastPositionSec = Math.max(
      0,
      Math.floor(typeof status.lastPositionSec === "number" ? status.lastPositionSec : 0)
    );
    const durationSec = toSeconds(status.durationSec);
    const watchedAt = typeof status.watchedAt === "string" ? status.watchedAt : undefined;

    next.videos[videoId] = {
      watched,
      watchedAt,
      progressPct,
      lastPositionSec,
      durationSec,
      lastSeenAt,
    };
  }

  return pruneWatchState(next, now);
}

export function loadWatchState(now: Date = new Date()): WatchStateV1 {
  if (!canUseStorage()) {
    return defaultState(now);
  }

  try {
    const raw = window.localStorage.getItem(WATCH_STATE_STORAGE_KEY);
    if (!raw) return defaultState(now);
    const parsed = JSON.parse(raw) as unknown;
    return migrate(parsed, now);
  } catch {
    try {
      window.localStorage.removeItem(WATCH_STATE_STORAGE_KEY);
    } catch {
      // Ignore storage failures.
    }
    return defaultState(now);
  }
}

function pruneWatchState(state: WatchStateV1, now: Date = new Date()): WatchStateV1 {
  const retentionCutoff = now.getTime() - RETENTION_DAYS * 24 * 60 * 60 * 1000;
  const keptEntries = Object.entries(state.videos)
    .filter(([, status]) => {
      const seenAt = parseISO(status.lastSeenAt);
      if (!Number.isFinite(seenAt)) return true;
      return seenAt >= retentionCutoff;
    })
    .sort(([, a], [, b]) => {
      const aSeen = parseISO(a.lastSeenAt);
      const bSeen = parseISO(b.lastSeenAt);
      if (!Number.isFinite(aSeen) || !Number.isFinite(bSeen)) return 0;
      return bSeen - aSeen;
    })
    .slice(0, MAX_WATCH_STATE_ENTRIES);

  const prunedVideos: Record<string, WatchStatus> = {};
  for (const [videoId, status] of keptEntries) {
    prunedVideos[videoId] = status;
  }

  return {
    ...state,
    updatedAt: now.toISOString(),
    videos: prunedVideos,
  };
}

export function saveWatchState(state: WatchStateV1, now: Date = new Date()): void {
  if (!canUseStorage()) return;

  try {
    const pruned = pruneWatchState(state, now);
    window.localStorage.setItem(WATCH_STATE_STORAGE_KEY, JSON.stringify(pruned));
  } catch {
    // Ignore storage failures (quota/private mode).
  }
}

export function getWatchStatus(state: WatchStateV1, videoId: string): WatchStatus | undefined {
  return state.videos[videoId];
}

export function isWatched(status: WatchStatus | undefined): boolean {
  return Boolean(status?.watched);
}

export function upsertProgress(params: UpsertProgressParams): WatchStateV1 {
  const now = params.now ?? new Date();
  const previous = getWatchStatus(params.state, params.videoId);
  const durationSec = toSeconds(params.durationSec);
  const currentSec = Math.max(0, Math.floor(params.currentTimeSec));
  const rawProgressPct =
    durationSec && durationSec > 0 ? (currentSec / durationSec) * 100 : (previous?.progressPct ?? 0);
  const progressPct = clampPercent(rawProgressPct);
  const watched = Boolean(previous?.watched) || rawProgressPct >= WATCHED_THRESHOLD_PCT;

  const nextStatus: WatchStatus = {
    watched,
    watchedAt: watched ? (previous?.watchedAt ?? now.toISOString()) : previous?.watchedAt,
    progressPct,
    lastPositionSec: currentSec,
    durationSec: durationSec ?? previous?.durationSec,
    lastSeenAt: now.toISOString(),
  };

  if (
    previous &&
    previous.watched === nextStatus.watched &&
    previous.watchedAt === nextStatus.watchedAt &&
    previous.progressPct === nextStatus.progressPct &&
    previous.lastPositionSec === nextStatus.lastPositionSec &&
    previous.durationSec === nextStatus.durationSec
  ) {
    return params.state;
  }

  return {
    ...params.state,
    updatedAt: now.toISOString(),
    videos: {
      ...params.state.videos,
      [params.videoId]: nextStatus,
    },
  };
}

export function markWatched(params: MarkWatchedParams): WatchStateV1 {
  const now = params.now ?? new Date();
  const previous = getWatchStatus(params.state, params.videoId);
  const durationSec = toSeconds(params.durationSec) ?? previous?.durationSec;
  const progressPct = Math.max(previous?.progressPct ?? 0, 100);

  const nextStatus: WatchStatus = {
    watched: true,
    watchedAt: previous?.watchedAt ?? now.toISOString(),
    progressPct,
    lastPositionSec: durationSec ?? previous?.lastPositionSec ?? 0,
    durationSec,
    lastSeenAt: now.toISOString(),
  };

  if (
    previous &&
    previous.watched &&
    previous.progressPct === nextStatus.progressPct &&
    previous.lastPositionSec === nextStatus.lastPositionSec &&
    previous.durationSec === nextStatus.durationSec
  ) {
    return params.state;
  }

  return {
    ...params.state,
    updatedAt: now.toISOString(),
    videos: {
      ...params.state.videos,
      [params.videoId]: nextStatus,
    },
  };
}

export function orderVideosByWatchStatus<T extends { id: string }>(
  videos: T[],
  state: WatchStateV1,
  mode: WatchOrderingMode
): T[] {
  if (mode === "hide") {
    return videos.filter((video) => !isWatched(getWatchStatus(state, video.id)));
  }

  const unwatched = videos.filter((video) => !isWatched(getWatchStatus(state, video.id)));
  const watched = videos.filter((video) => isWatched(getWatchStatus(state, video.id)));
  return [...unwatched, ...watched];
}
