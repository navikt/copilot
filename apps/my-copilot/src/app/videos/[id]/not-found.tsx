import Link from "next/link";
import { Heading, Box } from "@navikt/ds-react";

export default function NotFound() {
  return (
    <main className="min-h-screen bg-white flex items-center justify-center">
      <Box as="div" paddingBlock="space-24" className="text-center">
        <Heading level="1" size="xlarge" className="mb-4">
          Video not found
        </Heading>
        <p className="text-gray-600 mb-8">The video you're looking for doesn't exist or is not publicly available.</p>
        <Link href="/" className="text-blue-600 hover:underline">
          Back to home
        </Link>
      </Box>
    </main>
  );
}
