import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Kjente begrensninger i cplt",
  description:
    "Det cplt stenger med vilje: nøkler og hemmeligheter, sky-CLI-er, Docker, nettverk og git-operasjoner. Med det du kan gjøre i stedet.",
};

const TOC: TocItem[] = [
  { id: "filer", label: "Filer og hemmeligheter" },
  { id: "verktoy", label: "Verktøy" },
  { id: "nettverk", label: "Nettverk" },
  { id: "git", label: "Git" },
];

const FAQ = "/nav-pilot/guider/cplt-feilmeldinger";
const KNOWN_IMPACTS = "https://github.com/navikt/cplt/blob/main/docs/known-impacts.md";

export default function CpltBegrensninger() {
  return (
    <DocPage
      label="Referanse"
      upgrade
      title="Kjente begrensninger i cplt"
      description="Det cplt stenger med vilje, og hva du gjør i stedet. Alt gjelder når nav-pilot starter agenten i cplt."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="filer" size="medium" level="2">
            Filer og hemmeligheter
          </LinkableHeading>
          <Bullets>
            <li>
              <code className={code}>~/.ssh</code>, <code className={code}>~/.gnupg</code>,{" "}
              <code className={code}>~/.aws</code>, <code className={code}>~/.azure</code>,{" "}
              <code className={code}>~/.kube</code> og <code className={code}>~/.config/gcloud</code> er stengt. Du kan
              åpne én fil om gangen for lesing, ikke hele mappa:
            </li>
          </Bullets>
          <CodeBlock
            compact
          >{`cplt config set allow.read ~/.config/gcloud/application_default_credentials.json`}</CodeBlock>
          <Bullets>
            <li>
              macOS: <code className={code}>.env</code>-filer og nøkkelfiler i prosjektet er stengt. På Linux er de
              åpne, se{" "}
              <NextLink href={`${FAQ}#env-filer`} className={linkClass}>
                Feil i sandkassen
              </NextLink>
              .
            </li>
            <li>
              Miljøvariabler som kan være hemmeligheter, blir fjernet. Send inn dem du trenger med{" "}
              <code className={code}>sandbox.pass_env</code>.
            </li>
            <li>
              Linux og WSL: agenten når ikke nøkkelringen, se{" "}
              <NextLink href={`${FAQ}#keyring`} className={linkClass}>
                System vault not available
              </NextLink>
              .
            </li>
            <li>
              macOS: <code className={code}>~/Desktop</code> og <code className={code}>~/Documents</code> er stengt av
              macOS, også uten cplt. Kopier filen inn i prosjektet, eller gi terminalen full disktilgang.
            </li>
          </Bullets>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="verktoy" size="medium" level="2">
            Verktøy
          </LinkableHeading>
          <Bullets>
            <li>
              <code className={code}>gcloud</code>, <code className={code}>aws</code>, <code className={code}>az</code>{" "}
              og <code className={code}>kubectl</code> med dine påloggingsdata virker ikke. Kjør dem selv, utenfor cplt.
            </li>
            <li>
              Docker og Testcontainers er stengt, fordi Docker gir tilgang til hele maskinen. Se{" "}
              <NextLink href="/nav-pilot/guider/cplt-gradle#testcontainers" className={linkClass}>
                Testcontainers
              </NextLink>{" "}
              for hvordan du åpner.
            </li>
            <li>
              <code className={code}>ps</code>, <code className={code}>top</code> og <code className={code}>sudo</code>{" "}
              virker ikke på macOS, se{" "}
              <NextLink href={`${FAQ}#setuid`} className={linkClass}>
                setuid
              </NextLink>
              .
            </li>
            <li>
              Globale installasjoner med npm og pnpm feiler. Kjør dem utenfor cplt, se{" "}
              <NextLink href="/nav-pilot/guider/cplt-node#globale" className={linkClass}>
                Globale installasjoner
              </NextLink>
              .
            </li>
          </Bullets>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="nettverk" size="medium" level="2">
            Nettverk
          </LinkableHeading>
          <Bullets>
            <li>Bare port 443 er åpen ut.</li>
            <li>Proxyen stenger hoster med privat IP-adresse.</li>
            <li>macOS: localhost er stengt. På Linux når agenten en lokal tjeneste på en åpen port.</li>
            <li>
              Proxyen ser bare trafikk som bruker den. Uten tvungen proxy kan et program koble seg direkte til port 443,
              og da gjelder ingen tillatelsesliste.
            </li>
          </Bullets>
          <BodyLong>
            Hvordan du åpner, står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-nettverk" className={linkClass}>
              Nettverk i sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="git" size="medium" level="2">
            Git
          </LinkableHeading>
          <Bullets>
            <li>
              Push til standardgrenen, force push og <code className={code}>gh pr merge</code> er stoppet.
            </li>
            <li>
              macOS: <code className={code}>git config</code>, <code className={code}>git remote set-url</code>,{" "}
              <code className={code}>git clone</code>, <code className={code}>git init</code> og git-hooks må kjøres
              utenfor cplt.
            </li>
            <li>SSH-nøkler er stengt. Bruk HTTPS.</li>
            <li>Commitene til agenten er usignerte.</li>
          </Bullets>
          <BodyLong>
            Se{" "}
            <NextLink href="/nav-pilot/guider/cplt-git" className={linkClass}>
              Git og GitHub i sandkassen
            </NextLink>
            . Hele lista, med detaljer per plattform, står i{" "}
            <a href={KNOWN_IMPACTS} className={linkClass}>
              known-impacts.md
            </a>{" "}
            i cplt.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
