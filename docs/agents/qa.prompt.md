# QA: RCS + Juju End-to-End Autonomous Workflow

This prompt drives an agent through the full RCS + Juju happy-path QA autonomously.
Run it from the root of the `remote-controller-store` repo.

---

## Step 1 — Install Juju (if Juju patch changed)

Run: `(cd /home/ubuntu/repos/juju && make install)`

Skip if the Juju patch was not modified this session.

---

## Step 2 — Verify compose services are healthy

Run: `(cd /home/ubuntu/repos/test/remote-controller-store && docker compose ps)`

- If `rcsd` or `rcs-keycloak` are not healthy, run:
  `(cd /home/ubuntu/repos/test/remote-controller-store && docker compose up -d --wait)`
- If services remain unhealthy after that, stop and diagnose logs:
  `docker compose logs rcsd` / `docker compose logs rcs-keycloak`

---

## Step 3 — Rebuild the RCS CLI

Run: `(cd /home/ubuntu/repos/test/remote-controller-store && make build link)`

This produces `build/rcs` and symlinks it as `./rcs` at the repo root.

---

## Step 4 — Establish a fresh RCS session

Run both in sequence:

```
/home/ubuntu/repos/test/remote-controller-store/rcs logout
/home/ubuntu/repos/test/remote-controller-store/rcs login http://localhost:8484
```

The login command prints a device-flow URL and code. If running autonomously, present
the URL to the user and wait for them to authenticate before continuing.

After login completes, verify with:

```
/home/ubuntu/repos/test/remote-controller-store/rcs whoami
```

Expected: JSON with `email`, `addr`, `expires_at`, and `namespace` fields.

---

## Step 5 — Ensure an active namespace is set

Run: `/home/ubuntu/repos/test/remote-controller-store/rcs ns list`

- If the output is empty, create and activate one:
  ```
  /home/ubuntu/repos/test/remote-controller-store/rcs ns create dev
  /home/ubuntu/repos/test/remote-controller-store/rcs use dev
  ```
- If namespaces already exist, pick one and run `rcs use <name>`.

Verify context: `/home/ubuntu/repos/test/remote-controller-store/rcs context`

---

## Step 6 — Check for an existing controller

Run: `juju controllers`

**If error `401 Unauthorized`**: session is expired — go back to Step 4.

**If error `X-RCS-Namespace header is required`**: no namespace active — go back to Step 5.

**If `No controllers registered`**: proceed to Step 7 to bootstrap.

**If controller `lol` is listed**: skip Step 7 and go to Step 8.

---

## Step 7 — Bootstrap controller `lol`

Resolve the LXD bridge IP and bootstrap:

```bash
juju bootstrap lxd lol --config login-token-refresh-url=http://$(lxc network list --format json | jq -r '.[] | select(.name=="lxdbr0") | .config["ipv4.address"] | split("/")[0]'):8484/.well-known/jwks.json --verbose --debug
```

Wait for bootstrap to complete. Expected final output: `Bootstrap complete, controller "lol" is now available`.

---

## Step 8 — List models

Run: `juju models`

Expected: a table containing at least the `controller` model.

---

## Step 9 — Add a model

Run: `juju update-credential localhost --controller lol; juju add-model a`

If `a` already exists, try the next letter (`b`, `c`, etc.).

Expected: `Added 'a' model on lxd/localhost with credential 'localhost' (ubuntu)`

---

## Done

All steps passed. The RCS namespace-aware server, CLI, and Juju integration are working end-to-end.
