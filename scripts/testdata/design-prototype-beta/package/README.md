# Nav Design beta

Aksel and Figma prototyping in an existing React/Aksel project. Includes one
agent, the Aksel skill, and connections to Aksel MCP and standard Figma MCP.
No nav-pilot CLI or files committed to your project are required.

## Install

After the marketplace change is merged:

1. In Copilot app or VS Code Copilot, open **Customize / Customizations** and add
   the marketplace `navikt/copilot` (or `https://github.com/navikt/copilot.git`).
2. Find **nav-design-beta** and select **Install**.
3. Review the MCP connections and authenticate with Figma through the client UI.
4. Open your project, select **nav-design-beta**, and provide a Figma design link
   plus the change you want to prototype.

Example: “Build the form in this Figma frame using the Aksel version in this
project. Check keyboard navigation and show me how to preview it.”

If the marketplace or tools are unavailable, check your client version and
organization policy. Do not copy package files into your project as a workaround.

## Permissions

Enabling the package can start connections to `https://aksel-mcp.nav.no/mcp` and
`https://mcp.figma.com/mcp`. You need access to the Figma design and may encounter
plan/seat limits. Authentication stays in the client; never paste credentials
into chat. Use non-sensitive test designs and synthetic data.

The agent can read/edit project files, fetch documentation, use named Aksel and
Figma read tools, and run prototype installs, previews and tests with normal
client approvals. Do not grant allow-all permissions. The agent's tool list is
not a client-wide restriction, and this package does not provide the nav-pilot
launcher or cplt sandbox.

## Update or remove

Use the client's installed-plugin controls to update, disable or uninstall the
package. Automatic updates depend on the client and policy. MCP access and
authentication are managed separately in the client.

This is a beta: app/VS Code compatibility, tool discovery, authentication and
update behavior have not yet been verified end to end. Report problems in
`navikt/copilot` with your client version, package version and non-sensitive
reproduction steps.
