import { Skeleton } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main className="min-h-screen bg-white">
      <div className="max-w-7xl mx-auto px-4 py-8">
        <Skeleton variant="rectangle" width="50%" height="40px" className="mb-4" />
        <Skeleton variant="text" width="80%" className="mb-8" />

        <div className="aspect-video bg-gray-200 rounded-lg mb-8" />

        <div className="grid grid-cols-2 gap-4 mb-8">
          {[1, 2, 3, 4].map((i) => (
            <div key={i}>
              <Skeleton variant="text" width="40%" className="mb-2" />
              <Skeleton variant="text" width="60%" />
            </div>
          ))}
        </div>
      </div>
    </main>
  );
}
