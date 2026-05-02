Controller Auth via RCS-Minted JWTs
=====================================

Overview
--------
RCS replaces JIMM as the identity/auth broker between the Juju CLI and
controllers. Instead of macaroon or password login, the CLI obtains a
short-lived JWT from RCS and presents it directly to the controller.
The controller validates the JWT signature using RCS's JWKS endpoint.

No juju login. Only rcs login.

Flow
----
1. User runs `rcs login` -> session.json written to
   ~/.local/share/rcs/session.json containing {token, server}.

2. User runs `juju add-controller` (or equivalent) to register a controller
   with RCS. RCS stores the controller UUID and API addresses.

3. CLI detects session.json on startup (already done in clientStore()).
   All ClientStore calls go to RCS over HTTP using the session Bearer token.

4. On any `juju` command that needs a controller connection, the CLI's
   LoginProvider calls RCS to mint a short-lived JWT for the target
   controller, then sends that JWT directly to the controller in
   LoginRequest.Token (exactly what JIMM does in createLoginRequest).

5. The controller validates the JWT signature via its JWKS cache, which
   points at RCS's /.well-known/jwks.json (set via login-token-refresh-url
   controller config).

JWT Format (matches what Juju's JWTAuthenticator expects)
---------------------------------------------------------
  alg: RS256
  claims:
    sub:    "user-<name>@external"   (Juju user tag string)
    aud:    ["<controller-uuid>"]
    iss:    "<rcs-server-url>"
    exp:    now + 60s                (single-use, short-lived)
    jti:    <unique id>              (replay protection)
    access: {
      "controller-<uuid>": "superuser",
      "model-<uuid>":      "admin"    (optional, per-command scope)
    }

Wire encoding: base64.StdEncoding.EncodeToString(jwtBytes)
Sent as: LoginRequest{AuthTag: userTag, Token: base64JWT}
         Authorization: Bearer <base64JWT>  (for HTTP endpoints)

Implementation Steps
--------------------

Step 1 — RCS: JWKS endpoint
  File: internal/store/ (new handler or existing router)
  - Generate an RSA-2048 key pair on startup (or load from config/secret).
  - Expose GET /.well-known/jwks.json returning the public key in JWK Set
    format (RFC 7517). No auth required.
  - The controller admin sets login-token-refresh-url to this URL after
    bootstrapping.

Step 2 — RCS: JWT minting endpoint
  File: internal/store/ (new handler)
  - POST /rcs/v1/controllers/{controller_uuid}/token
  - Auth: Bearer session token (existing middleware already handles this).
  - Checks: caller must have access to the named controller in OpenFGA.
  - Mints JWT with claims above, signs with the RSA private key.
  - Returns: {"token": "<base64-encoded-jwt>"}
  - Add route to OpenAPI spec and regenerate client.

Step 3 — Juju: RCS JWT LoginProvider
  File: juju/api/rcsjwtloginprovider.go  (new file)
  - Type rcsJWTLoginProvider implementing api.LoginProvider.
  - Constructor takes: rcsServerURL, rcsSessionToken, controllerUUID.
  - Login():
      1. POST to RCS token endpoint, get base64 JWT.
      2. caller.APICall(ctx, "Admin", 3, "", "Login",
             &params.LoginRequest{AuthTag: userTag, Token: jwt}, &result)
      3. Return api.NewLoginResultParams(result).
  - AuthHeader(): return Authorization: Bearer <jwt> (for debug-log etc.)
  - String(): return "RCSJWTLoginProvider"
  Note: mirrors jwtLoginProvider in apiserver/admin_external_login_jwt_test.go
        and JIMM's createLoginRequest in internal/jujuclient/dial.go.

Step 4 — Juju: wire LoginProvider in cmd/modelcmd/base.go
  File: juju/cmd/modelcmd/base.go  (~line 614)
  - Before the OIDCLogin check, add:
      if _, err := os.Stat(rcsSessionPath); err == nil {
          session := loadRCSSession()  // already exists in main.go helpers
          dialOpts.LoginProvider = api.NewRCSJWTLoginProvider(
              session.Server,
              session.Token,
              controllerDetails.ControllerUUID,
          )
      }
  - This means: any juju command that talks to a controller will
    automatically use JWT auth when session.json is present.
  - OIDCLogin block remains unchanged for non-RCS controllers.

Non-goals (for now)
-------------------
- accounts.yaml / AccountDetails changes (no-op UpdateAccount already
  planned; AccountDetails returns ephemeral external user identity).
- juju login command (irrelevant in RCS mode).
- Model-scoped access claims (start with controller-level superuser,
  tighten later).

Key References
--------------
- JWT authenticator (controller side):
    juju/apiserver/authentication/jwt/jwt.go
- JWKS parser worker (reads login-token-refresh-url):
    juju/internal/worker/jwtparser/worker.go
    juju/internal/jwtparser/jwt.go
- Controller config key:
    juju/controller/config.go  LoginTokenRefreshURL = "login-token-refresh-url"
- Reference JWT login provider (test):
    juju/apiserver/admin_external_login_jwt_test.go  jwtLoginProvider
- JIMM's equivalent JWT login (the thing we're replicating for CLI):
    jimm/internal/jujuclient/dial.go  createLoginRequest
- LoginProvider interface:
    juju/api/interface.go  LoginProvider
- Current injection point for LoginProvider:
    juju/cmd/modelcmd/base.go  ~line 614  (OIDCLogin block)
