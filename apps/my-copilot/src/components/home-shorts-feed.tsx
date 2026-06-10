"use client";

import { useSyncExternalStore } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import { ShortsFeed } from "./shorts-feed";

type HomeShortsFeedProps = {
  videos: HomepageVideo[];
};

export function HomeShortsFeed({ videos }: HomeShortsFeedProps) {
  const isClient = useSyncExternalStore(
    () => () => {},
    () => true,
    () => false
  );

  if (!isClient) return null;

  return <ShortsFeed videos={videos} />;
}
