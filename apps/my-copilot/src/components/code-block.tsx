"use client";

import React from "react";
import { Dialog, DialogBackdrop, DialogPanel, DialogTitle } from "@headlessui/react";
import { XMarkIcon } from "@navikt/aksel-icons";
import { Box } from "@navikt/ds-react";

interface CodeBlockProps {
  filename: string;
  children: string;
  maxHeight?: string;
}

export function CodeBlock({ filename, children, maxHeight }: CodeBlockProps) {
  const [isModalOpen, setIsModalOpen] = React.useState(false);
  const hasMaxHeight = !!maxHeight;

  return (
    <>
      <div className="rounded-lg overflow-hidden border border-gray-700 shadow-lg">
        {/* Title bar */}
        <Box
          paddingBlock="space-4"
          paddingInline="space-8"
          background="default"
          className="bg-[#323233] flex items-center"
          style={{ gap: "8px" }}
        >
          <div className="flex" style={{ gap: "6px" }}>
            <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
            <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
            <div className="w-3 h-3 rounded-full bg-[#28c840]" />
          </div>
          <span className="text-gray-400 text-xs font-mono" style={{ marginLeft: "8px" }}>
            {filename}
          </span>
        </Box>
        {/* Code content with optional max height */}
        <div className="bg-[#1e1e1e] relative">
          <Box
            padding="space-8"
            className="overflow-hidden"
            style={
              hasMaxHeight
                ? ({
                    maxHeight: maxHeight,
                  } as React.CSSProperties)
                : undefined
            }
          >
            <pre className="text-[#d4d4d4] text-xs font-mono whitespace-pre-wrap leading-relaxed">{children}</pre>
          </Box>
          {hasMaxHeight && (
            <>
              <div className="absolute bottom-0 left-0 right-0 h-16 bg-linear-to-t from-[#1e1e1e] to-transparent pointer-events-none" />
              <button
                onClick={() => setIsModalOpen(true)}
                className="absolute left-1/2 -translate-x-1/2 text-xs text-white font-medium hover:bg-blue-600 bg-blue-500 rounded-md transition-all shadow-lg"
                style={{ bottom: "8px", padding: "8px 16px" }}
              >
                Vis hele filen →
              </button>
            </>
          )}
        </div>
      </div>

      {/* Modal Dialog */}
      <Dialog open={isModalOpen} onClose={setIsModalOpen} className="relative z-50">
        <DialogBackdrop
          transition
          className="fixed inset-0 bg-black/95 transition-opacity data-closed:opacity-0 data-enter:duration-300 data-enter:ease-out data-leave:duration-200 data-leave:ease-in"
        />

        <div className="fixed inset-0 z-10 w-screen overflow-y-auto">
          <Box padding="space-8" className="flex min-h-full items-center justify-center">
            <DialogPanel
              transition
              className="relative transform overflow-hidden rounded-lg bg-[#1e1e1e] text-left shadow-xl transition-all data-closed:translate-y-4 data-closed:opacity-0 data-enter:duration-300 data-enter:ease-out data-leave:duration-200 data-leave:ease-in w-full max-w-4xl data-closed:sm:translate-y-0 data-closed:sm:scale-95"
            >
              {/* Modal Title Bar */}
              <Box
                paddingBlock="space-6"
                paddingInline="space-8"
                className="bg-[#323233] flex items-center justify-between border-b border-gray-700"
              >
                <div className="flex items-center" style={{ gap: "8px" }}>
                  <div className="flex gap-1.5">
                    <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
                    <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
                    <div className="w-3 h-3 rounded-full bg-[#28c840]" />
                  </div>
                  <DialogTitle className="text-gray-300 text-sm ml-2 font-mono">{filename}</DialogTitle>
                </div>
                <button
                  onClick={() => setIsModalOpen(false)}
                  className="text-gray-400 hover:text-white transition-colors"
                  aria-label="Lukk modal"
                >
                  <XMarkIcon className="w-5 h-5" />
                </button>
              </Box>

              {/* Modal Code Content */}
              <Box padding="space-12" className="max-h-[80vh] overflow-y-auto">
                <pre className="text-[#d4d4d4] text-sm font-mono whitespace-pre-wrap leading-relaxed">{children}</pre>
              </Box>
            </DialogPanel>
          </Box>
        </div>
      </Dialog>
    </>
  );
}
