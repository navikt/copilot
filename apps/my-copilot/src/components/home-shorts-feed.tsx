"use client";

import type { HomepageVideo } from "@/lib/public-videos";
import { ShortsFeed } from "./shorts-feed";

type HomeShortsFeedProps = {
  videos: HomepageVideo[];
};

export function HomeShortsFeed({ videos }: HomeShortsFeedProps) {
  return <ShortsFeed videos={videos} />;
}
