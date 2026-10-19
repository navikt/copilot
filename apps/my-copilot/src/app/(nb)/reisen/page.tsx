import { BodyLong, BodyShort, Heading, Link, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { formatDate } from "@/lib/format";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { getAllCustomizations } from "@/lib/customizations";
import { getNewsItems } from "@/lib/news";
import { PHASES } from "./milestones";

export const metadata: Metadata = {
  title: "Reisen: hva Nav har bygget med KI-agenter",
  description: "Tidslinjen for KI-arbeidet i navikt/copilot: kode og agenter, med kilde for hvert steg.",
};

const REPO = "https://github.com/navikt/copilot";

const monthFormat = new Intl.DateTimeFormat("nb-NO", { month: "long", year: "numeric", timeZone: "UTC" });

/** «8. oktober 2026», or «juli–oktober 2026» for an item that spans several months. */
function formatWhen(date: string, end?: string): string {
  if (date.length === 4) return date;
  if (date.length === 7 && !end) return monthFormat.format(new Date(date));
  if (!end) return formatDate(date);
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
    <InsightPage
      label="Innsikt"
      title="Reisen"
      description="Mange KI-satsinger er lysbilder. Vår er kode og agenter i et åpent repo, med målinger."
      intro={
        <>
          Nav har bygget agenter, regler og verktøy som utviklere bruker hver dag. Alt ligger i{" "}
          <Link href={REPO}>navikt/copilot</Link>. Hver fase i tidslinjen lenker til koden, pull requestene og
          kunngjøringene bak den.
        </>
      }
      source="Det som ikke har lenke, bygger på teamets egen beskrivelse."
      width="prose"
    >
      {/* TODO(#1512): bransjens tidslinje kommer når hver linje har en kilde. */}

      <InsightSection id="tidslinje" title="Tidslinje">
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
              <Heading size="small" level="3" spacing>
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
              {m.figures && (
                <div className="grid gap-4 sm:grid-cols-2 mb-3">
                  {m.figures.map((f) => (
                    <figure key={f.src} className={f.narrow ? "m-0 max-w-[320px]" : "m-0"}>
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img
                        src={f.src}
                        alt={f.alt}
                        loading="lazy"
                        className="w-full h-auto rounded border border-[var(--ax-border-neutral-subtle)]"
                      />
                      <figcaption>
                        <BodyShort size="small" textColor="subtle">
                          {f.href ? <Link href={f.href}>{f.caption}</Link> : f.caption}
                        </BodyShort>
                      </figcaption>
                    </figure>
                  ))}
                </div>
              )}
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
      </InsightSection>

      <InsightSection id="tall" title="Tall vi kan vise fram">
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
      </InsightSection>

      <InsightSection id="sporsmal" title="Spørsmål vi tar med videre">
        <BodyLong spacing>
          Denne siden handler om KI i produktutviklingen. Den er ikke Navs samlede KI-strategi. Spørsmålene under styrer
          hva vi prøver ut og måler. De er ikke beslutninger.
        </BodyLong>
        <VStack as="ol" gap="space-8" className="list-decimal list-inside">
          <li>
            <BodyShort as="span">
              Hvilke resultater for brukerne og Nav skal agentisk KI forbedre, utover spart tid og mer kode?
            </BodyShort>
          </li>
          <li>
            <BodyShort as="span">
              Hvordan bør ansvar, roller og kompetanse endres når agentene skriver mer av koden?
            </BodyShort>
          </li>
          <li>
            <BodyShort as="span">
              Mer kode kan bli en byrde. Hvordan unngår vi at raskere utvikling gir mer vedlikehold og flere systemer
              enn vi trenger?
            </BodyShort>
          </li>
          <li>
            <BodyShort as="span">
              Hvor går grensene for hva agentene får gjøre, og hvor skal mennesker bestemme?
            </BodyShort>
          </li>
          <li>
            <BodyShort as="span">
              Billigere modeller kan gi større regninger når bruken øker. Hvordan balanserer vi kvalitet, kostnad,
              sikkerhet og kontroll, målt per løst oppgave?
            </BodyShort>
          </li>
        </VStack>
      </InsightSection>

      <InsightSection id="teamet" title="Teamet">
        <BodyLong spacing>
          Det startet som en grasrotbevegelse, med et fellesskap fra dag én.{" "}
          <Link href="https://github.com/Starefossen">Hans Kristian Flaatten</Link> driver arbeidet sammen med teamet
          bak navikt/copilot. Teamet måler før det bestemmer, skriver ned det som ikke virket og jobber i et åpent repo.
        </BodyLong>
        <BodyLong>
          Produktteamene i Nav fantes lenge før Copilot. De er en av de viktigste grunnene til at så mye av dette har
          lyktes.
        </BodyLong>
      </InsightSection>
    </InsightPage>
  );
}
