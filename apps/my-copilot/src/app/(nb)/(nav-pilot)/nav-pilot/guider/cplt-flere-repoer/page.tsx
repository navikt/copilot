import { BodyLong, Box, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Flere repoer og deling i team",
  description:
    "Gi agenten tilgang til flere repoer med cplt link, og del lista med teamet i .cplt.toml. Hver kollega godkjenner én gang med cplt trust accept repos.",
};

const TOC: TocItem[] = [
  { id: "legg-til", label: "Legg til et repo til" },
  { id: "dele", label: "Del lista med teamet" },
  { id: "kollega", label: "Det kollegaen ser" },
  { id: "sikkerhet", label: "Hva godkjenningen gir" },
  { id: "begrensninger", label: "Begrensninger" },
];

export default function CpltFlereRepoer() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Flere repoer og deling i team"
      description="Jobber agenten i en tjeneste og et bibliotek samtidig, legger du til det andre repoet. Skriv det i .cplt.toml, så får kollegaene samme oppsett med én kommando."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="legg-til" size="medium" level="2">
            Legg til et repo til
          </LinkableHeading>
          <BodyLong>
            Stå i repoet du starter agenten fra, og oppgi det andre repoet med navn. cplt finner utsjekkingen på
            maskinen din og sjekker at <code className={code}>origin</code> er det repoet du ba om:
          </BodyLong>
          <CodeBlock compact>{`cplt link navikt/sykepenger-model`}</CodeBlock>
          <BodyLong>
            cplt leter ved siden av repoet du står i, og i mappestrukturen <code className={code}>vert/org/repo</code>{" "}
            hvis du bruker den. Ligger utsjekkingen et annet sted, oppgi mappa. Den blir også sjekket:
          </BodyLong>
          <CodeBlock compact>{`cplt link navikt/sykepenger-model ~/kode/sykepenger-model`}</CodeBlock>
          <BodyLong>
            Lenken gjelder denne utsjekkingen og alle senere økter. Oppstartsoversikten viser hvilke repoer som er med.
            Fjern lenken igjen med <code className={code}>--unlink</code>:
          </BodyLong>
          <CodeBlock compact>{`cplt link navikt/sykepenger-model --unlink`}</CodeBlock>
          <BodyLong>
            Trenger du repoet bare én gang, bruk <code className={code}>--repo-dir</code> i stedet. Det lagrer
            ingenting:
          </BodyLong>
          <CodeBlock compact>{`cplt --repo-dir ~/kode/sykepenger-model`}</CodeBlock>
          <BodyLong>Et lenket repo får det samme som prosjektmappa:</BodyLong>
          <Bullets>
            <li>Agenten kan lese, skrive og kjøre programmer i hele repoet.</li>
            <li>
              <code className={code}>gh pr create</code> og andre skrivekommandoer i <code className={code}>gh</code>{" "}
              virker mot repoet.
            </li>
            <li>
              Git-vakten beskytter default branch i hvert repo, også om den heter noe annet enn{" "}
              <code className={code}>main</code>.
            </li>
            <li>
              <code className={code}>[deny]</code> i repoets egen <code className={code}>.cplt.toml</code> gjelder.
            </li>
            <li>Rapporten etter økta viser endringene i hvert repo for seg.</li>
          </Bullets>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="dele" size="medium" level="2">
            Del lista med teamet
          </LinkableHeading>
          <BodyLong>
            Trenger alle i teamet de samme repoene, legg til <code className={code}>--propose</code>. Da lenker cplt
            repoet for deg og skriver det inn i <code className={code}>.cplt.toml</code>. Fila blir laget hvis den ikke
            finnes, og resten av den blir stående:
          </BodyLong>
          <CodeBlock compact>
            {`cplt link --propose navikt/sykepenger-model
git add .cplt.toml
git commit -m "chore: propose sykepenger-model for cplt sessions"
git push`}
          </CodeBlock>
          <BodyLong>Fila får en liste med navn på repoer, aldri stier:</BodyLong>
          <CodeBlock filename=".cplt.toml">{`[propose]
repos = ["navikt/sykepenger-model", "navikt/spleis-testdata"]`}</CodeBlock>
          <BodyLong>
            Lista er et forslag. Den gir ingen tilgang før hver kollega har godkjent den på sin egen maskin. Fila må
            være sjekket inn: cplt ser bort fra <code className={code}>repos</code> i en{" "}
            <code className={code}>.cplt.toml</code> som ikke er committet.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="kollega" size="medium" level="2">
            Det kollegaen ser
          </LinkableHeading>
          <BodyLong>
            Når en kollega har hentet endringen og starter agenten, sier cplt fra om forslaget før oppstartsoversikten.
            Agenten får ikke tilgang til repoene ennå:
          </BodyLong>
          <CodeBlock compact>
            {`[cplt] Untrusted .cplt.toml: 1 unapproved permission(s), not applied. Review: cplt trust
  ○ repos: navikt/sykepenger-model, navikt/spleis-testdata (each resolved and origin-verified on this machine)
  Approve: cplt trust accept --all`}
          </CodeBlock>
          <BodyLong>Se hele forslaget først. Der står det også hvilke repoer cplt fant på maskinen:</BodyLong>
          <CodeBlock compact>{`cplt trust show`}</CodeBlock>
          <BodyLong>
            Godkjenn bare repoene. <code className={code}>cplt trust accept --all</code> godkjenner også resten av
            forslaget:
          </BodyLong>
          <CodeBlock compact>{`cplt trust accept repos`}</CodeBlock>
          <BodyLong>
            cplt finner hvert repo og sjekker <code className={code}>origin</code>, akkurat som{" "}
            <code className={code}>cplt link</code>. Mangler et repo på maskinen, spør cplt om det skal klones ved siden
            av repoet du står i:
          </BodyLong>
          <CodeBlock compact>{`Clone navikt/sykepenger-model into /Users/deg/kode/sykepenger-model? [Y/n]`}</CodeBlock>
          <BodyLong>
            Svarer du ja, kloner cplt repoet, sjekker <code className={code}>origin</code> og lenker det. Uten terminal
            skriver cplt bare klonekommandoen og kjører ingenting. Til slutt står det hvor mange av de foreslåtte
            repoene som er lenket.
          </BodyLong>
          <BodyLong>
            Kloner du et repo senere, sier oppstarten fra om at det ikke er lenket. Kjør{" "}
            <code className={code}>cplt trust accept repos</code> på nytt, så blir det lagt til. Det samme gjelder når
            noen legger et nytt repo i lista.
          </BodyLong>
          <BodyLong>
            Har du allerede lenket et repo selv, lar godkjenningen det være. Angrer du, fjern det godkjenningen la til:
          </BodyLong>
          <CodeBlock compact>{`cplt trust revoke repos`}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="sikkerhet" size="medium" level="2">
            Hva godkjenningen gir
          </LinkableHeading>
          <BodyLong>
            Hvert repo du godkjenner, får samme tilgang som prosjektmappa: lese, skrive og kjøre. Fem repoer i lista er
            fem slike tilganger. Derfor er det et eget steg:
          </BodyLong>
          <Bullets>
            <li>
              Lista har navn, ikke stier. En innsjekket fil kan ikke peke cplt til en mappe fila selv velger. Det er
              maskinen din som bestemmer hvor repoene ligger.
            </li>
            <li>Et forslag som ikke er godkjent, gir ingenting.</li>
            <li>
              <code className={code}>--accept-repo-config</code> godkjenner ikke repoer. Det flagget gjelder én økt, og
              en lenke varer.
            </li>
            <li>
              Godkjenningen gjelder akkurat dette forslaget i denne utsjekkingen. Endrer noen lista, må du godkjenne på
              nytt.
            </li>
            <li>cplt kloner aldri noe uten å spørre deg.</li>
          </Bullets>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <BodyLong>
              «origin verified» betyr at <code className={code}>origin</code> stemte da du godkjente. På Linux kan
              agenten endre <code className={code}>.git/config</code> i en mappe den har hatt skrivetilgang til. Mangler
              det ekte repoet, kan en slik mappe bli lenket i stedet. Den får ikke mer tilgang enn den hadde før, men du
              kan tro at riktig repo er med når det ikke er det.
            </BodyLong>
          </Box>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="begrensninger" size="medium" level="2">
            Begrensninger
          </LinkableHeading>
          <Bullets>
            <li>
              Har du to kloner av samme repo, kan bare én av dem godkjenne <code className={code}>repos</code>, fordi
              godkjenningen lagres per <code className={code}>origin</code>. Bruk{" "}
              <code className={code}>cplt link</code> i den andre.
            </li>
            <li>
              En fork blir ikke funnet, fordi <code className={code}>origin</code> peker på forken.
            </li>
            <li>
              Et repo uten <code className={code}>origin</code>, eller med bare <code className={code}>upstream</code>,
              blir ikke funnet.
            </li>
            <li>Et monorepo kan bare legges til i sin helhet.</li>
            <li>
              Bare repoet du starter fra, kan foreslå. Står det <code className={code}>[propose]</code> i et lenket
              repo, sier cplt fra ved oppstart og bruker det ikke.
            </li>
          </Bullets>
          <BodyLong>
            Resten av <code className={code}>.cplt.toml</code> står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett" className={linkClass}>
              Sett opp cplt i et repo
            </NextLink>
            . Git og pull requests i flere repoer står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-git" className={linkClass}>
              Git og GitHub i sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
