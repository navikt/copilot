"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import { BodyShort, Heading, HStack, Skeleton, VStack } from "@navikt/ds-react";
import type { Contributor } from "@/lib/customization-types";

interface ContributorsProps {
  itemId: string;
}

export function Contributors({ itemId }: ContributorsProps) {
  const [contributors, setContributors] = useState<Contributor[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const controller = new AbortController();

    fetch(`/api/contributors?id=${encodeURIComponent(itemId)}`, { signal: controller.signal })
      .then((res) => (res.ok ? res.json() : []))
      .then((data: Contributor[]) => setContributors(data))
      .catch((err) => {
        if (err.name !== "AbortError") setContributors([]);
      })
      .finally(() => setLoading(false));

    return () => controller.abort();
  }, [itemId]);

  if (loading) {
    return (
      <VStack gap="space-8">
        <Heading size="xsmall" level="4">
          Bidragsytere
        </Heading>
        <HStack gap="space-4">
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} variant="circle" width={32} height={32} />
          ))}
        </HStack>
      </VStack>
    );
  }

  if (contributors.length === 0) return null;

  return (
    <VStack gap="space-8">
      <Heading size="xsmall" level="4">
        Bidragsytere
      </Heading>
      <HStack gap="space-8" wrap align="center">
        {contributors.map((c) => (
          <a
            key={c.login}
            href={`https://github.com/${c.login}`}
            target="_blank"
            rel="noopener noreferrer"
            title={c.login}
            className="flex items-center gap-1.5 no-underline hover:underline text-current"
          >
            <Image src={c.avatarUrl} alt="" width={28} height={28} className="rounded-full" />
            <BodyShort size="small">{c.login}</BodyShort>
          </a>
        ))}
      </HStack>
    </VStack>
  );
}
