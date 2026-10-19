import { defineApp, withApprovals } from "apps";
import { liveOpenapiRouter } from "apps/openapi";
import { always } from "apps/operations/approval";
import { origin, provider } from "./provider.js";

// Every tool comes from Traicr's own OpenAPI contract at
// /api/v1/openapi.json (docs/specs/agent-api.md, ADR 0011), so a server change
// reaches agents without editing this app. Fix a wrong tool in that document,
// not here. One shared admin token, linked by someone who can edit the app.
const requirements = { app: { accounts: { service: provider } } };

// Deleting is permanent and uploading writes the archive, so both ask first.
// Renaming and every read run without asking.
const askFirst = new Set(["traces.deleteTrace", "imports.uploadTraceZip"]);

export default defineApp(requirements, async ({ app, cache, fetch, signal }) => ({
  tools: withApprovals(
    liveOpenapiRouter({
      source: { url: `${origin}/api/v1/openapi.json` },
      allowedOrigin: origin,
      baseUrl: origin,
      securitySchemes: { adminToken: { type: "http", scheme: "bearer" } },
      methods: { token: [{ scheme: "adminToken", field: "token", part: "value", prefix: "" }] },
      oauth: [],
      account: app.accounts.service,
      cache,
      fetch,
      signal,
    }),
    (_operation, name) => (askFirst.has(name) ? always() : undefined),
  ),
}));
