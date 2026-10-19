import { ProviderError, defineProvider, object, secrets, string } from "apps";

// Executor reaches Traicr over the Docker `proxy` network, like the other
// homelab apps, so plain HTTP stays inside that network. Change this if the
// container is named differently.
export const host = "traicr-server:8080";
export const origin = `http://${host}`;

// Credentials are entered through Executor's account form, never app source.
// With `hosts`, app code holds a handle that Executor fills in only on
// requests to Traicr. There is no `request` placement: Executor sends placed
// credentials only over HTTPS.
export const provider = defineProvider({
  name: "Traicr",
  hosts: [host],
  auth: {
    token: secrets({
      label: "Traicr admin token (TRAICR_ADMIN_TOKEN)",
      fields: object({ token: string({ minLength: 1 }) }),
    }),
  },
  health: async ({ account, fetch, signal }) => {
    // The status summary is a cheap read that needs a valid token.
    const response = await fetch(`${origin}/api/v1/status`, {
      signal,
      headers: { authorization: `Bearer ${account.fields.token}` },
    });
    if (response.status === 401) throw new ProviderError({ reason: "unauthorized", status: 401 });
    if (!response.ok) throw new ProviderError({ reason: "unavailable", status: response.status });
    const { version } = (await response.json()) as { version?: unknown };
    const shown = typeof version === "string" && /^[\w.+-]{1,32}$/.test(version) ? version : "";
    return { accountInfo: { displayName: `Traicr ${shown}`.trim() } };
  },
});
