"use client";

import React, { ReactNode, useRef, useState, useEffect } from "react";
import { BodyShort, VStack } from "@navikt/ds-react";

interface CarouselProps {
  children: ReactNode[];
  showIndicators?: boolean;
  showSwipeHint?: boolean;
  className?: string;
}

export function Carousel({ children, showIndicators = true, showSwipeHint = true, className = "" }: CarouselProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [activeIndex, setActiveIndex] = useState(0);

  useEffect(() => {
    const container = scrollRef.current;
    if (!container) return;

    const handleScroll = () => {
      const scrollLeft = container.scrollLeft;
      const itemWidth = container.scrollWidth / children.length;
      const index = Math.round(scrollLeft / itemWidth);
      setActiveIndex(index);
    };

    container.addEventListener("scroll", handleScroll);
    return () => container.removeEventListener("scroll", handleScroll);
  }, [children.length]);

  const scrollToIndex = (index: number) => {
    const container = scrollRef.current;
    if (!container) return;

    const itemWidth = container.scrollWidth / children.length;
    container.scrollTo({
      left: itemWidth * index,
      behavior: "smooth",
    });
  };

  return (
    <div className={`relative ${className}`}>
      {/* Swipe hint */}
      {showSwipeHint && (
        <VStack gap="space-6">
          <div className="flex items-center justify-between">
            <BodyShort className="text-gray-500 text-xs flex items-center">
              <span>←</span> Swipe for flere eksempler <span>→</span>
            </BodyShort>
            {showIndicators && (
              <div className="flex">
                {children.map((_, index) => (
                  <button
                    key={index}
                    onClick={() => scrollToIndex(index)}
                    className={`h-2 rounded-full transition-all duration-300 ${
                      index === activeIndex ? "bg-blue-600 w-5" : "bg-gray-300 w-2"
                    }`}
                    aria-label={`Go to slide ${index + 1} of ${children.length}`}
                    style={{ marginLeft: index > 0 ? "8px" : "0" }}
                  />
                ))}
              </div>
            )}
          </div>
          <div
            ref={scrollRef}
            className="flex overflow-x-auto snap-x snap-mandatory scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent touch-pan-x"
            style={{ touchAction: "pan-x", gap: "16px", paddingBottom: "16px" }}
          >
            {children.map((child, index) => (
              <div key={index} className="shrink-0 w-full md:w-auto snap-start">
                {child}
              </div>
            ))}
          </div>
        </VStack>
      )}
      {/* Right fade */}
      {showSwipeHint && (
        <div className="absolute right-0 top-10 bottom-4 w-16 bg-linear-to-l from-white via-white/80 to-transparent pointer-events-none md:hidden" />
      )}
    </div>
  );
}
