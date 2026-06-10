"use client";

import { useCallback, useRef, useState } from "react";
import {
  loadWatchState,
  markWatched,
  saveWatchState,
  upsertProgress,
  type WatchStateV1,
} from "@/lib/video-watch-state";

export type StorageAdapter = {
  watchState: WatchStateV1;
  updateProgress: (videoId: string, currentSecond: number, duration: number | undefined) => void;
  markComplete: (videoId: string, duration: number | undefined) => void;
  flushProgress: (videoId: string, currentSecond: number, duration: number | undefined) => void;
};

export function useStorageAdapter(): StorageAdapter {
  const [watchState, setWatchState] = useState<WatchStateV1>(() => loadWatchState());
  const persistedProgressSecondById = useRef<Map<string, number>>(new Map());

  const updateProgress = useCallback((videoId: string, currentSecond: number, duration: number | undefined) => {
    if (currentSecond <= 0 || currentSecond % 5 !== 0) return;

    const lastPersistedSecond = persistedProgressSecondById.current.get(videoId) ?? -1;
    if (lastPersistedSecond === currentSecond) return;
    persistedProgressSecondById.current.set(videoId, currentSecond);

    setWatchState((prev) => {
      const next = upsertProgress({
        state: prev,
        videoId,
        currentTimeSec: currentSecond,
        durationSec: duration,
      });
      if (next !== prev) {
        saveWatchState(next);
      }
      return next;
    });
  }, []);

  const markComplete = useCallback((videoId: string, duration: number | undefined) => {
    setWatchState((prev) => {
      const next = markWatched({
        state: prev,
        videoId,
        durationSec: duration,
      });
      if (next !== prev) {
        saveWatchState(next);
      }
      return next;
    });
  }, []);

  const flushProgress = useCallback((videoId: string, currentSecond: number, duration: number | undefined) => {
    if (currentSecond <= 0) return;

    const lastPersistedSecond = persistedProgressSecondById.current.get(videoId) ?? -1;
    if (lastPersistedSecond === currentSecond) return;
    persistedProgressSecondById.current.set(videoId, currentSecond);

    setWatchState((prev) => {
      const next = upsertProgress({
        state: prev,
        videoId,
        currentTimeSec: currentSecond,
        durationSec: duration,
      });
      if (next !== prev) {
        saveWatchState(next);
      }
      return next;
    });
  }, []);

  return {
    watchState,
    updateProgress,
    markComplete,
    flushProgress,
  };
}
