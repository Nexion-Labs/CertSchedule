# CertSchedule

Web app for issuing and auto-renewing TLS certificates via [certbot](https://certbot.eff.org/) and syncing them into Kubernetes `kubernetes.io/tls` Secrets. Manage domains from a React UI; renewals run on an in-process scheduler — no external CronJob needed.

- **Backend**: Go, hexagonal (ports & adapters) architecture, SQLite storage, JWT auth
- **Frontend**: React + TypeScript + Vite + Tailwind, built and embedded into the Go binary — one container, one process
- **Certs**: certbot CLI run as a subprocess, HTTP-01 or DNS-01 (Cloudflare / AWS Route53) per domain
- **Scheduling**: in-process cron (`robfig/cron`), checks for certs nearing expiry and renews automatically

## Repository layout

```
cmd/server/            composition root (main.go)
internal/domain/       entities + port interfaces (no external deps)
internal/application/  use cases (DomainService, CertificateService, SchedulerService, AuthService)
internal/adapters/     httpapi, sqlite, certbot, k8s, scheduler — implement/consume domain ports
internal/platform/     crypto, jwt, logging
internal/webui/         embeds the built web/dist frontend into the binary
web/                    React frontend (Vite + TS + Tailwind)
deploy/k8s/             Deployment, Service, RBAC, PVC manifests
```

## Local development

Backend and frontend run as two separate dev servers, proxied together by Vite.

```bash
cp .env.example .env        # edit JWT_SECRET / ENCRYPTION_KEY / ADMIN_PASSWORD
make run                    # starts the Go API on :8080

cd web && npm install
npm run dev                 # Vite dev server on :5173, proxies /api and /healthz to :8080
```

Open http://localhost:5173, log in with `ADMIN_USERNAME`/`ADMIN_PASSWORD` from `.env`.

Set `CERTBOT_DRY_RUN=true` (and/or `CERTBOT_STAGING=true`) while developing so you don't hit Let's Encrypt's real rate limits. `CERTBOT_DRY_RUN=true` skips writing certs to disk entirely — issuance/renewal jobs still run and log success/failure, but no `Certificate` row or Kubernetes Secret update happens.

Requires a `certbot` binary on `PATH` for real (non-dry-run) issuance — install with `brew install certbot` / `apt install certbot` plus the relevant DNS plugin (`certbot-dns-cloudflare`, `certbot-dns-route53`) for DNS-01 domains.

### Tests

```bash
go test ./...
go vet ./...
```

## Single-binary build

`make build-full` builds the frontend, copies its output into `internal/webui/dist`, then compiles a single Go binary that serves both the API and the SPA:

```bash
make build-full
./bin/certschedule
```

## Docker

```bash
docker build -t certschedule:latest .
```

The image is a 3-stage build: Node builds the frontend, Go compiles the binary with the frontend embedded, and the runtime stage is `python:slim` with `certbot` + the Cloudflare/Route53 DNS plugins installed (certbot is itself a Python tool, so this is the natural runtime base).

For local docker-compose testing:

```bash
cp .env.example .env   # fill in JWT_SECRET / ENCRYPTION_KEY / ADMIN_PASSWORD
docker compose up --build
```

Without a mounted kubeconfig, the app starts fine but k8s Secret updates fail (logged as a warning, not a crash) — uncomment the kubeconfig volume/env lines in `docker-compose.yml` to point it at a real cluster. Inside an actual cluster, the app auto-detects and uses the in-cluster ServiceAccount instead — no kubeconfig needed there.

## Kubernetes deployment

Manifests are in `deploy/k8s/`:

```bash
kubectl apply -f deploy/k8s/deployment.yaml   # creates the certschedule namespace too
kubectl apply -f deploy/k8s/rbac.yaml
kubectl apply -f deploy/k8s/pvc.yaml
cp deploy/k8s/secret.example.yaml deploy/k8s/secret.yaml   # fill in real values, do not commit
kubectl apply -f deploy/k8s/secret.yaml
kubectl apply -f deploy/k8s/service.yaml
```

Notes:

- **Single replica only.** SQLite is single-writer; the Deployment is pinned to `replicas: 1` with `strategy: Recreate`. Don't scale this horizontally without swapping SQLite for a real database.
- **RBAC is cluster-wide by design.** Domains can target any namespace/Secret name, so the shipped `ClusterRole` grants `get/list/create/update/patch` on `secrets` across the whole cluster. Narrow this to specific namespaces (`Role` + `RoleBinding` per namespace) if your environment requires tighter scoping.
- **Persistence.** The PVC backs `/app/data`, which holds the SQLite DB, certbot's config/work/logs dirs, and the HTTP-01 webroot. Losing this volume loses domain/cert history (though live certs already pushed to k8s Secrets are unaffected).
- Set `CERTBOT_STAGING=false` in the Deployment (already the default there) once you're ready to issue real certificates — leave it `true` while testing manifests/rollout to avoid Let's Encrypt rate limits.

## Export / import domain config

Domain configuration can be backed up and restored as JSON, from the "Export"/"Import" buttons on the domain list, or directly:

```bash
# Export (add ?include_credentials=true to include decrypted DNS provider
# credentials — treat that output as a plaintext secrets file)
curl -s "http://localhost:8080/api/v1/domains/export" -H "Authorization: Bearer $TOKEN" > domains.json

# Import: upserts by domain name — an existing domain's namespace/secret/
# auto-renew settings are updated in place, a new name creates a new domain.
# Credentials are optional on import; a DNS-01 domain imported without one
# just can't issue until you add credentials via Edit.
curl -s -X POST "http://localhost:8080/api/v1/domains/import" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  --data @domains.json
```

The import response reports a per-domain `created`/`updated`/`error` outcome so a bad entry doesn't block the rest of the file from importing.

## Download certbot data (full backup)

This is unrelated to the JSON export above: that covers only the app's own database fields (namespace, secret name, DNS provider, etc). This downloads certbot's actual on-disk state — `config/live`, `config/archive`, `config/renewal/*.conf`, `config/accounts`, and the DNS credential files under `credentials/` — as a single `tar.gz`, from the "Download certbot data" button on the domain list, or directly:

```bash
curl -s "http://localhost:8080/api/v1/certbot/archive" -H "Authorization: Bearer $TOKEN" -o certbot-data.tar.gz
```

Treat this archive as a plaintext secrets bundle: certbot stores private keys and DNS provider credentials unencrypted on disk (unlike this app's own DB rows, which encrypt the key and DNS credential at rest). It's meant for migrating or restoring certbot's renewal state to a new host. Note that `renewal/*.conf` files record the **original** `--config-dir` as an absolute path — if you restore the archive under a different absolute path, edit those `.conf` files accordingly before running `certbot renew`.

## Configuration reference

See `.env.example` for the full list of environment variables (HTTP address, SQLite path, JWT/encryption secrets, admin credentials, certbot binary/flags, scheduler interval, kubeconfig path).

## Security notes

- DNS provider credentials and issued private keys are encrypted at rest (AES-256-GCM) using `ENCRYPTION_KEY` — treat that key like any other production secret; losing it makes stored credentials/keys unrecoverable.
- Auth is a single shared admin account (JWT + bcrypt password), suitable for a small ops team behind existing network controls — it is not a multi-tenant user system.
