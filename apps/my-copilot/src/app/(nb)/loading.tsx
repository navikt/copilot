import { Heading, Skeleton } from "@navikt/ds-react";

// This skeleton is the front page's own, so it lives in the (home) group and
// its Suspense boundary covers only the front page. At the app root it wrapped
// every route, and a boundary above a page means the shell streams with status
// 200 before notFound() can throw: an unknown URL answered 200 with the
// not-found page drawn client side.

export default function Loading() {
  return (
    <main className="p-4 mx-4">
      <section className="mb-8">
        <Heading size="xlarge" level="1" className="mb-4">
          GitHub Copilot
        </Heading>
        <div className="space-y-3 mb-8">
          <Skeleton variant="text" width="90%" />
          <Skeleton variant="text" width="85%" />
        </div>

        <div className="mb-4">
          <Skeleton variant="text" width="40%" className="mb-2" />
          <div className="space-y-2">
            <Skeleton variant="text" width="60%" />
            <Skeleton variant="text" width="55%" />
            <Skeleton variant="text" width="65%" />
          </div>
        </div>
      </section>

      <section className="mb-8">
        <Skeleton variant="text" width="30%" className="mb-4" />
        <div className="bg-white rounded-lg border border-gray-200 p-6 shadow-sm">
          <Skeleton variant="rectangle" height={200} />
        </div>
      </section>
    </main>
  );
}
