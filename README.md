# remote-juju-controller-store

A SQLite-backed HTTP server and CLI that replace Juju's local file-based
client store with a remote, authenticated store.

Controller metadata (controllers, models, accounts, credentials, bootstrap
config, cookies) is persisted in a single SQLite database instead of local
Juju YAML files.

This repo also includes JWT-based login plumbing so Juju can authenticate to
controllers as external users via RCS.

## What works

- Remote ClientStore implementation for Juju state metadata
- OIDC device-flow login via `rcs login`
- Persisted RCS session file at `~/.local/share/rcs/session.json`
- JWT minting endpoint for controller login (`POST /controllers/:name/token`)
- JWKS endpoint for controller-side JWT verification (`/.well-known/jwks.json`)
- Juju CLI integration:
	- auto-uses RemoteStore when RCS session exists
	- uses RCS JWT login provider for controller login

## Architecture in one minute

1. User runs `rcs login <rcsd-addr>`.
2. `rcs` completes OIDC device flow and exchanges the IdP token at
	 `POST /auth/device`.
3. `rcsd` mints an RCS session JWT and `rcs` saves it to
	 `~/.local/share/rcs/session.json`.
4. Juju sees the session file and switches from file store to RemoteStore.
5. On API connect, Juju asks RCS for a short-lived controller JWT
	 (`POST /controllers/:name/token`).
6. Juju sends that token in `Admin.Login` to the controller.
7. Controller verifies using RCS JWKS.

## Repository components

- `cmd/srv`: RCS server binary (`rcsd`)
- `cmd/rcs`: RCS CLI (`rcs login`, `logout`, `whoami`)
- `internal/store`: HTTP handlers and auth/token middleware
- `internal/db`: SQLite schema/init
- `api/openapi.yaml`: API contract
- `pkg/client`: generated API client

## Quickstart (local dev)

### 1. Start services

```bash
docker compose up -d --wait
```

This starts:

- `rcsd` on `:8484`
- Keycloak on `:3082`

### 2. Build CLI tools

```bash
make build link
```

This builds:

- `build/rcsd`
- `build/rcs`
- symlink `./rcs`

### 3. Log in to RCS

```bash
./rcs logout
./rcs login http://localhost:8484
./rcs whoami
```

After login, session is stored in:

- `~/.local/share/rcs/session.json`

with fields:

- `addr`
- `token`

## Juju integration behavior
Once the patch within `./juju/4.0.patch` is applied to a the 4.0 branch of Juju, the following behaviour happens:

When `~/.local/share/rcs/session.json` exists and is valid:

- Juju uses `RemoteStore` (RCS-backed) instead of local file store.
- Juju uses the RCS JWT login provider for controller auth.

When no valid session exists:

- Juju falls back to the default local file-backed client store.

## Bootstrap and controller JWT setup

Use RCS JWKS as the login token refresh URL when bootstrapping:

```bash
juju bootstrap lxd a \
	--config login-token-refresh-url=http://<rcsd-host>:8484/.well-known/jwks.json
```

For local LXD bridge setups, resolve host bridge IP and substitute `<rcsd-host>`.

## Controller model visibility note

If bootstrap is performed as controller admin but subsequent calls are made as
an external user, that user may not see `admin/controller` until granted access.

Grant admin on controller model:

```bash
juju grant alice@example.com admin admin/controller
```

## Useful commands

```bash
# Build both binaries
make build

# Build and symlink ./rcs
make build link

# Run server locally
make serve

# Regenerate API client
make generate
```

## Developer docs

- `docs/developer/rcs-session-auth.txt`
- `docs/developer/juju-cli-rcs-auth.txt`
- `docs/developer/remote-store.txt`
- `docs/qa/happy-path.txt`

## Current limitations / next areas

- Model-level grants for external users are not fully automated yet.
- OpenFGA integration exists in compose but is not the primary authz path for
	this happy-path flow.

## Security notes

- RCS session tokens are bearer tokens; protect `session.json`.
- Session file is written with mode `0600` by the CLI.
- Controller login JWTs are intentionally short-lived.