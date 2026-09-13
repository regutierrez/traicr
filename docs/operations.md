# Operations

Traicr is a single-user service that trusts the network path to the server. Run one non-root container with one persistent data volume. Traicr serves plain HTTP; it does not manage TLS or backups.

## Direct access over Tailscale

`compose.yaml` publishes `8080:8080`, so Docker binds host port 8080 on every interface. Inside the container the process listens on `TRAICR_LISTEN_ADDRESS`, which defaults to `:8080`. Traicr does not read a bind-address environment variable.

To publish only on a Tailscale address, change the Compose `ports` mapping to that address:

```yaml
ports:
  - "100.64.0.10:8080:8080"
```

To publish only on this machine, use `127.0.0.1:8080:8080`. A host firewall can also limit who reaches the published port. Do not publish on every interface unless another firewall provides the intended isolation.

```sh
export TRAICR_ADMIN_TOKEN='replace-with-a-long-random-token'
docker compose up -d
```

Confirm the published socket with `docker compose ps` or the host's socket inspection tools.

Store the token in a root-owned environment file with mode `0600` when managing the service non-interactively. Do not put the token in Compose YAML, shell history, logs, or a source repository.

## Server configuration

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `TRAICR_ADMIN_TOKEN` | yes | none | API bearer token and key material for signed browser sessions |
| `TRAICR_LISTEN_ADDRESS` | no | `:8080` | Plain-HTTP server socket inside the process |
| `TRAICR_DATA_DIR` | no | `/data` | SQLite, retained Source Records, and temporary import files |
| `TRAICR_SECURE_COOKIES` | no | `false` | Set to `true` only when browsers reach Traicr through HTTPS |
| `TRAICR_MAX_UPLOAD_BYTES` | no | `8589934592` (8 GiB) | Maximum compressed upload size |
| `TRAICR_MAX_EXPANDED_BYTES` | no | `34359738368` (32 GiB) | Maximum total expanded archive size |
| `TRAICR_MAX_FILE_BYTES` | no | `4294967296` (4 GiB) | Maximum expanded size of one archived file |

The production container runs as UID and GID `10001:10001`, has a read-only root filesystem, and writes only to `/data`. A named Docker volume is initialized with the correct ownership. For a bind mount, create an empty directory owned by `10001:10001` with mode `0700`; do not make it world-writable.

```sh
sudo install -d -o 10001 -g 10001 -m 0700 /srv/traicr
```

Traicr retains unredacted source records. Restrict host access to the Docker volume and any copied archives accordingly.

## HTTPS with Caddy

Caddy may terminate TLS in front of Traicr. Keep the application on a private loopback or container-network address, set `TRAICR_SECURE_COOKIES=true`, and preserve the original `Host` header. Traicr deliberately ignores every forwarded header, including `Forwarded` and `X-Forwarded-*`; do not depend on them for client identity, scheme, or access control.

```caddyfile
traicr.example.ts.net {
	reverse_proxy traicr-server:8080 {
		header_up Host {host}
	}
}
```

Set `TRAICR_SECURE_COOKIES=true` before allowing browser logins through this HTTPS endpoint. Traicr itself still speaks HTTP behind Caddy.

## Uploads and retries

The collector uploads a complete numbered Trace ZIP to `POST /api/v1/imports`. The response is newline-delimited JSON on the same request. Phases are `validating`, per-trace `normalizing`, then `complete` or `failed`. Search indexing happens during `normalizing`, not as a separate phase. If the connection, collector, or server stops before a complete response, retry the whole ZIP. There is no resumable upload; imports are designed to deduplicate an exact retry.

One trace is never split between archives. The collector's split size is a soft target, so one large trace can produce an archive larger than that target. Configure all three upload limits for the largest trusted trace while retaining finite bounds against malformed archives.

## Token rotation

Replace `TRAICR_ADMIN_TOKEN` and restart the container:

```sh
export TRAICR_ADMIN_TOKEN='new-long-random-token'
docker compose up -d --force-recreate
```

Rotation immediately rejects the old bearer token and invalidates every browser session cookie signed with it. Log in again and update the token stored by each collector.

## Upgrades

Read the release notes, pull the new server image, and recreate the container while keeping the same `/data` volume:

```sh
docker compose pull
docker compose up -d
```

After restart, Traicr detects stored revisions produced by older normalizers and rebuilds them automatically, one revision at a time. A successful rebuild atomically replaces that revision's normalized observations. A failed rebuild retains the previous searchable Events and records the failure; it does not require recollecting the source machine.

Collector upgrades are manual. Download the binary for the source machine's operating system and architecture, verify it against `SHA256SUMS`, and replace the previous binary.

Backups are outside Traicr's first-release scope. An operator who adds volume snapshots must use SQLite-safe snapshot procedures and account for the database, WAL, and retained objects as one consistent data set.
