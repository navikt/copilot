"""CI only: stands in for Texas so the mcp-compliance Action can run against
mcp-kodeverk in ci.yaml. Answers /introspect and /exchange on 127.0.0.1. Copied
from navikt/mcp-server-kotlin-template; never deployed."""
import json
import os
from http.server import BaseHTTPRequestHandler, HTTPServer

CLAIMS = {
    "ci-user-token": {"active": True, "idtyp": "user", "NAVident": "Z999999", "azp": "ci-gateway"},
    "ci-app-token": {"active": True, "idtyp": "app", "roles": ["access_as_application"], "azp": "ci-gateway"},
}


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))) or "{}")
        if self.path == "/introspect":
            out = CLAIMS.get(body.get("token"), {"active": False})
        elif self.path == "/exchange":
            out = {"access_token": "ci-obo-token"}
        else:
            self.send_error(404)
            return
        data = json.dumps(out).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *_):
        pass  # no request logging: bodies carry tokens


HTTPServer(("127.0.0.1", int(os.environ.get("FAKE_TEXAS_PORT", "8099"))), Handler).serve_forever()
