import { sourceHeadings } from "./page-headings";

describe("sourceHeadings", () => {
  it("leser id og tekst slik lenkesjekken og søket trenger dem", () => {
    const src = `
      <LinkableHeading id="start" size="medium" level="2">1. Start serveren</LinkableHeading>
      <LinkableHeading size="small">
        Din plassering
        i Nav
      </LinkableHeading>
      <LinkableHeading size="small">Bruk <code>setup</code></LinkableHeading>
      <Heading level="3" id="flere">Flere{" "}ord</Heading>
      <h2>{title}</h2>`;
    expect(sourceHeadings(src)).toEqual([
      { tag: "LinkableHeading", id: "start", text: "1. Start serveren" },
      { tag: "LinkableHeading", id: "din-plassering-i-nav", text: "Din plassering i Nav" },
      // The component renders no id when the children are not one string.
      { tag: "LinkableHeading", id: undefined, text: "Bruk setup" },
      { tag: "Heading", id: "flere", text: "Flere ord" },
      { tag: "h2", id: undefined, text: undefined },
    ]);
  });
});
