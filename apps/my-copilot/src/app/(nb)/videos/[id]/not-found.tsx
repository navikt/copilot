import Link from "next/link";
import { Heading, Box } from "@navikt/ds-react";

export default function NotFound() {
  return (
    <main className="min-h-screen bg-black text-white flex items-center justify-center">
      <Box as="div" paddingBlock="space-24" className="text-center">
        <Box paddingBlock="space-8">
          <Heading level="1" size="xlarge">
            Fant ikke video
          </Heading>
        </Box>
        <Box paddingBlock="space-16">
          <p className="text-white/70">Videoen finnes ikke, eller er ikke offentlig tilgjengelig.</p>
        </Box>
        <Link href="/" className="text-blue-300 hover:underline">
          Tilbake til forsiden
        </Link>
      </Box>
    </main>
  );
}
