"use client";

import { Process } from "@navikt/ds-react";
import type { ReactNode } from "react";

// Process.Event is a compound component, so it must render on the client.
export function FleetGatewaySteps({ steps }: { steps: { title: string; content: ReactNode }[] }) {
  return (
    <Process hideStatusText>
      {steps.map((step, i) => (
        <Process.Event key={step.title} title={step.title} bullet={i + 1}>
          {step.content}
        </Process.Event>
      ))}
    </Process>
  );
}
