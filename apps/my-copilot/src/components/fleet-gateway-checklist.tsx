"use client";

import { Checkbox, CheckboxGroup } from "@navikt/ds-react";

// Client-side only: nothing is stored, the boxes reset on reload.
export function FleetGatewayChecklist({ items }: { items: string[] }) {
  return (
    <CheckboxGroup legend="Sjekkliste før pilot" description="Kryssene lagres ikke.">
      {items.map((item) => (
        <Checkbox key={item} value={item}>
          {item}
        </Checkbox>
      ))}
    </CheckboxGroup>
  );
}
