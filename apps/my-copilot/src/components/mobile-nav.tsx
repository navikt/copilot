"use client";

import { InternalHeaderButton } from "@navikt/ds-react/InternalHeader";
import { Dropdown } from "@navikt/ds-react";
import { MenuHamburgerIcon } from "@navikt/aksel-icons";
import { useRouter } from "next/navigation";

export function MobileNav() {
  const router = useRouter();

  return (
    <Dropdown>
      <InternalHeaderButton as={Dropdown.Toggle} className="flex items-center h-full">
        <MenuHamburgerIcon title="Meny" fontSize="1.5rem" />
      </InternalHeaderButton>
      <Dropdown.Menu>
        <Dropdown.Menu.List>
          <Dropdown.Menu.List.Item
            onClick={() => {
              router.push("/");
            }}
          >
            Min Copilot
          </Dropdown.Menu.List.Item>
          <Dropdown.Menu.List.Item
            onClick={() => {
              router.push("/best-practices");
            }}
          >
            Beste Praksis
          </Dropdown.Menu.List.Item>
          <Dropdown.Menu.List.Item
            onClick={() => {
              router.push("/customizations");
            }}
          >
            Verktøy
          </Dropdown.Menu.List.Item>
          <Dropdown.Menu.List.Item
            onClick={() => {
              router.push("/usage");
            }}
          >
            Statistikk
          </Dropdown.Menu.List.Item>
          <Dropdown.Menu.List.Item
            onClick={() => {
              router.push("/overview");
            }}
          >
            Kostnad
          </Dropdown.Menu.List.Item>
        </Dropdown.Menu.List>
      </Dropdown.Menu>
    </Dropdown>
  );
}
