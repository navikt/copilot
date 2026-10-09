import { BodyLong, BodyShort, Box, Heading, Link, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { PageHero } from "@/components/page-hero";
import { getAllCustomizations } from "@/lib/customizations";
import { getNewsItems } from "@/lib/news";
import { PHASES } from "./milestones";

export const metadata: Metadata = {
  title: "Reisen: hva Nav har bygget med KI-agenter",
  description: "Tidslinjen for KI-arbeidet i navikt/copilot: kode og agenter, med kilde for hvert steg.",
};

const REPO = "https://github.com/navikt/copilot";

const dateFormat = new Intl.DateTimeFormat("nb-NO", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});
const monthFormat = new Intl.DateTimeFormat("nb-NO", { month: "long", year: "numeric", timeZone: "UTC" });

/** «8. oktober 2026», or «juli–oktober 2026» for an item that spans several months. */
function formatWhen(date: string, end?: string): string {
  if (date.length === 4) return date;
  if (date.length === 7 && !end) return monthFormat.format(new Date(date));
  if (!end) return dateFormat.format(new Date(date));
  const [from, to] = [monthFormat.format(new Date(date)), monthFormat.format(new Date(end))];
  if (from === to) return from;
  const sameYear = date.slice(0, 4) === end.slice(0, 4);
  return `${sameYear ? from.replace(/ \d{4}$/, "") : from}–${to}`;
}

// TODO(#1511): legg til tall om bruk og kostnad når de er godkjent for publisering.
const nb = (n: number) => n.toLocaleString("nb-NO");

// Read per request, like /modeller: the catalog and the articles ship with the image.
export default function ReisenPage() {
  const items = getAllCustomizations();
  const count = (type: string) => items.filter((item) => item.type === type).length;
  const numbers = [
    { value: nb(count("skill")), label: "skills i katalogen", url: "/verktoy" },
    { value: nb(count("agent")), label: "agenter i katalogen", url: "/verktoy" },
    { value: nb(count("instruction")), label: "instruksjoner i katalogen", url: "/verktoy" },
    { value: nb(getNewsItems().length), label: "nyhetssaker på norsk", url: "/nyheter" },
  ];

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        label="Innsikt"
        title="Reisen"
        description="Mange KI-satsinger er lysbilder. Vår er kode og agenter i et åpent repo, med målinger."
      />
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-3xl mx-auto"
      >
        <VStack gap="space-40">
          <BodyLong>
            Nav har bygget agenter, regler og verktøy som utviklere bruker hver dag. Alt ligger i{" "}
            <Link href={REPO}>navikt/copilot</Link>. Hver fase i tidslinjen lenker til koden, pull requestene og
            kunngjøringene bak den. Det som ikke har lenke, bygger på teamets egen beskrivelse.
          </BodyLong>

          {/* TODO(#1512): bransjens tidslinje kommer når hver linje har en kilde. */}

          <section aria-labelledby="tidslinje">
            <Heading size="large" level="2" id="tidslinje" spacing>
              Tidslinje
            </Heading>
            <VStack
              as="ol"
              gap="space-32"
              aria-labelledby="tidslinje"
              className="border-l-2 border-[var(--ax-border-neutral-subtle)]"
              style={{ paddingInlineStart: "var(--ax-space-24)" }}
            >
              {PHASES.map((m) => (
                <li key={m.title} className="relative">
                  <span
                    aria-hidden
                    className="absolute -left-[35px] top-1 size-5 rounded-full bg-[var(--ax-bg-accent-strong)]"
                  />
                  <BodyShort size="small" textColor="subtle">
                    <time dateTime={m.date}>{formatWhen(m.date, m.end)}</time>
                  </BodyShort>
                  <Heading size="medium" level="3" spacing>
                    {m.title}
                  </Heading>
                  <BodyLong spacing>{m.text}</BodyLong>
                  <BodyShort spacing>
                    <strong>Status:</strong> {m.status}
                  </BodyShort>
                  <ul aria-label={`Viktige milepæler: ${m.title}`} className="list-none mb-3">
                    {m.milestones.map((ms) => (
                      <li key={ms.text}>
                        <BodyShort size="small">
                          <time dateTime={ms.date} className="font-semibold">
                            {formatWhen(ms.date)}
                          </time>
                          {" – "}
                          {ms.url ? <Link href={ms.url}>{ms.text}</Link> : ms.text}
                        </BodyShort>
                      </li>
                    ))}
                  </ul>
                  <BodyShort size="small">
                    {m.sources.length > 1 ? "Kilder:" : "Kilde:"}{" "}
                    {m.sources.map((s, i) => (
                      <span key={"url" in s ? s.url : "team"}>
                        {"url" in s ? <Link href={s.url}>{s.label}</Link> : "teamets egen beskrivelse"}
                        {i < m.sources.length - 1 && ", "}
                      </span>
                    ))}
                  </BodyShort>
                </li>
              ))}
            </VStack>
          </section>

          <section aria-labelledby="tall">
            <Heading size="large" level="2" id="tall" spacing>
              Tall vi kan vise fram
            </Heading>
            <dl className="grid gap-4 sm:grid-cols-2">
              {numbers.map((n) => (
                <div key={n.label} className="flex flex-col-reverse">
                  <dt>
                    <Link href={n.url}>{n.label}</Link>
                  </dt>
                  <dd className="text-3xl font-semibold">{n.value}</dd>
                </div>
              ))}
            </dl>
          </section>

          <section aria-labelledby="teamet">
            <Heading size="large" level="2" id="teamet" spacing>
              Teamet
            </Heading>
            <BodyLong spacing>
              Det startet som en grasrotbevegelse, med et fellesskap fra dag én.{" "}
              <Link href="https://github.com/Starefossen">Hans Kristian Flaatten</Link> driver arbeidet sammen med
              teamet bak navikt/copilot. Teamet måler før det bestemmer, skriver ned det som ikke virket og jobber i et
              åpent repo.
            </BodyLong>
            <BodyLong>
              Produktteamene i Nav fantes lenge før Copilot. De er en av de viktigste grunnene til at så mye av dette
              har lyktes.
            </BodyLong>
          </section>
        </VStack>
      </Box>
    </main>
  );
}
