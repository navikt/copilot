import { Heading, BodyShort, Alert } from "@navikt/ds-react";

export default function GettingStarted() {
  return (
    <div className="space-y-8">
      <section>
        <Heading size="small" level="3" className="mb-3 text-blue-700">
          Hvordan få tilgang?
        </Heading>
        <BodyShort className="text-gray-700 mb-4">
          For å bruke GitHub Copilot i Nav må du ha en tildelt lisens. Har du ikke dette ennå, kan du be om tilgang via
          Porten eller ved å spørre i Slack-kanalen <strong>#copilot-hjelp</strong>. Når du har fått tildelt lisensen,
          er den knyttet til din GitHub-brukerkonto (den som er medlem av navikt-organisasjonen).
        </BodyShort>
      </section>

      <section>
        <Heading size="small" level="3" className="mb-3">
          Installasjon og innlogging
        </Heading>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div className="bg-surface-default border border-border-subtle p-6 rounded-lg">
            <Heading size="xsmall" level="4" className="mb-2">
              Visual Studio Code
            </Heading>
            <ol className="list-decimal pl-5 space-y-2 text-gray-700 text-sm">
              <li>
                Søk etter <strong>GitHub Copilot</strong> under «Extensions»-fanen.
              </li>
              <li>Installer den offisielle utvidelsen fra GitHub.</li>
              <li>Du vil bli bedt om å logge inn med GitHub-kontoen din nederst til høyre.</li>
              <li>Godkjenn tilgangen i nettleseren.</li>
            </ol>
          </div>
          <div className="bg-surface-default border border-border-subtle p-6 rounded-lg">
            <Heading size="xsmall" level="4" className="mb-2">
              IntelliJ / JetBrains
            </Heading>
            <ol className="list-decimal pl-5 space-y-2 text-gray-700 text-sm">
              <li>
                Gå til <strong>Settings &gt; Plugins</strong>.
              </li>
              <li>
                Søk etter <strong>GitHub Copilot</strong> og installer.
              </li>
              <li>Start IDE-en på nytt.</li>
              <li>
                Klikk på Copilot-ikonet nederst til høyre og velg <strong>Login to GitHub</strong>.
              </li>
            </ol>
          </div>
        </div>
      </section>

      <Alert variant="info" className="mt-4">
        <Heading size="small" level="3">
          Feilsøking: "Du har ikke tilgang"
        </Heading>
        <BodyShort>
          Hvis editoren din sier at du ikke har tilgang selv om du vet at lisensen er tildelt, prøv å{" "}
          <strong>logge ut og inn igjen</strong> av GitHub i editoren din. Ofte henger gamle sesjons-tokens igjen.
        </BodyShort>
      </Alert>
    </div>
  );
}
