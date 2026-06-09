"use client";

import { useEffect, useRef } from "react";
import { useSearchParams } from "next/navigation";
import type { HomepageVideo } from "@/lib/public-videos";
import type { PlaybackEvent } from "@/lib/video-playback-machine";

type UseUrlSyncAdapterArgs = {
  videos: HomepageVideo[];
  initialActiveId: string;
  isViewerOpen: boolean;
  dispatch: (event: PlaybackEvent) => void;
  setActiveId: (id: string) => void;
  setIsViewerOpen: (open: boolean) => void;
  onOpenViewer: (videoId: string) => void;
};

export function useUrlSyncAdapter({
  videos,
  initialActiveId,
  isViewerOpen,
  dispatch,
  setActiveId,
  setIsViewerOpen,
  onOpenViewer,
}: UseUrlSyncAdapterArgs) {
  const searchParams = useSearchParams();
  const urlControlledViewer = useRef(false);

  useEffect(() => {
    if (!searchParams) return;
    const videoId = searchParams.get("video");

    if (videoId && videos.some((video) => video.id === videoId)) {
      urlControlledViewer.current = true;
      const frame = window.requestAnimationFrame(() => {
        setActiveId(videoId);
        setIsViewerOpen(true);
        dispatch({ type: "OPEN" });
        onOpenViewer(videoId);
      });
      return () => window.cancelAnimationFrame(frame);
    }

    if (urlControlledViewer.current && isViewerOpen) {
      const frame = window.requestAnimationFrame(() => {
        setIsViewerOpen(false);
        dispatch({ type: "CLOSE" });
        setActiveId(initialActiveId);
        urlControlledViewer.current = false;
      });
      return () => window.cancelAnimationFrame(frame);
    }
  }, [searchParams, videos, initialActiveId, isViewerOpen, dispatch, setActiveId, setIsViewerOpen, onOpenViewer]);
}
