import Link from "next/link";
import { Heading, Box } from "@navikt/ds-react";

export default function NotFound() {
  return (
    <main className="min-h-screen bg-white flex items-center justify-center">
      <Box as="div" paddingBlock="space-24" className="text-center">
        <Box paddingBlock="space-8">
          <Heading level="1" size="xlarge">
            Video not found
          </Heading>
        </Box>
        <Box paddingBlock="space-16">
          <p className="text-gray-600">The video you're looking for doesn't exist or is not publicly available.</p>
        </Box>
        <Link href="/" className="text-blue-600 hover:underline">
          Back to home
        </Link>
      </Box>
    </main>
  );
}
