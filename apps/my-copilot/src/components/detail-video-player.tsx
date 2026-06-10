"use client";

import type { HomepageVideo } from "@/lib/public-videos";
import { useDetailPageController } from "./use-detail-page-controller";
import { VideoPlayerSurface } from "./video-player-surface";

interface DetailVideoPlayerProps {
  video: HomepageVideo;
}

export function DetailVideoPlayer({ video }: DetailVideoPlayerProps) {
  const {
    playbackState,
    mediaHandlers,
    setVideoNode,
    onTogglePlayback,
    onSeekBackward,
    onSeekForward,
    onReplay,
    onFullscreen,
  } = useDetailPageController({ video });

  return (
    <VideoPlayerSurface
      video={video}
      isActive={true}
      playbackState={playbackState}
      mediaHandlers={mediaHandlers}
      setVideoNode={setVideoNode}
      onPrimaryAction={onTogglePlayback}
      onSeekBackward={onSeekBackward}
      onSeekForward={onSeekForward}
      onReplay={onReplay}
      onFullscreen={onFullscreen}
      hudHideDelayMs={3000}
      hudLeaveDelayMs={500}
    />
  );
}
