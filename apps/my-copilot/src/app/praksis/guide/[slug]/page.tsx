import { allGuides } from "../../data";
import { notFound } from "next/navigation";
import { Heading, VStack } from "@navikt/ds-react";
import { BackToTop } from "@/components/back-to-top";

export function generateStaticParams() {
  return allGuides.map((g) => ({ slug: g.id }));
}

export default async function GuidePage(props: { params: Promise<{ slug: string }> }) {
  const params = await props.params;
  const guide = allGuides.find((g) => g.id === params.slug);

  if (!guide) {
    notFound();
  }

  return (
    <div>
      <VStack gap="space-32">
        <div>
          <Heading level="1" size="xlarge" spacing>
            {guide.title}
          </Heading>
          <p className="text-xl text-gray-600 leading-relaxed max-w-3xl">{guide.description}</p>
        </div>

        <VStack gap="space-48" className="mt-8">
          {guide.components.map((Component, i) => (
            <Component key={i} />
          ))}
        </VStack>
      </VStack>
      <BackToTop />
    </div>
  );
}
