import type { HomepageVideo } from "@/lib/public-videos";

export const demoVideo: HomepageVideo = {
  id: "nav-pilot-s01e02-context-and-session",
  title: "Kontekst og session-flyt",
  description: "Lær hvordan du styrer kontekst, checkpoints og flyt i Nav Pilot.",
  category: "copilot",
  durationSec: 228,
  language: "nb",
  posterUrl:
    "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e02-context-and-session/poster.jpg",
  playUrl:
    "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e02-context-and-session/master.m3u8",
  mp4Url:
    "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e02-context-and-session/video.mp4",
  aspectRatio: "9 / 16",
  metadata: {
    series: "video-demoer-kost-token-optimalisering",
    season: 1,
    episode: 2,
    tags: ["context", "resume", "copilot-cli"],
    overlay: [
      { kind: "episode-number", anchor: "top-left", labels: ["02"] },
      { kind: "badge", anchor: "top-right", labels: ["✓"] },
      { kind: "chip", anchor: "bottom-full", labels: ["context", "session", "flow"] },
    ],
  },
};

export const relatedVideos: HomepageVideo[] = [
  demoVideo,
  {
    ...demoVideo,
    id: "nav-pilot-s01e01-prompt",
    title: "Prompt-struktur som fungerer",
    durationSec: 165,
    posterUrl: "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e01-prompt/poster.jpg",
    playUrl: "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e01-prompt/master.m3u8",
    mp4Url: "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e01-prompt/video.mp4",
    metadata: {
      ...demoVideo.metadata,
      episode: 1,
      overlay: [{ kind: "episode-number", anchor: "top-left", labels: ["01"] }],
    },
  },
  {
    ...demoVideo,
    id: "nav-pilot-s01e03-modes",
    title: "Riktig modus til riktig jobb",
    durationSec: 291,
    posterUrl: "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e03-modes/poster.jpg",
    playUrl: "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e03-modes/master.m3u8",
    mp4Url: "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-pilot-s01e03-modes/video.mp4",
    metadata: {
      ...demoVideo.metadata,
      episode: 3,
      overlay: [{ kind: "episode-number", anchor: "top-left", labels: ["03"] }],
    },
  },
  {
    ...demoVideo,
    id: "nav-copilot-cplt-bonus-d-cplt-sandbox",
    title: "Bonus D: Cplt sandbox — kom i gang på 3 minutter",
    category: "cplt",
    durationSec: 122,
    posterUrl:
      "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-copilot-cplt-bonus-d-cplt-sandbox/poster.jpg",
    playUrl:
      "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-copilot-cplt-bonus-d-cplt-sandbox/master.m3u8",
    mp4Url:
      "https://storage.googleapis.com/copilot-videos-public-dev/videos/nav-copilot-cplt-bonus-d-cplt-sandbox/video.mp4",
    metadata: {
      ...demoVideo.metadata,
      episode: 4,
      overlay: [{ kind: "episode-number", anchor: "top-left", labels: ["D"] }],
    },
  },
];
