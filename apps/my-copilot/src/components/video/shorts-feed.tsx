"use client";

import { HStack, VStack } from "@navikt/ds-react";
import type { HomepageVideo } from "@/lib/public-videos";
import type { PlaybackState } from "@/lib/video-playback-machine";
import {
  type ShortsFeedController,
  type ShortsFeedMediaHandlers,
  useShortsFeedController,
} from "./hooks/use-shorts-feed-controller";
import { VideoPlayerSurface } from "./video-player-surface";

type ShortsFeedProps = {
  videos: HomepageVideo[];
  initialVideoId?: string;
};

// Presentational card. All interaction is delegated to the controller; this
// component only maps the resolved playback state onto chrome.
function ShortsFeedCard({
  video,
  isActive,
  playbackState,
  mediaHandlers,
  setVideoNode,
  setCardNode,
  onOpen,
  onKeyDown,
  onCenterAction,
  onSeekBackward,
  onSeekForward,
  onReplay,
  onFullscreen,
}: {
  video: HomepageVideo;
  isActive: boolean;
  playbackState: PlaybackState;
  mediaHandlers: ShortsFeedMediaHandlers;
  setVideoNode: ShortsFeedController["setVideoNode"];
  setCardNode: ShortsFeedController["setCardNode"];
  onOpen: () => void;
  onKeyDown: (event: React.KeyboardEvent<HTMLDivElement>) => void;
  onCenterAction: () => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  onReplay: () => void;
  onFullscreen: () => void;
}) {
  return (
    <div ref={(node) => setCardNode(video.id, node)} className="group snap-start shrink-0 w-[240px] sm:w-[260px]">
      <VideoPlayerSurface
        video={video}
        isActive={isActive}
        playbackState={playbackState}
        mediaHandlers={mediaHandlers}
        setVideoNode={setVideoNode}
        onOpen={onOpen}
        onKeyDown={onKeyDown}
        onPrimaryAction={onCenterAction}
        onSeekBackward={onSeekBackward}
        onSeekForward={onSeekForward}
        onReplay={onReplay}
        onFullscreen={onFullscreen}
        aspectRatio="9 / 16"
      />
    </div>
  );
}

export function ShortsFeed({ videos, initialVideoId }: ShortsFeedProps) {
  const controller = useShortsFeedController({ videos, initialVideoId });
  const {
    orderedVideos,
    resolvedActiveId,
    isViewerOpen,
    playbackState,
    scrollContainerRef,
    setVideoNode,
    setCardNode,
    mediaHandlers,
    openViewer,
    onPrimaryAction,
    replayPlayback,
    seekPlayback,
    toggleFullscreen,
    handleCardKeyDown,
  } = controller;

  return (
    <VStack gap="space-12">
      <div ref={scrollContainerRef} className="overflow-x-auto overscroll-x-contain snap-x snap-mandatory">
        <HStack gap="space-16" wrap={false} align="start">
          {orderedVideos.map((video) => {
            const isActive = isViewerOpen && resolvedActiveId === video.id;

            return (
              <ShortsFeedCard
                key={video.id}
                video={video}
                isActive={isActive}
                playbackState={playbackState}
                mediaHandlers={mediaHandlers(video.id)}
                setVideoNode={setVideoNode}
                setCardNode={setCardNode}
                onOpen={() => openViewer(video.id)}
                onKeyDown={(event) => handleCardKeyDown(event, video.id)}
                onCenterAction={() => onPrimaryAction(video.id)}
                onSeekBackward={() => seekPlayback(video.id, -5)}
                onSeekForward={() => seekPlayback(video.id, 5)}
                onReplay={() => replayPlayback(video.id)}
                onFullscreen={() => toggleFullscreen(video.id)}
              />
            );
          })}
        </HStack>
      </div>
    </VStack>
  );
}
