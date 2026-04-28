Namespaces Implementation Plan v2

Goal
- Implement namespace-aware RCS storage and authorization with production-safe
  behavior for Juju bootstrap/login flows.
- Use Juju access semantics and persist them directly in SQLite.

Outcomes
- Users can create/list/use/delete namespaces.
- All controller-store resources are namespace-isolated.
- Membership and role checks are enforced server-side.
- CLI persists active namespace and sends it for namespace-scoped requests.
- Bootstrap/login/controller lifecycle works end-to-end with namespaces enabled.

Scope for This Plan
- SQL-backed namespace membership and authorization.
- Namespace header transport and middleware enforcement.
- Hardening of bootstrap, JWT login, and model/controller resolution behavior.
- OpenAPI and generated client updates for namespace-aware transport.

Non-goals for This Plan
- Full name-to-UUID API migration in one cut.
- Backward compatibility guarantees for unreleased, in-flight behavior.

Design Decisions
- Namespace identity: stable UUID id plus unique human-readable name.
- Active namespace: selected by CLI and sent with namespace-scoped requests.
- Membership model: owner, admin, member.
- Owner source of truth: namespaces.created_by, not namespace_members.
- Namespace creation is explicit, never implicit at login.
- JWT login identity must stay aligned with account identity expectations in
  Juju client login checks.

Phases

Phase 0: Contract and API shape
1. Define namespace endpoints and errors.
2. Define active namespace transport using X-RCS-Namespace header.
3. Define role semantics:
	- owner: full control, includes namespace delete and owner-protected actions.
	- admin: manage members except owner-protected actions.
	- member: normal namespace-scoped resource operations.
4. Mark namespace-agnostic endpoints explicitly:
	- auth and namespace management endpoints.
	- jwks endpoint.

Phase 1: Database schema and migration
1. Ensure namespaces and namespace_members tables exist and are indexed.
2. Ensure all resource tables are namespace-scoped:
	- controllers, models, accounts, credentials, bootstrap_config, cookie_jars,
	  and meta tables.
3. Add migration guardrails for existing sqlite files:
	- idempotent creation.
	- safe fallback for stale local DBs during development.
4. Add typed identity columns for forward migration (unreleased-safe):
	- controllers.controller_uuid.
	- models.model_uuid.
	- keep name keys in parallel for transitional compatibility.

Phase 2: Server middleware and helpers
1. Add namespace middleware that resolves namespace from header.
2. Enforce namespace membership in middleware for all resource routes.
3. Add helper methods:
	- resolve active namespace.
	- load namespace by name.
	- membership and role checks.
4. Standardize error behavior:
	- missing namespace header returns clear 400.
	- unauthorized namespace access returns 403.
	- missing entities return consistent 404 messages.

Phase 3: Namespace and membership behavior
1. Implement namespace create/list/delete.
2. Implement member add/list/remove with owner protection.
3. Enforce owner protection rules:
	- owner cannot be re-added as non-owner member.
	- owner cannot be removed via member removal path.
4. Keep ownership transfer out of scope unless explicitly added.

Phase 4: Resource scoping and identity hardening
1. Scope all resource queries and writes by namespace.
2. Keep current/previous controller and model meta namespace-local.
3. Harden controller uniqueness behavior:
	- unique by name per namespace.
	- unique by controller UUID per namespace.
4. Harden model identity behavior:
	- preserve qualifier compatibility during transition.
	- ensure lookups and current-model operations work with qualified names used
	  by Juju flows.
5. Stop relying on ad-hoc JSON shape for identity-critical lookups.

Phase 5: JWT login and account consistency
1. Ensure mint controller token endpoint is namespace-aware.
2. Ensure JWT claims are complete:
	- sub user identity.
	- audience includes controller UUID.
	- access includes controller-<uuid> permission.
3. Ensure account details and login identity remain compatible with Juju
	login checks.
4. Ensure all token mint and direct model request paths include namespace
	header where required.
5. Add targeted retry for transient startup unauthorized during bootstrap
	verification if needed.

Phase 6: CLI and session behavior
1. Session file includes namespace.
2. Add and stabilize namespace commands:
	- namespace create.
	- namespace list.
	- use.
	- context display.
3. Ensure login does not auto-create namespace.
4. Ensure request clients always propagate active namespace for
	namespace-scoped endpoints.

Phase 7: OpenAPI and generated client
1. Add namespace header parameter where required in openapi spec.
2. Add namespace endpoints and payload schemas.
3. Regenerate client.
4. Verify no generated path bypasses namespace header propagation.

Phase 8: Juju Access Model in SQLite
1. Model access data using Juju semantics:
	- user identity (including external users).
	- controller access levels.
	- model access levels.
	- optional inherited/default access behavior where required.
2. Add explicit SQL tables for access control rather than embedding authz in
	JSON fields or ad-hoc joins.
3. Implement server-side access checks against these tables for controller and
	model operations.
4. Keep namespace membership and Juju access checks consistent:
	- namespace membership gates visibility.
	- Juju access level gates operation permissions.

Phase 9: Testing and verification
1. Database tests:
	- namespace isolation.
	- uniqueness rules by namespace and UUID.
2. API tests:
	- missing header behavior.
	- cross-namespace denial.
	- owner protection.
3. Bootstrap/login tests:
	- fresh login plus explicit namespace creation then bootstrap.
	- jwt login works after bootstrap.
	- models/controllers/destroy-controller workflows succeed.
4. CLI tests:
	- session namespace persistence.
	- namespace command behavior.
	- request namespace propagation.

Milestone Delivery Slices
1. Slice A: namespace schema and middleware baseline.
2. Slice B: namespace CRUD and membership with owner protection.
3. Slice C: full resource scoping and meta isolation.
4. Slice D: bootstrap/login hardening (jwt plus account/model consistency).
5. Slice E: openapi and client generation alignment.
6. Slice F: typed UUID identity columns and transition support.
7. Slice G: Juju access model tables and permission enforcement in SQLite.

Open Questions to Resolve Before Finalizing Implementation
1. Should namespace names remain globally unique or become owner-scoped?
2. Should delete namespace hard-delete all data or support soft-delete?
3. Do we introduce ownership transfer now or keep owner immutable?
4. Do we expose UUID-first controller/model APIs now or in a follow-up slice?
5. What is the explicit policy for account user identity mapping during JWT
	login versus local cache qualification?
6. Which Juju access levels do we persist at controller and model scope, and
	how do we map them into RCS authorization checks?

Definition of Done
1. Namespace CRUD and membership checks work with owner protections.
2. Resource reads and writes are isolated by namespace.
3. Bootstrap, login, models, and destroy-controller flows work with
	namespaces enabled.
4. Session namespace selection and propagation are stable.
5. Typed UUID identity is available in storage for controller/model resources.
6. Tests cover isolation, authorization, and bootstrap/login regression paths.
