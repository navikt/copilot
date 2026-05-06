/**
 * SVG diagram showing the trust boundaries in Nav's Copilot architecture.
 * Used on the ordliste page to help security teams understand the data flow.
 */
export function TrustBoundaryDiagram() {
  return (
    <div className="w-full overflow-x-auto">
      <svg
        viewBox="0 0 960 580"
        className="w-full"
        style={{ width: "100%", margin: "0 auto", display: "block" }}
        role="img"
        aria-label="Arkitekturdiagram som viser tillitsgrenser mellom utviklermaskin, GitHub Copilot API og model providers"
      >
        {/* Background */}
        <rect width="960" height="580" rx="12" fill="white" stroke="#e2e8f0" strokeWidth="1" />

        {/* ── Trust boundary 1: Developer machine ── */}
        <rect
          x="20"
          y="20"
          width="380"
          height="540"
          rx="10"
          fill="#f8fafc"
          stroke="#64748b"
          strokeWidth="1.5"
          strokeDasharray="6 3"
        />
        <text x="40" y="42" fill="#334155" fontSize="10" fontWeight="600">
          Utviklermaskin
        </text>

        {/* ══════ Agent sandbox ══════ */}

        {/* cplt sandbox */}
        <rect
          x="40"
          y="55"
          width="230"
          height="155"
          rx="8"
          fill="#f0fdf4"
          stroke="#10b981"
          strokeWidth="1.5"
          strokeDasharray="4 2"
        />
        <text x="55" y="73" fill="#166534" fontSize="9" fontWeight="600">
          cplt sandbox
        </text>

        {/* Agent harness */}
        <rect x="52" y="82" width="206" height="118" rx="8" fill="#0c1a14" />
        <text x="155" y="105" textAnchor="middle" fill="#4ade80" fontSize="12" fontWeight="600" fontFamily="monospace">
          Agent harness
        </text>
        <text x="155" y="121" textAnchor="middle" fill="#94a3b8" fontSize="9">
          Copilot CLI eller OpenCode
        </text>

        {/* Skills */}
        <rect x="62" y="133" width="92" height="30" rx="5" fill="#1e293b" />
        <text x="108" y="149" textAnchor="middle" fill="#a78bfa" fontSize="8.5" fontWeight="600">
          Skills
        </text>
        <text x="108" y="160" textAnchor="middle" fill="#94a3b8" fontSize="7">
          (kun instruksjoner)
        </text>

        {/* Tool calling */}
        <rect x="160" y="133" width="88" height="30" rx="5" fill="#1e293b" />
        <text x="204" y="149" textAnchor="middle" fill="#60a5fa" fontSize="8.5" fontWeight="600">
          Tool calling
        </text>
        <text x="204" y="160" textAnchor="middle" fill="#94a3b8" fontSize="7">
          filer, terminal, MCP
        </text>

        {/* ── Inference arrow: Agent → GitHub API ── */}
        <line x1="258" y1="100" x2="480" y2="100" stroke="#3b82f6" strokeWidth="2" markerEnd="url(#arrowBlue2)" />
        <text x="370" y="92" textAnchor="middle" fill="#3b82f6" fontSize="9" fontWeight="600">
          Inference context (TLS)
        </text>

        {/* Response arrow (points left into agent) */}
        <line x1="480" y1="120" x2="260" y2="120" stroke="#3b82f6" strokeWidth="1.2" strokeDasharray="3 2" />
        <polygon points="260,115 260,125 248,120" fill="#3b82f6" />
        <text x="370" y="135" textAnchor="middle" fill="#3b82f6" fontSize="7.5">
          Svar + MCP allowlist config
        </text>

        {/* ══════ Files section — inside machine ══════ */}

        {/* Project files */}
        <rect x="40" y="228" width="165" height="48" rx="8" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="122" y="246" textAnchor="middle" fill="#334155" fontSize="9" fontWeight="600">
          Prosjektfiler (kode)
        </text>
        <text x="122" y="261" textAnchor="middle" fill="#64748b" fontSize="7.5">
          Inference context · kastes etter svar
        </text>

        {/* Arrow: Project files → up to agent (short, clear) */}
        <line x1="122" y1="228" x2="122" y2="212" stroke="#22c55e" strokeWidth="1.5" markerEnd="url(#arrowGreen2)" />
        <text x="122" y="222" fill="#166534" fontSize="7.5" fontWeight="600" textAnchor="middle">
          Lest av agent
        </text>

        {/* Context exclusion */}
        <rect x="218" y="228" width="165" height="48" rx="8" fill="#fffbeb" stroke="#d97706" strokeWidth="1" />
        <text x="300" y="246" textAnchor="middle" fill="#92400e" fontSize="9" fontWeight="600">
          Context exclusion
        </text>
        <text x="300" y="261" textAnchor="middle" fill="#78716c" fontSize="7.5">
          .env, secrets, nøkler
        </text>

        {/* Blocked: dashed line from exclusion toward agent with X */}
        <line x1="300" y1="228" x2="300" y2="215" stroke="#dc2626" strokeWidth="1.5" strokeDasharray="3 2" />
        <line x1="295" y1="210" x2="305" y2="220" stroke="#dc2626" strokeWidth="2.5" />
        <line x1="305" y1="210" x2="295" y2="220" stroke="#dc2626" strokeWidth="2.5" />
        <text x="325" y="213" fill="#dc2626" fontSize="7.5" fontWeight="600">
          Blokkert av sandbox
        </text>

        {/* ══════ Local MCP — inside machine ══════ */}

        {/* Tool calls arrow: agent bottom → down to local MCP */}
        <line x1="155" y1="200" x2="155" y2="298" stroke="#6366f1" strokeWidth="2" markerEnd="url(#arrowIndigo)" />

        {/* Local MCP servers */}
        <rect x="40" y="298" width="345" height="105" rx="8" fill="#f0f9ff" stroke="#6366f1" strokeWidth="1" />
        <text x="212" y="318" textAnchor="middle" fill="#4338ca" fontSize="9" fontWeight="600">
          Lokale MCP-servere
        </text>
        <text x="212" y="332" textAnchor="middle" fill="#64748b" fontSize="8">
          Kjører mot lokale prosesser på maskinen
        </text>

        <rect x="55" y="345" width="72" height="34" rx="4" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="91" y="360" textAnchor="middle" fill="#334155" fontSize="8" fontWeight="600">
          playwright
        </text>
        <text x="91" y="372" textAnchor="middle" fill="#94a3b8" fontSize="7">
          nettleser
        </text>

        <rect x="135" y="345" width="65" height="34" rx="4" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="167" y="360" textAnchor="middle" fill="#334155" fontSize="8" fontWeight="600">
          next.js
        </text>
        <text x="167" y="372" textAnchor="middle" fill="#94a3b8" fontSize="7">
          dev server
        </text>

        <rect x="208" y="345" width="65" height="34" rx="4" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="240" y="360" textAnchor="middle" fill="#334155" fontSize="8" fontWeight="600">
          intellij
        </text>
        <text x="240" y="372" textAnchor="middle" fill="#94a3b8" fontSize="7">
          IDE
        </text>

        <text x="212" y="395" textAnchor="middle" fill="#166534" fontSize="7.5">
          Ingen ekstern kommunikasjon
        </text>

        {/* ══════ Bottom info inside machine ══════ */}
        <text x="200" y="430" textAnchor="middle" fill="#475569" fontSize="8">
          Ingen personopplysninger i kodebaser
        </text>
        <text x="200" y="445" textAnchor="middle" fill="#475569" fontSize="8">
          Sandbox blokkerer hemmeligheter på kernel-nivå
        </text>
        <text x="200" y="460" textAnchor="middle" fill="#475569" fontSize="8">
          Skills gir ingen ekstra tilgang
        </text>

        {/* MCP Registry annotation */}
        <rect x="40" y="475" width="345" height="30" rx="5" fill="#eef2ff" stroke="#6366f1" strokeWidth="0.5" />
        <text x="212" y="488" textAnchor="middle" fill="#4338ca" fontSize="8.5" fontWeight="600">
          Godkjent via MCP Registry
        </text>
        <text x="212" y="500" textAnchor="middle" fill="#64748b" fontSize="7.5">
          Harnessen henter allowlist-config fra registry ved oppstart
        </text>

        {/* ══════ OUTSIDE MACHINE: Right side ══════ */}

        {/* ── Trust boundary 2: GitHub Copilot API ── */}
        <rect
          x="480"
          y="55"
          width="175"
          height="210"
          rx="10"
          fill="#eff6ff"
          stroke="#3b82f6"
          strokeWidth="1.5"
          strokeDasharray="6 3"
        />
        <text x="495" y="73" fill="#1e40af" fontSize="10" fontWeight="600">
          GitHub Copilot API
        </text>
        <text x="495" y="86" fill="#3b82f6" fontSize="7.5">
          Tillitsgrense (databehandleravtale)
        </text>

        {/* Gateway box */}
        <rect x="492" y="95" width="150" height="158" rx="8" fill="white" stroke="#3b82f6" strokeWidth="1" />
        <text x="567" y="115" textAnchor="middle" fill="#1e40af" fontSize="11" fontWeight="600">
          Gateway
        </text>
        <text x="567" y="133" textAnchor="middle" fill="#64748b" fontSize="8">
          Autentisering
        </text>
        <text x="567" y="148" textAnchor="middle" fill="#64748b" fontSize="8">
          Org policy
        </text>
        <text x="567" y="163" textAnchor="middle" fill="#64748b" fontSize="8">
          MCP allowlist
        </text>
        <text x="567" y="178" textAnchor="middle" fill="#64748b" fontSize="8">
          Ingen lagring/trening
        </text>
        <text x="567" y="193" textAnchor="middle" fill="#64748b" fontSize="8">
          Ruter til provider
        </text>
        <text x="567" y="208" textAnchor="middle" fill="#64748b" fontSize="8">
          Data kastes etter svar
        </text>

        {/* Arrow: GitHub → Model providers */}
        <line x1="655" y1="160" x2="778" y2="160" stroke="#a855f7" strokeWidth="2" markerEnd="url(#arrowPurple2)" />

        {/* ── Trust boundary 3: Model providers ── */}
        <rect
          x="780"
          y="55"
          width="130"
          height="210"
          rx="10"
          fill="#faf5ff"
          stroke="#a855f7"
          strokeWidth="1.5"
          strokeDasharray="6 3"
        />
        <text x="795" y="73" fill="#6b21a8" fontSize="10" fontWeight="600">
          Model providers
        </text>
        <text x="795" y="86" fill="#a855f7" fontSize="7.5">
          Via GitHub (ikke direkte)
        </text>

        <rect x="790" y="100" width="110" height="33" rx="5" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="845" y="120" textAnchor="middle" fill="#334155" fontSize="8">
          OpenAI (GPT-4o, o3)
        </text>

        <rect x="790" y="140" width="110" height="33" rx="5" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="845" y="160" textAnchor="middle" fill="#334155" fontSize="8">
          Anthropic (Claude)
        </text>

        <rect x="790" y="180" width="110" height="33" rx="5" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="845" y="200" textAnchor="middle" fill="#334155" fontSize="8">
          Google (Gemini)
        </text>

        {/* ══════ Remote MCP — OUTSIDE machine boundary ══════ */}

        {/* Tool calls arrow: agent right side → straight right to remote MCP */}
        <line x1="258" y1="155" x2="490" y2="340" stroke="#6366f1" strokeWidth="1.5" markerEnd="url(#arrowIndigo)" />

        <rect x="490" y="290" width="230" height="105" rx="8" fill="white" stroke="#6366f1" strokeWidth="1.5" />
        <text x="605" y="310" textAnchor="middle" fill="#4338ca" fontSize="9" fontWeight="600">
          Remote MCP-servere
        </text>
        <text x="605" y="324" textAnchor="middle" fill="#64748b" fontSize="8">
          Kobler til eksterne API-er
        </text>

        <rect x="505" y="337" width="80" height="34" rx="4" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="545" y="352" textAnchor="middle" fill="#334155" fontSize="8" fontWeight="600">
          github-mcp
        </text>
        <text x="545" y="364" textAnchor="middle" fill="#94a3b8" fontSize="7">
          api.github.com
        </text>

        <rect x="593" y="337" width="72" height="34" rx="4" fill="white" stroke="#e2e8f0" strokeWidth="1" />
        <text x="629" y="352" textAnchor="middle" fill="#334155" fontSize="8" fontWeight="600">
          figma-mcp
        </text>
        <text x="629" y="364" textAnchor="middle" fill="#94a3b8" fontSize="7">
          api.figma.com
        </text>

        <rect
          x="673"
          y="337"
          width="35"
          height="34"
          rx="4"
          fill="#f8fafc"
          stroke="#6366f1"
          strokeWidth="0.5"
          strokeDasharray="3 2"
        />
        <text x="690" y="359" textAnchor="middle" fill="#6366f1" fontSize="9">
          …
        </text>

        {/* Arrow: Remote MCP → External APIs */}
        <line
          x1="605"
          y1="395"
          x2="605"
          y2="420"
          stroke="#dc2626"
          strokeWidth="1.5"
          strokeDasharray="4 2"
          markerEnd="url(#arrowRed2)"
        />

        {/* ══════ External APIs ══════ */}
        <rect x="490" y="420" width="230" height="90" rx="8" fill="#fef2f2" stroke="#fca5a5" strokeWidth="1" />
        <text x="605" y="442" textAnchor="middle" fill="#991b1b" fontSize="9" fontWeight="600">
          Eksterne API-er
        </text>
        <text x="605" y="460" textAnchor="middle" fill="#64748b" fontSize="8">
          api.github.com · api.figma.com · …
        </text>
        <text x="605" y="478" textAnchor="middle" fill="#64748b" fontSize="8">
          Nais-apper · andre tjenester
        </text>
        <text x="605" y="500" textAnchor="middle" fill="#991b1b" fontSize="7.5">
          Autentisert per server (OAuth/token)
        </text>

        {/* ── Legend ── */}
        <rect x="750" y="300" width="155" height="130" rx="6" fill="#f8fafc" stroke="#e2e8f0" strokeWidth="1" />
        <text x="765" y="320" fill="#334155" fontSize="9" fontWeight="600">
          Dataflyt
        </text>
        <line x1="765" y1="336" x2="785" y2="336" stroke="#3b82f6" strokeWidth="2" />
        <text x="790" y="340" fill="#475569" fontSize="7.5">
          Inference
        </text>
        <line x1="765" y1="356" x2="785" y2="356" stroke="#6366f1" strokeWidth="1.5" />
        <text x="790" y="360" fill="#475569" fontSize="7.5">
          Agent → MCP
        </text>
        <line x1="765" y1="376" x2="785" y2="376" stroke="#dc2626" strokeWidth="1.5" strokeDasharray="4 2" />
        <text x="790" y="380" fill="#475569" fontSize="7.5">
          MCP → ekstern API
        </text>
        <line x1="765" y1="396" x2="785" y2="396" stroke="#3b82f6" strokeWidth="1" strokeDasharray="3 2" />
        <text x="790" y="400" fill="#475569" fontSize="7.5">
          Config ved oppstart
        </text>
        <line x1="765" y1="416" x2="785" y2="416" stroke="#22c55e" strokeWidth="1.5" />
        <text x="790" y="420" fill="#475569" fontSize="7.5">
          Fillesing
        </text>

        {/* Arrow markers */}
        <defs>
          <marker id="arrowGreen2" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="#22c55e" />
          </marker>
          <marker id="arrowBlue2" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="#3b82f6" />
          </marker>
          <marker id="arrowBlueLeft" markerWidth="8" markerHeight="6" refX="0" refY="3" orient="auto">
            <polygon points="8 0, 0 3, 8 6" fill="#3b82f6" />
          </marker>
          <marker id="arrowPurple2" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="#a855f7" />
          </marker>
          <marker id="arrowIndigo" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="#6366f1" />
          </marker>
          <marker id="arrowRed2" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="#dc2626" />
          </marker>
        </defs>
      </svg>
    </div>
  );
}
