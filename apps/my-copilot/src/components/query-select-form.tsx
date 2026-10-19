import type { FormEventHandler, ReactNode } from "react";
import { Button, HStack, Select } from "@navikt/ds-react";

/**
 * GET form with one Select and a «Vis» button, shared by the Innsikt period and month pickers.
 * Works in server and client components; pass `onSubmit` only from a client component.
 * The Select is keyed on `value` so navigation resets an unsubmitted edit.
 */
export function QuerySelectForm({
  action,
  formLabel,
  label,
  name,
  value,
  options,
  onSubmit,
  before,
  after,
}: {
  action: string;
  formLabel: string;
  label: string;
  name: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onSubmit?: FormEventHandler<HTMLFormElement>;
  /** Hidden fields placed before the controls. */
  before?: ReactNode;
  /** Extra controls between the Select and the button. */
  after?: ReactNode;
}) {
  return (
    <form action={action} method="get" aria-label={formLabel} onSubmit={onSubmit}>
      {before}
      <HStack gap="space-8" align="end" wrap>
        <Select key={value} label={label} name={name} defaultValue={value}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </Select>
        {after}
        <Button type="submit" variant="secondary-neutral">
          Vis
        </Button>
      </HStack>
    </form>
  );
}
