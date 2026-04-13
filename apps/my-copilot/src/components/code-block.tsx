"use client";

import React from "react";
import { Dialog, DialogBackdrop, DialogPanel, DialogTitle } from "@headlessui/react";
import { XMarkIcon } from "@navikt/aksel-icons";
import { Box } from "@navikt/ds-react";

interface CodeBlockProps {
  filename?: string;
  children: string;
  maxHeight?: string;
  compact?: boolean;
}

function CopyButton({ text, dark = true }: { text: string; dark?: boolean }) {
  const [copied, setCopied] = React.useState(false);

  const handleCopy = () => {
    navigator.clipboard
      .writeText(text)
      .then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 1500);
      })
      .catch(() => {
        // Clipboard API not available or permission denied — ignore silently
      });
  };

  return (
    <button
      type="button"
      onClick={handleCopy}
      className={`transition-colors ${dark ? "text-gray-400 hover:text-white" : "text-gray-400 hover:text-gray-700"}`}
      aria-label={copied ? "Kopiert" : "Kopier kode"}
      title={copied ? "Kopiert!" : "Kopier"}
    >
      {copied ? (
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="text-green-400"
        >
          <polyline points="20 6 9 17 4 12" />
        </svg>
      ) : (
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
        </svg>
      )}
    </button>
  );
}

export function CodeBlock({ filename, children, maxHeight, compact }: CodeBlockProps) {
  const [isModalOpen, setIsModalOpen] = React.useState(false);
  const hasMaxHeight = !!maxHeight;

  if (compact) {
    return (
      <div className="rounded-lg overflow-hidden border border-gray-200 shadow-sm">
        <div className="bg-[#f1f5f9] relative flex items-start justify-between">
          <Box padding="space-8" className="flex-1 min-w-0">
            <pre className="text-[#334155] text-xs font-mono whitespace-pre-wrap">{children}</pre>
          </Box>
          <Box padding="space-8" className="shrink-0">
            <CopyButton text={children} dark={false} />
          </Box>
        </div>
      </div>
    );
  }

  return (
    <>
      <div className="rounded-lg overflow-hidden border border-gray-700 shadow-lg">
        {/* Title bar */}
        <Box
          paddingBlock="space-4"
          paddingInline="space-8"
          background="default"
          className="bg-[#323233] flex items-center justify-between"
          style={{ gap: "8px" }}
        >
          <div className="flex items-center" style={{ gap: "8px" }}>
            <div className="flex" style={{ gap: "6px" }}>
              <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
              <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
              <div className="w-3 h-3 rounded-full bg-[#28c840]" />
            </div>
            <span className="text-gray-400 text-xs font-mono" style={{ marginLeft: "8px" }}>
              {filename}
            </span>
          </div>
          <CopyButton text={children} />
        </Box>
        {/* Code content with optional max height */}
        <div className="bg-[#1e293b] relative">
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
            <pre className="text-[#d4d4d4] text-xs font-mono whitespace-pre-wrap">{children}</pre>
          </Box>
          {hasMaxHeight && (
            <>
              <div className="absolute bottom-0 left-0 right-0 h-16 bg-linear-to-t from-[#1e293b] to-transparent pointer-events-none" />
              <button
                onClick={() => setIsModalOpen(true)}
                className="absolute left-1/2 -translate-x-1/2 text-xs text-white font-medium hover:bg-blue-600 bg-blue-500 rounded-md transition-all shadow-lg"
                style={{ bottom: "10px", padding: "6px 14px" }}
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
              className="relative transform overflow-hidden rounded-lg bg-[#1e293b] text-left shadow-xl transition-all data-closed:translate-y-4 data-closed:opacity-0 data-enter:duration-300 data-enter:ease-out data-leave:duration-200 data-leave:ease-in w-full max-w-4xl data-closed:sm:translate-y-0 data-closed:sm:scale-95"
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
