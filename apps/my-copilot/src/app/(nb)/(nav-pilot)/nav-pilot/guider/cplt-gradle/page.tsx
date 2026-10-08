import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Kotlin og Gradle i sandkassen",
  description:
    "Slik bygger og tester du Kotlin-prosjekter med Gradle når nav-pilot og Copilot kjører i cplt: daemon, MockK, GitHub Packages, interne Nav-verter, JDK-er og Testcontainers.",
};

const TOC: TocItem[] = [
  { id: "oppsett", label: "Oppsett som dekker de fleste" },
  { id: "tillatelsesliste", label: "Wrapperen og en liste over tillatte verter" },
  { id: "github-packages", label: "Pakker fra GitHub Packages" },
  { id: "interne-verter", label: "Interne Nav-verter" },
  { id: "jdk", label: "JDK-er som Gradle laster ned" },
  { id: "testcontainers", label: "Testcontainers og Docker" },
  { id: "feil", label: "Når bygget feiler" },
];

const FAQ = "/nav-pilot/guider/cplt-feilmeldinger";
const KNOWN_IMPACTS = "https://github.com/navikt/cplt/blob/main/docs/known-impacts.md";
const PENSJONSBREV = "https://github.com/navikt/pensjonsbrev/blob/main/.cplt.toml";

export default function CpltGradle() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Kotlin og Gradle i sandkassen"
      description="Det du trenger for at Gradle-bygg og tester skal virke når agenten kjører i cplt."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="oppsett" size="medium" level="2">
            Oppsett som dekker de fleste
          </LinkableHeading>
          <BodyLong>
            Et vanlig Kotlin-prosjekt trenger to innstillinger. Gradle starter en daemon som snakker med bygget over en
            tilfeldig port på localhost, og cplt stenger localhost (
            <NextLink href={`${FAQ}#gradle-connect`} className={linkClass}>
              også med --no-daemon
            </NextLink>
            ). MockK, Mockito og ByteBuddy kobler seg til JVM-en mens testene kjører, og den socketen stenger cplt på
            macOS.
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set sandbox.allow_localhost_any true
cplt config set sandbox.allow_jvm_attach true`}
          </CodeBlock>
          <BodyLong>
            Med <code className={code}>allow_localhost_any</code> når agenten alle tjenester som lytter på localhost,
            også en lokal database, og på Linux åpner den mer, se{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett#localhost" className={linkClass}>
              Tjenester på localhost
            </NextLink>
            . <code className={code}>allow_jvm_attach</code> åpner bare socketene{" "}
            <code className={code}>/tmp/.java_pid&lt;PID&gt;</code>, ingen andre.
          </BodyLong>
          <BodyLong>Gjelder det ett prosjekt, legg innstillingene i repoet, så får hele teamet dem:</BodyLong>
          <CodeBlock compact>
            {`cplt config set --repo sandbox.allow_localhost_any true
