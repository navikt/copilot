import { BodyLong, BodyShort, Box, Heading, Link, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { PageHero } from "@/components/page-hero";
import { getAllCustomizations } from "@/lib/customizations";
import { getNewsItems } from "@/lib/news";
import { MILESTONES } from "./milestones";

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
  if (!end) return dateFormat.format(new Date(date));
  const [from, to] = [monthFormat.format(new Date(date)), monthFormat.format(new Date(end))];
  if (from === to) return from;
  const sameYear = date.slice(0, 4) === end.slice(0, 4);
  return `${sameYear ? from.replace(/ \d{4}$/, "") : from}–${to}`;
}

// Repo activity has no data source in the app, so these are snapshots with the date they were taken.
// Commits: `git rev-list --count origin/main`. PRs: GitHub search API, is:pr is:merged.
const SNAPSHOT_DATE = "9. oktober 2026";
const COMMITS = 1501;
const MERGED_PRS = 901;

const nb = (n: number) => n.toLocaleString("nb-NO");

// Read per request, like /modeller: the catalog and the articles ship with the image.
export default function ReisenPage() {
  const items = getAllCustomizations();
  const count = (type: string) => items.filter((item) => item.type === type).length;
  const numbers = [
    {
      value: nb(MERGED_PRS),
      label: `flettede pull requests (${SNAPSHOT_DATE})`,
      url: `${REPO}/pulls?q=is%3Apr+is%3Amerged`,
    },
    { value: nb(COMMITS), label: `commits på main (${SNAPSHOT_DATE})`, url: `${REPO}/commits/main` },
    { value: nb(count("skill")), label: "skills i katalogen", url: "/verktoy" },
    { value: nb(count("agent")), label: "agenter i katalogen", url: "/verktoy" },
    { value: nb(count("instruction")), label: "instruksjoner i katalogen", url: "/verktoy" },
    { value: nb(getNewsItems().length), label: "nyhetssaker", url: "/nyheter" },
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
            <Link href={REPO}>navikt/copilot</Link>. Hvert steg under lenker til koden, pull requesten eller
            kunngjøringen bak det. Forhistorien fra før repoet bygger på teamets egen beskrivelse.
          </BodyLong>

          {/* TODO(#1512): bransjens tidslinje kommer når hver linje har en kilde. */}

          <section aria-labelledby="tidslinje">
            <Heading size="large" level="2" id="tidslinje" spacing>
              Tidslinje
            </Heading>
            <ol aria-labelledby="tidslinje" className="border-l-2 border-[var(--ax-border-neutral-subtle)] pl-6">
              {MILESTONES.map((m) => (
                <li key={m.title} className={`relative last:pb-0 ${m.major ? "pb-10" : "pb-6"}`}>
                  <span
                    aria-hidden
                    className={
                      m.major
                        ? "absolute -left-[35px] top-1 size-5 rounded-full bg-[var(--ax-bg-accent-strong)]"
                        : "absolute -left-[31px] top-1.5 size-3 rounded-full bg-[var(--ax-border-neutral-subtle)]"
                    }
                  />
                  <BodyShort size="small" textColor="subtle">
                    <time dateTime={m.date}>{formatWhen(m.date, m.end)}</time>
                  </BodyShort>
                  <Heading size={m.major ? "medium" : "xsmall"} level="3">
                    {m.title}
                  </Heading>
                  <BodyLong>
                    {m.text} Kilde:{" "}
                    {m.sources.map((s, i) => (
                      <span key={"url" in s ? s.url : "team"}>
                        {i > 0 && ", "}
                        {"url" in s ? <Link href={s.url}>{s.label}</Link> : "teamets egen beskrivelse"}
                      </span>
                    ))}
                  </BodyLong>
                </li>
              ))}
            </ol>
          </section>

          <section aria-labelledby="tall">
            <Heading size="large" level="2" id="tall" spacing>
              Tall vi kan vise fram
            </Heading>
            <BodyShort spacing>
              Bare tall fra det åpne repoet. Tall om bruk og kostnad venter på godkjenning, se{" "}
              <Link href={`${REPO}/issues/1511`}>#1511</Link>.
            </BodyShort>
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
            <BodyLong>
              Alt dette er bygget av teamet bak navikt/copilot. Teamet måler før det bestemmer, skriver ned det som ikke
              virket, og gjør arbeidet i et åpent repo. Derfor kan alle sjekke tallene over.
            </BodyLong>
          </section>
        </VStack>
      </Box>
    </main>
  );
}
