import { Heading, BodyShort } from "@navikt/ds-react";

export default function UnderstandingCode() {
  return (
    <div className="space-y-8">
      <section>
        <Heading size="small" level="3" className="mb-3 text-blue-700">
          Lesing og kodeforståelse
        </Heading>
        <BodyShort className="text-gray-700 mb-4">
          Mer enn 50 % av tiden til en utvikler går med til å <em>lese</em> andres kode. Copilot er like kraftig til å
          forklare kompleks eller ukommentert legacy-kode som den er til å skrive ny kode.
        </BodyShort>
      </section>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <section className="bg-surface-default border border-border-subtle p-6 rounded-lg">
          <Heading size="xsmall" level="4" className="mb-2">
            Forklar denne koden
          </Heading>
          <BodyShort size="small" className="text-gray-700 mb-3">
            Marker en uforståelig funksjon eller en hel fil, og bruk Copilot Chat til å be om en overordnet forklaring.
          </BodyShort>
          <div className="bg-gray-50 p-4 rounded-md border border-gray-200">
            <p className="text-sm font-mono text-gray-800">
              "Forklar hva denne funksjonen gjør, steg for steg. Er det noen skjulte sideeffekter jeg bør være obs på?"
            </p>
          </div>
        </section>

        <section className="bg-surface-default border border-border-subtle p-6 rounded-lg">
          <Heading size="xsmall" level="4" className="mb-2">
            Hjelp med Stack Traces
          </Heading>
          <BodyShort size="small" className="text-gray-700 mb-3">
            I stedet for å google obskure feilmeldinger, kan du kopiere hele stack-tracen inn i Copilot Chat sammen med
            filen der feilen oppsto.
          </BodyShort>
          <div className="bg-gray-50 p-4 rounded-md border border-gray-200">
            <p className="text-sm font-mono text-gray-800">
              "Jeg får denne feilen: [lim inn stack trace]. Hvilken del av koden min forårsaker dette, og hvordan fikser
              jeg det?"
            </p>
          </div>
        </section>

        <section className="bg-surface-default border border-border-subtle p-6 rounded-lg">
          <Heading size="xsmall" level="4" className="mb-2">
            Dokumentering av legacy-kode
          </Heading>
          <BodyShort size="small" className="text-gray-700 mb-3">
            Har du arvet et prosjekt uten kommentarer? Be Copilot om å generere Javadoc, TSDoc eller GoDoc for deg.
          </BodyShort>
          <div className="bg-gray-50 p-4 rounded-md border border-gray-200">
            <p className="text-sm font-mono text-gray-800">
              "Skriv en utfyllende Javadoc for denne klassen som forklarer forretningslogikken, ikke bare
              linje-for-linje hva koden gjør."
            </p>
          </div>
        </section>

        <section className="bg-surface-default border border-border-subtle p-6 rounded-lg">
          <Heading size="xsmall" level="4" className="mb-2">
            Finn sikkerhetshull
          </Heading>
          <BodyShort size="small" className="text-gray-700 mb-3">
            Be Copilot om å gå gjennom koden med et kritisk sikkerhetsblikk før du sender koden til review.
          </BodyShort>
          <div className="bg-gray-50 p-4 rounded-md border border-gray-200">
            <p className="text-sm font-mono text-gray-800">
              "Gjennomgå denne filen. Er det noen åpenbare sikkerhetshull, som SQL-injection eller manglende
              validering?"
            </p>
          </div>
        </section>
      </div>
    </div>
  );
}
