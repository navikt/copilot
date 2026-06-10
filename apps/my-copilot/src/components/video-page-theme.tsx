"use client";

import { useEffect } from "react";

export function VideoPageTheme() {
  useEffect(() => {
    document.body.classList.add("video-detail-active");
    return () => {
      document.body.classList.remove("video-detail-active");
    };
  }, []);

  return null;
}
