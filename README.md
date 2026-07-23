# go-tangra-netboot

Tangra control-plane module for **netbootd** (the [universe](../../universe) project): bare-metal
provisioning over PXE/iPXE with Ubuntu autoinstall. netbootd runs on a separate host, owns the DHCP
and TFTP servers and all provisioning state; this module federates it into the Tangra platform's
gRPC mesh and Vben admin UI.

## What this module is

A **stateless, authorizing proxy**. It has no database. Machines, profiles, DHCP configuration,
sessions and boot artifacts all live on the remote netbootd instance; this module contributes:

- **Tangra-native gRPC API** — `netboot.service.v1`, registered with the admin gateway alongside its
  OpenAPI document, proto descriptor and menu definitions.
- **Authorization** — netbootd has a single coarse operator identity, so fine-grained RBAC is
  entirely this module's responsibility. Every RPC checks a permission before the upstream is
  touched, and fails closed.
- **Credential custody** — the netbootd operator credentials never leave this process; browser
  clients authenticate to Tangra, not to netbootd.
- **Transport hardening** — TLS pinning, bounded retries, response size limits, cross-host redirect
  refusal, single-flight re-authentication.
- **Audit, metrics and mTLS** — via the standard `go-tangra-common` middleware.

```
browser ──HTTPS──▶ admin gateway ──mTLS gRPC──▶ go-tangra-netboot ──HTTPS──▶ netbootd (separate host)
   Vben UI          transcodes REST             authorizes, maps           DHCP/TFTP/boot HTTP
   (federated)      attaches identity           translates errors          TimescaleDB + Valkey
```

## gRPC services

| Service | RPCs | Purpose |
|---------|------|---------|
| `NetbootMachineService` | List, Get, Create, Update, Delete, Provision, CancelProvision, ListUnknownBoots, RegisterUnknownMachine | Machine inventory and provisioning |
| `NetbootProfileService` | List, Get, Create, Update, Clone, Delete, Preview | Autoinstall profiles |
| `NetbootDhcpService` | GetConfig, UpdateConfig, Enable, Disable, ListLeases, ListForeignServers | DHCP scope and leases |
| `NetbootSessionService` | List, Get | Provisioning history and event timelines |
| `NetbootArtifactService` | List, Get, Delete, ListTransfers | Kernels, initrds, iPXE binaries |
| `NetbootSystemService` | Health, GetInfo, CheckUpstream, GetStats | Health and dashboard counters |

**Ports:** 10000 gRPC (mTLS), 10001 HTTP (federated frontend + descriptors), 10010 Prometheus.

## Permission model

| Permission | Viewer | Operator | Admin |
|------------|:------:|:--------:|:-----:|
| `netboot.*.view`, `netboot.system.view` | ✔ | ✔ | ✔ |
| `netboot.machine.create` / `.update` / `.provision` | | ✔ | ✔ |
| `netboot.profile.create` / `.update` | | ✔ | ✔ |
| `netboot.machine.delete`, `netboot.profile.delete` | | | ✔ |
| `netboot.dhcp.manage` | | | ✔ |
| `netboot.artifact.delete` | | | ✔ |

`platform:admin`, `super:admin` and `tenant:manager` carry the full admin set. Roles arrive as
`x-md-global-roles` gRPC metadata from the admin-service transcoder; a request with no roles is
denied every permission.

Two deliberate escalations beyond the obvious mapping:

- **`PreviewProfile` requires `profile.update`, not `profile.view`.** The rendered seed exposes the
  entire installation recipe, so it is restricted to the audience that may change it.
- **DHCP management and artifact deletion are admin-only.** Mis-scoping the provisioning network or
  removing a kernel breaks every machine on the segment, which is a different magnitude of change
  from arming one host.

## Configuration

The upstream connection is configured **entirely through the environment**, so credentials never
live in a config file that could be baked into an image.

| Variable | Required | Default | Purpose |
|----------|:--------:|---------|---------|
| `NETBOOTD_ENDPOINT` | ✔ | — | `https://netbootd.example.internal:8080` |
| `NETBOOTD_USERNAME` | ✔ | — | Operator account on the netbootd instance |
| `NETBOOTD_PASSWORD` | ✔* | — | That account's password |
| `NETBOOTD_PASSWORD_FILE` | ✔* | — | Path to a mounted secret holding the password (**preferred**) |
| `NETBOOTD_CA_FILE` | | system roots | PEM bundle used to verify the upstream certificate |
| `NETBOOTD_TIMEOUT` | | `15s` | Per-request timeout |
| `NETBOOTD_MAX_RETRIES` | | `2` | Retries for idempotent requests |
| `NETBOOTD_MAX_RESPONSE_BYTES` | | `8388608` | Response size cap |
| `NETBOOTD_ALLOW_PLAINTEXT` | | `false` | Permit an `http://` upstream |
| `NETBOOTD_INSECURE_SKIP_VERIFY` | | `false` | Development only; **requires** `ALLOW_PLAINTEXT` too |

