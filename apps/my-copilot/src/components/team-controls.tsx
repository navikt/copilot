"use client";

import { createContext, useContext, useState, type ReactNode } from "react";
import { Checkbox, CheckboxGroup, HStack, Search, VStack } from "@navikt/ds-react";
import TeamMonthPicker from "./team-month-picker";

const TeamControlsContext = createContext<{ search: string; columns: string[] }>({ search: "", columns: [] });
export const useTeamControls = () => useContext(TeamControlsContext);

export default function TeamControls({ month, children }: { month: string; children: ReactNode }) {
  const [search, setSearch] = useState("");
  const [columns, setColumns] = useState<string[]>([]);
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
          <details>
            <summary className="cursor-pointer">Velg kolonner</summary>
            <CheckboxGroup legend="Bruksmønster" size="small" value={columns} onChange={setColumns}>
              <Checkbox value="models">Modeller</Checkbox>
              <Checkbox value="feature">Funksjon</Checkbox>
              <Checkbox value="language">Språk</Checkbox>
            </CheckboxGroup>
          </details>
        </HStack>
        {children}
      </VStack>
    </TeamControlsContext.Provider>
  );
}
