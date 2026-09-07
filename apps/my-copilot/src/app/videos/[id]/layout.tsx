import { notFound } from "next/navigation";
import { fetchVideoById } from "@/lib/public-videos";

type Props = {
  params: Promise<{ id: string }>;
  children: React.ReactNode;
};

// The existence check lives here rather than in page.tsx because loading.tsx
// wraps the page in a Suspense boundary. Anything thrown inside that boundary
// arrives after the shell has streamed with status 200, so notFound() rendered
// the right page under the wrong status and told search engines it exists.
// A layout renders outside the boundary. Next dedupes identical fetch calls
// within one render, so the page's own fetchVideoById makes no second request:
// measured against a counting stub, one page load is one API call.
export default async function VideoLayout({ params, children }: Props) {
  const { id } = await params;
  const video = await fetchVideoById(id);

  if (!video) notFound();

  return children;
}