\* one of `NETBOOTD_PASSWORD` or `NETBOOTD_PASSWORD_FILE`. The file form is preferred: an
environment variable is readable from `/proc` and leaks into crash dumps.

Platform variables (`ADMIN_GRPC_ENDPOINT`, `FRONTEND_ENTRY_URL`, `GRPC_ADVERTISE_ADDR`,
`LCM_BOOTSTRAP_ENDPOINT`, `MODULE_BOOTSTRAP_SECRET`, `LCM_CA_FINGERPRINT`, `CERTS_DIR`,
`NETBOOT_HTTP_ADDR`, `METRICS_ADDR`) follow the conventions of every other Tangra module.

A module with no `NETBOOTD_ENDPOINT` starts successfully, reports itself unhealthy and fails every
call with `CONFIGURATION_ERROR` — it does not crash-loop. A *malformed* endpoint does fail startup,
because silently degrading would leave operators staring at an empty machine list with no
explanation.

## Security notes

- **Credentials never render.** `netbootd.Secret` implements `Stringer`, `GoStringer` and
  `json.Marshaler` to emit `[REDACTED]`; the plaintext is reachable only through an explicit
  `Reveal()` call, and the profile password is injected into the outbound body by a single
  `MarshalJSON` method. `ProfileInput.password` is additionally marked `(redact.v3.value)` so the
  redacting gRPC registrars strip it from anything echoed back.
- **No SSRF surface.** The upstream host comes from configuration only; request data reaches it as
  escaped path segments and query values against compile-time path constants.
- **Cross-host redirects are refused** rather than followed, so the session cookie cannot be handed
  to a third party by a redirect.
- **TLS 1.2 minimum**, with an optional CA bundle that *replaces* the system roots when supplied.
  `NETBOOTD_INSECURE_SKIP_VERIFY` alone is rejected — it must be paired with an explicit plaintext
  acknowledgement, so one stray variable cannot disable verification in production.
- **Bounded everything**: request timeout, retry count, exponential backoff, response body size.
  Oversized bodies are rejected rather than truncated, so partial JSON is never parsed.
- **Non-idempotent requests are never replayed.** A failed `POST` may already have been applied
  upstream; only reads, `PUT` and `DELETE` are retried.
- **Upstream 5xx and transport detail are not relayed.** They can name internal hosts and database
  objects; the caller receives a sanitised reason and the detail goes to the log. Validation
  details (422) *are* forwarded, since they are safe and actionable.
- **Uploads are deliberately not proxied.** Artifact upload is a multipart stream of kernel-sized
  payloads; relaying it would let any authorized operator pin hundreds of megabytes of this
  module's memory. Operators upload to the netbootd host directly.

## Frontend

A Vue 3 module-federation remote (`@tangra/module-netboot`) exposing `./module` from
`src/index.ts`, consuming the shell remote for layout, stores and i18n. The production build is
embedded into the Go binary via `go:embed all:frontend-dist` and served from `/modules/netboot/`;
`cmd/server/assets/menus.yaml` declares the Vben menus, dashboard widgets, permissions and roles.

```bash
cd frontend
pnpm install
pnpm dev        # http://localhost:3014, expects the shell on :5666
pnpm build      # dist/ -> cmd/server/assets/frontend-dist/ in the Docker build
```

## Build and test

```bash
make generate          # buf lint + generate + openapi + descriptor + wire
make build-server      # ./bin/netboot-server
make test              # go test -race ./...
make test-cover        # coverage.html across ./internal/...
make test-cover-check  # fails below 80% statement coverage
make lint              # gofmt + go vet
make docker            # container image
```

Tests run against an in-process stub of the netbootd admin API that reproduces its response
envelope and session-cookie authentication, so the JSON contract — `protojson`'s snake_case names,
64-bit integers encoded as strings, `EmitUnpopulated` nulls — is exercised for real rather than
mocked away.

## Dependencies

- **Framework**: Kratos v2, `kratos-bootstrap`, Wire
- **Protobuf**: Buf, protovalidate, `protoc-gen-redact`
- **Platform**: `go-tangra-common` (registration, mTLS, audit, metrics, grpcx)
- **Upstream**: netbootd admin API (`universe/backend/api/netboot/v1`)