cplt config set --repo sandbox.allow_jvm_attach true
git add .cplt.toml && git commit -m "chore: cplt-oppsett for Gradle"
cplt trust accept --all`}
          </CodeBlock>
          <BodyLong>
            Hvordan <code className={code}>.cplt.toml</code> og godkjenningen virker, står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett#forste-gang" className={linkClass}>
              Sett opp cplt i et repo
            </NextLink>
            . <code className={code}>cplt init</code> foreslår både <code className={code}>allow_jvm_attach</code> og{" "}
            <code className={code}>allow_localhost_any</code> for et Gradle-bygg. På Linux trenger du ikke{" "}
            <code className={code}>allow_jvm_attach</code>, for der stenger ikke cplt socketen.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="tillatelsesliste" size="medium" level="2">
            Wrapperen og en liste over tillatte verter
          </LinkableHeading>
          <BodyLong>
            I standardoppsettet stopper cplt bare kjente skadelige verter og verter med private adresser. Da laster{" "}
            <code className={code}>./gradlew</code> ned Gradle og avhengigheter som vanlig.
          </BodyLong>
          <BodyLong>
            Har du slått på en liste over tillatte verter med <code className={code}>proxy.default_allowlist</code>, er
            Maven Central og Gradle-plugins med, se{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett#tillatelsesliste" className={linkClass}>
              Pakkeregistre og en liste over tillatte verter
            </NextLink>
            .
          </BodyLong>
          <BodyLong>
            Selve Gradle-distribusjonen som wrapperen laster ned, er ikke med. Den kommer fra{" "}
            <code className={code}>services.gradle.org</code>, som sender videre til{" "}
            <code className={code}>github.com</code> og{" "}
            <code className={code}>release-assets.githubusercontent.com</code>. Uten disse stopper{" "}
            <code className={code}>./gradlew</code> før bygget starter, og proxyloggen viser{" "}
            <code className={code}>BLOCKED-ALLOWLIST</code>. Legg til vertene:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set allow.domains services.gradle.org
cplt config set allow.domains github.com
cplt config set allow.domains release-assets.githubusercontent.com`}
          </CodeBlock>
          <BodyLong>
            Copilot har <code className={code}>github.com</code> på lista fra før, så med Copilot kan du hoppe over den
            linja.
          </BodyLong>
          <BodyLong>Sjekk en vert uten å kjøre bygget:</BodyLong>
          <CodeBlock compact>{`cplt check net services.gradle.org`}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="github-packages" size="medium" level="2">
            Pakker fra GitHub Packages
          </LinkableHeading>
          <BodyLong>
            Mange navikt-biblioteker ligger i GitHub Packages. Speilet er den enkleste måten å hente dem på i
            sandkassen.
          </BodyLong>

          <LinkableHeading id="speilet" size="small" level="3">
            Navs speil, uten token
          </LinkableHeading>
          <BodyLong>
            <code className={code}>github-package-registry-mirror.gc.nav.no</code> er et åpent speil av de offentlige
            pakkene i GitHub Packages. Det krever ingen token, så agenten trenger ikke tilgang til noen hemmeligheter:
          </BodyLong>
          <CodeBlock compact>
            {`repositories {
    mavenCentral()
    maven("https://github-package-registry-mirror.gc.nav.no/cached/maven-release")
}`}
          </CodeBlock>
          <BodyLong>
            Speilet står ikke på cplts liste over tillatte verter. Bruker du en slik liste, legg det til:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.domains github-package-registry-mirror.gc.nav.no`}</CodeBlock>

          <LinkableHeading id="gradle-properties" size="small" level="3">
            Token i ~/.gradle/gradle.properties
          </LinkableHeading>
          <BodyLong>
            <code className={code}>maven.pkg.github.com</code> krever token. På macOS stenger cplt{" "}
            <code className={code}>~/.gradle/gradle.properties</code> fordi fila ofte har tokens i seg, og finnes fila,
            stopper hvert Gradle-bygg i sandkassen med{" "}
            <NextLink href={`${FAQ}#gradle-properties`} className={linkClass}>
              Error when loading properties file
            </NextLink>
            . Det tryggeste er å flytte det du trenger over til speilet og ta tokenet ut av fila. Ellers gir du agenten
            lesetilgang til fila, og dermed alle tokenene i den:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.read ~/.gradle/gradle.properties`}</CodeBlock>
          <BodyLong>
            Hva det koster å åpne tokenfilene, og hvordan du åpner alle tre på én gang, står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett#github-packages" className={linkClass}>
              Pakker fra GitHub Packages
            </NextLink>
            .
          </BodyLong>
          <BodyLong>
            Leser bygget tokenet fra <code className={code}>GITHUB_TOKEN</code>, kommer det an på agenten. Copilot får{" "}
            <code className={code}>GITHUB_TOKEN</code>, <code className={code}>GH_TOKEN</code> og{" "}
            <code className={code}>COPILOT_GITHUB_TOKEN</code> slik de står i skallet ditt, fordi Copilot trenger et
            GitHub-token. OpenCode og de andre agentene får dem ikke, og det gjør heller ikke{" "}
            <code className={code}>cplt exec</code>. Der sender <code className={code}>--pass-env GITHUB_TOKEN</code>{" "}
            den med, men da har agenten tokenet ditt.
          </BodyLong>
          <BodyLong>
            <code className={code}>Received status code 401 from server: Unauthorized</code> betyr at forespørselen kom
            fram, men at tokenet mangler.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="interne-verter" size="medium" level="2">
            Interne Nav-verter
          </LinkableHeading>
          <BodyLong>
            cplt stopper forbindelser til verter som peker til en privat IP-adresse. Det gjelder også Gradle, fordi cplt
            sender JVM-ens nettverkstrafikk gjennom proxyen sin. Når en slik vert er et Maven-repo, stopper bygget med{" "}
            <code className={code}>Private target blocked by cplt</code>. Det kan gjelde{" "}
            <code className={code}>repo.adeo.no</code> på Nav-nettet og verter under{" "}
            <code className={code}>intern.nav.no</code>. Åpne vertene med navn:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set proxy.allow_private_domains repo.adeo.no
cplt config set proxy.allow_private_domains intern.nav.no`}
          </CodeBlock>
          <BodyLong>
            Ett navn dekker alle undervertene. For ett prosjekt kan du bruke{" "}
            <code className={code}>cplt config set --repo proxy.allow_private_domains …</code> og godkjenne med{" "}
            <code className={code}>cplt trust accept</code>, slik{" "}
            <a href={PENSJONSBREV} className={linkClass}>
              pensjonsbrev
            </a>{" "}
            gjør. For én økt holder flagget <code className={code}>--allow-private-domain repo.adeo.no</code>.
          </BodyLong>
          <BodyLong>
            Står repoet oppført med en IP-adresse i stedet for et navn, for eksempel{" "}
            <code className={code}>https://10.20.30.40/repository/</code>, kan ingen innstilling åpne det. Bruk
            vertsnavnet i byggefila.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="jdk" size="medium" level="2">
            JDK-er som Gradle laster ned
          </LinkableHeading>
          <BodyLong>
            Med <code className={code}>jvmToolchain(…)</code> og foojay-pluginen laster Gradle ned JDK-en prosjektet ber
            om, til <code className={code}>~/.gradle/jdks</code>. På macOS kan agenten kjøre JDK-ene som ligger der, men
            ikke skrive nye dit, så en JDK som mangler, kan ikke lastes ned inne i cplt. På Linux er mappa skrivbar.
            Under en liste over tillatte verter må <code className={code}>api.foojay.io</code> og vertene JDK-en hentes
            fra, stå på lista. For Temurin er det de samme som for wrapperen.
          </BodyLong>
          <BodyLong>
            Kjør bygget én gang utenfor cplt, så ligger JDK-en klar. Etterpå kan du se hvilke JDK-er Gradle finner:
          </BodyLong>
          <CodeBlock compact>{`./gradlew javaToolchains`}</CodeBlock>
          <BodyLong>
            Vil du at bygget skal feile med en gang i stedet for å prøve å laste ned, sett dette i prosjektets{" "}
            <code className={code}>gradle.properties</code>:
          </BodyLong>
          <CodeBlock compact>{`org.gradle.java.installations.auto-download=false`}</CodeBlock>
          <BodyLong>
            Feilmeldingen fra foojay ser ut som et nettverksproblem. Se{" "}
            <NextLink href={`${FAQ}#foojay`} className={linkClass}>
              Unable to download toolchain
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="testcontainers" size="medium" level="2">
            Testcontainers og Docker
          </LinkableHeading>
          <BodyLong>
            cplt stenger Docker, fordi tilgang til Docker i praksis er root på maskinen (se{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett#docker" className={linkClass}>
              Docker
            </NextLink>{" "}
            i oppsettguiden). Testcontainers feiler derfor i cplt uten videre. Du har tre muligheter, fra tryggest til
            minst trygg:
          </BodyLong>
          <Bullets>
            <li>
              <strong>Kjør testene som trenger containere, utenfor cplt.</strong> La agenten kjøre resten.
            </li>
            <li>
              <strong>Start containerne selv, utenfor cplt, og åpne portene.</strong> Det passer for en database eller
              Kafka fra <code className={code}>docker compose</code>, men ikke for Testcontainers, som starter
              containerne selv.
            </li>
            <li>
              <strong>Gi agenten Docker.</strong> Da virker Testcontainers, også med colima. Agenten kan da gjøre alt
              Docker kan, og lese <code className={code}>~/.docker</code> med registerinnloggingene dine. Bruk det bare
              i et repo du stoler på, og helst bare for én økt.
            </li>
          </Bullets>
          <BodyLong>En database på port 5432, startet utenfor cplt:</BodyLong>
          <CodeBlock compact>{`cplt config set allow.localhost 5432`}</CodeBlock>
          <BodyLong>
            Slik gir du agenten Docker for én økt eller fast. Testcontainers kobler seg til containerne over tilfeldige
            porter på localhost, så slå på <code className={code}>allow_localhost_any</code> også:
          </BodyLong>
          <CodeBlock compact>
            {`cplt --allow-docker
cplt config set sandbox.allow_docker true --force`}
          </CodeBlock>
          <BodyLong>
            Med colima må <code className={code}>TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock</code> være
            satt, som utenfor cplt. cplt sender <code className={code}>DOCKER_HOST</code> og{" "}
            <code className={code}>TESTCONTAINERS_*</code> videre til agenten.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="feil" size="medium" level="2">
            Når bygget feiler
          </LinkableHeading>
          <BodyLong>Slå på loggen over blokkerte forbindelser, og kjør bygget i sandkassen selv:</BodyLong>
          <CodeBlock compact>
            {`cplt config set proxy.log_level blocked
cplt exec -- ./gradlew build`}
          </CodeBlock>
          <BodyLong>
            Linjer med <code className={code}>[proxy]</code> og <code className={code}>BLOCKED</code> viser hvilken vert
            som ble stoppet og hvorfor. Blokkeringer av filer og localhost står ikke der, se{" "}
            <NextLink href="/nav-pilot/guider/feilsoking#kjernen" className={linkClass}>
              Filer, programmer og localhost
            </NextLink>
            . Feilmeldingene fra Gradle og JVM står i{" "}
            <NextLink href={`${FAQ}#jvm`} className={linkClass}>
              Feil i sandkassen
            </NextLink>
            . Mer om hvorfor står i{" "}
            <a href={KNOWN_IMPACTS} className={linkClass}>
              known-impacts.md
            </a>{" "}
            (engelsk).
          </BodyLong>
          <BodyLong>
            Kommandoene på denne siden er testet med cplt fra 29. september 2026 på macOS, med et lite Kotlin-prosjekt,
            Gradle 9.7.0 og 9.8.0 og colima.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
