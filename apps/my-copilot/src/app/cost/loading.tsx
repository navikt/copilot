import { Heading, Skeleton } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main className="p-6 mx-4 max-w-7xl">
      <section className="mb-8">
        <Heading size="xlarge" level="1" className="mb-6">
          Lisensoversikt
        </Heading>
        <div className="space-y-3 mb-8">
          <Skeleton variant="text" width="70%" />
          <Skeleton variant="text" width="60%" />
        </div>

        <div className="bg-white rounded-lg border border-gray-200 p-6 shadow-sm">
          <Skeleton variant="text" width="40%" className="mb-4" />

          <div className="space-y-4">
            {[...Array(10)].map((_, i) => (
              <div key={i} className="flex items-center gap-4 p-4 bg-gray-50 rounded">
                <Skeleton variant="circle" width={40} height={40} />
                <div className="flex-1 space-y-2">
                  <Skeleton variant="text" width="30%" />
                  <Skeleton variant="text" width="50%" />
                </div>
                <Skeleton variant="rectangle" width={80} height={32} />
              </div>
            ))}
          </div>
        </div>
      </section>
    </main>
  );
}
