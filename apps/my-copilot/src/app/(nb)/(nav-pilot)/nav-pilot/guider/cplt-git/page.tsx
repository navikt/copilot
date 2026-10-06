import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Git og GitHub i sandkassen",
  description:
    "Hva agenten kan gjøre med git og gh når nav-pilot kjører i cplt: commit, push til egen gren, pull requests, andre repoer og signerte commits.",
};

const TOC: TocItem[] = [
  { id: "oppsett", label: "Før første økt" },
  { id: "push", label: "Push og pull request" },
  { id: "vaktene", label: "Det cplt stopper" },
  { id: "andre-repoer", label: "Andre repoer" },
  { id: "utenfor", label: "Kjør dette utenfor cplt" },
  { id: "opencode", label: "Push fra OpenCode" },
  { id: "signering", label: "Signerte commits" },
];

const FAQ = "/nav-pilot/guider/cplt-feilmeldinger";

export default function CpltGit() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Git og GitHub i sandkassen"
      description="Hva agenten kan gjøre med git og gh i cplt, og hva du må gjøre selv."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="oppsett" size="medium" level="2">
            Før første økt
          </LinkableHeading>
          <BodyLong>
            SSH-nøklene dine er stengt i sandkassen, så git må bruke HTTPS med tokenet fra{" "}
            <code className={code}>gh</code>. Kjør dette i terminalen, utenfor cplt:
          </BodyLong>
          <CodeBlock compact>
            {`gh auth login
gh auth setup-git
git remote -v          # skal vise https://github.com/...`}
          </CodeBlock>
          <BodyLong>
            Viser <code className={code}>git remote -v</code> <code className={code}>git@github.com:</code>, se{" "}
            <NextLink href={`${FAQ}#publickey`} className={linkClass}>
              Permission denied (publickey)
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="push" size="medium" level="2">
            Push og pull request
          </LinkableHeading>
          <BodyLong>
            Agenten kan committe, lage grener, hente, rebase og pushe egne grener. Be den pushe med grennavnet og oppgi
            grenen når den lager pull requesten:
          </BodyLong>
          <CodeBlock compact>
            {`git push origin HEAD:min-gren
gh pr create --head min-gren`}
          </CodeBlock>
          <BodyLong>
            Uten terminal pusher ikke <code className={code}>gh pr create</code> for deg. På macOS fjerner cplt{" "}
            <code className={code}>-u</code> fra <code className={code}>git push -u</code>, så grenen får ingen
            upstream.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="vaktene" size="medium" level="2">
            Det cplt stopper
          </LinkableHeading>
          <BodyLong>To vakter står på som standard:</BodyLong>
          <Bullets>
            <li>
              Git-vakta stopper push til standardgrenen og force push. <code className={code}>main</code> og{" "}
              <code className={code}>master</code> er alltid beskyttet. Med sikkerhetsnivået{" "}
              <code className={code}>strict</code> stopper den all push.
            </li>
            <li>
              gh-vakta lar agenten jobbe mot repoet den startet i, og stopper <code className={code}>gh pr merge</code>.
            </li>
          </Bullets>
          <BodyLong>
            Du merger selv. Hvilke nivåer som finnes, står i{" "}
            <NextLink href="/nav-pilot/forklaring/sandkassen#sikkerhetsniva" className={linkClass}>
              Sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="andre-repoer" size="medium" level="2">
            Andre repoer
          </LinkableHeading>
          <BodyLong>
            Skal agenten jobbe i et repo til, legg det til. cplt finner utsjekkingen på maskinen og sjekker at{" "}
            <code className={code}>origin</code> er det repoet du ba om:
          </BodyLong>
          <CodeBlock compact>{`cplt link navikt/annet-repo`}</CodeBlock>
          <BodyLong>Repoet får samme tilgang som prosjektmappa: lese, skrive og kjøre.</BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="utenfor" size="medium" level="2">
            Kjør dette utenfor cplt
          </LinkableHeading>
          <BodyLong>
            På macOS er <code className={code}>.git/config</code> og <code className={code}>.git/hooks</code>{" "}
            skrivebeskyttet, fordi de kan få git til å kjøre kode utenfor sandkassen. Disse må du kjøre selv, i en
            vanlig terminal:
          </BodyLong>
          <Bullets>
            <li>
              <code className={code}>git config</code> og <code className={code}>git config --global</code>
            </li>
            <li>
              <code className={code}>git remote add</code> og <code className={code}>git remote set-url</code>
            </li>
            <li>
              <code className={code}>git clone</code>, <code className={code}>git init</code> og{" "}
              <code className={code}>git submodule add</code>
            </li>
            <li>å lage eller endre git-hooks</li>
          </Bullets>
          <BodyLong>
            Feilmeldingene står under{" "}
            <NextLink href={`${FAQ}#git`} className={linkClass}>
              Git og GitHub i Feil i sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="opencode" size="medium" level="2">
            Push fra OpenCode
          </LinkableHeading>
          <BodyLong>
            Gjelder macOS. <code className={code}>gh auth login</code> legger tokenet i nøkkelringen. Copilot når den,
            OpenCode gjør ikke, så <code className={code}>git push</code> fra OpenCode får ikke noe token. cplt fjerner
            også <code className={code}>GH_TOKEN</code> fra miljøet til OpenCode. Send det inn:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set sandbox.pass_env GH_TOKEN
export GH_TOKEN=$(gh auth token)   # i skallet du starter nav-pilot fra`}
          </CodeBlock>
          <BodyLong>
            Da ligger tokenet ditt i miljøet til agenten, og den kan bruke det mot GitHub uten å gå via gh-vakta.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="signering" size="medium" level="2">
            Signerte commits
          </LinkableHeading>
          <BodyLong>
            <code className={code}>~/.gnupg</code> og <code className={code}>~/.ssh</code> er stengt, så cplt slår av
            signering i sandkassen. Commitene til agenten blir usignerte. Krever grenbeskyttelsen signerte commits, kan
            du slippe agenten til GPG-agenten din:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_gpg_signing true --force`}</CodeBlock>
          <BodyLong>
            Den private nøkkelen er fortsatt stengt, men agenten kan be GPG-agenten signere hva som helst mens økten
            varer. Signering med SSH-nøkkel (<code className={code}>gpg.format=ssh</code>) virker ikke.
          </BodyLong>
          <BodyLong>
            Start GPG-agenten før økten. Sandkassen stenger skriving til <code className={code}>~/.gnupg</code>, så gpg
            får trolig ikke startet agenten selv. Kjør dette utenfor cplt:
          </BodyLong>
          <CodeBlock compact>{`gpgconf --launch gpg-agent`}</CodeBlock>
          <BodyLong>
            GPG-signering stilles inn i cplt, ikke i nav-pilot. nav-pilot sender ikke flagg som{" "}
            <code className={code}>--allow-gpg-signing</code> videre til cplt. Sett dem i cplt-konfigurasjonen som vist
            over. Git over SSH kan ikke slås på: <code className={code}>~/.ssh</code> er alltid stengt, så bruk HTTPS.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
