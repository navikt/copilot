"use client";

import { createContext, useContext, useState, type ReactNode } from "react";
import { Button, Checkbox, CheckboxGroup, HStack, Popover, Search, VStack } from "@navikt/ds-react";
import { CodeIcon, CpuIcon, WrenchIcon } from "@navikt/aksel-icons";
import TeamMonthPicker from "./team-month-picker";

const TeamControlsContext = createContext<{ search: string; columns: string[] }>({ search: "", columns: [] });
export const useTeamControls = () => useContext(TeamControlsContext);

export default function TeamControls({ month, children }: { month: string; children: ReactNode }) {
  const [search, setSearch] = useState("");
  const [columns, setColumns] = useState<string[]>([]);
  const [open, setOpen] = useState(false);
  const [anchor, setAnchor] = useState<HTMLButtonElement | null>(null);
  return (
    <TeamControlsContext.Provider value={{ search, columns }}>
      <VStack gap="space-24">
        <HStack gap="space-24" align="end" wrap>
          <TeamMonthPicker month={month} />
          <Search
            label="Søk etter team"
            hideLabel={false}
            value={search}
            onChange={setSearch}
            size="small"
            variant="simple"
            className="max-w-xs"
          />
          <Button
            ref={setAnchor}
            type="button"
            size="small"
            variant="secondary-neutral"
            aria-expanded={open}
            aria-controls="team-column-picker"
            onClick={() => setOpen(!open)}
          >
            Velg kolonner
          </Button>
          <Popover
            id="team-column-picker"
            open={open}
            onClose={() => setOpen(false)}
            anchorEl={anchor}
            placement="bottom-end"
          >
            <Popover.Content
              onKeyDownCapture={(event) => {
                if (event.key === "Escape") anchor?.focus();
              }}
            >
              <CheckboxGroup legend="Bruksmønster" size="small" value={columns} onChange={setColumns}>
                <Checkbox value="models">
                  <HStack gap="space-8" align="center">
                    <CpuIcon aria-hidden fontSize="1.25rem" />
                    Modeller
                  </HStack>
                </Checkbox>
                <Checkbox value="feature">
                  <HStack gap="space-8" align="center">
                    <WrenchIcon aria-hidden fontSize="1.25rem" />
                    Funksjon
                  </HStack>
                </Checkbox>
                <Checkbox value="language">
                  <HStack gap="space-8" align="center">
                    <CodeIcon aria-hidden fontSize="1.25rem" />
                    Språk
                  </HStack>
                </Checkbox>
              </CheckboxGroup>
            </Popover.Content>
          </Popover>
        </HStack>
        {children}
      </VStack>
    </TeamControlsContext.Provider>
  );
}
