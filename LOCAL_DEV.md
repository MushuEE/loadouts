# Running Loadouts locally

Two processes: the Go API and the Vite dev server. The short version is at the bottom if
you just want the commands.

Everything the browser needs is served from **one port, 5173**. The dev server proxies
backend traffic, so you never point a browser at 8080. This matters more than it sounds —
see [Why a single port](#why-a-single-port).

---

## Start it

```bash
# 1. API — in-memory store, auto-seeded, :8080
cd loadouts/backend
go run ./cmd/server

# 2. UI — :5173, proxies /api to the API
cd loadouts/frontend
npm install          # first time only
npm run dev
```

Open **http://localhost:5173**.

No environment variables are needed. The client requests `/api/v1/...` relative to
whatever origin served the page, and [`vite.config.ts`](loadouts/frontend/vite.config.ts)
forwards `/api`, `/healthz` and `/sandbox` to `localhost:8080`.

---

## From another machine

Say the code runs on a workstation named `elmo.c.googlers.com` and you are on a laptop.

**Use the hostname directly:**

```
http://elmo.c.googlers.com:5173
```

Only 5173 has to be reachable. The API rides through the proxy on the same port, so no
firewall exception for 8080 and no CORS.

Two things have to be true, both already configured:

- The hostname is in `server.allowedHosts`. Vite answers **403** for any host it does not
  recognise — DNS-rebinding protection, and an easy 20 minutes to lose, because the
  failure looks like a server problem rather than a config one. `.googlers.com` (leading
  dot = domain and all subdomains) is allowlisted.
- `server.host` is `true`, so Vite binds all interfaces rather than loopback.

**If the hostname is not reachable**, forward the port over SSH and use localhost:

```bash
ssh -N -L 5173:localhost:5173 elmo.c.googlers.com
# then: http://localhost:5173
```

Only one `-L` is needed now that everything is on 5173.

---

## Why a single port

> [!IMPORTANT]
> The API base URL must stay **relative**. An absolute URL resolves against *the viewer's*
> machine, not the server's — so `http://localhost:8080/api/v1` only ever works when the
> browser and the backend are on the same box.

That was a real bug: the UI would load from a remote host and then fail every request with
*"Cannot reach the Loadouts API at http://localhost:8080/api/v1"*. The app looked broken;
the backend was fine and listening.

There were three copies of the same mistake, worth knowing about because any new absolute
URL will reintroduce it:

| Where | Was | Now |
| --- | --- | --- |
| `BASE_URL` in [`client.ts`](loadouts/frontend/src/api/client.ts) | `http://localhost:8080/api/v1` | `/api/v1` |
| `health()`, which derives its URL from `BASE_URL` | absolute, by inheritance | relative |
| `SANDBOX_BASE`, the plugin iframe `src` | `http://localhost:8080/sandbox` | `/sandbox` for local dev |

The sandbox one is the nastiest: it fails *inside an iframe*, so there is no console error
on the page, just an embed that never appears.

> [!WARNING]
> `SANDBOX_BASE=/sandbox` is a **local-dev convenience only**. Plugin embeds are supposed
> to be served from a separate origin (the tests assert `https://sandbox.test/...`). It is
> not a hole here only because the iframe uses `sandbox="allow-scripts"` *without*
> `allow-same-origin`, which forces an opaque origin regardless. Do not ship it.

Override any of these with `VITE_API_URL` / `SANDBOX_BASE` if you really do want to point
the UI at a backend somewhere else.

---

## Signing in

There are no passwords. The client asserts an identity with the `X-Profile-ID` header, and
**the profile switcher — the round chips at the bottom of the left icon rail — is the login
screen.** Your choice persists in `localStorage`.

| Profile | Use it to see |
| --- | --- |
| `gearhead` | The admin view. `owner` of `ul-backpacking`; owns most seeded loadouts |
| `fitcheck` | A second Profile belonging to the *same User*. `owner` of `nyc-fashion` |
| `trailsponsor` | A plain `member`, and a sponsor. What a non-admin is denied |

Authority is **per community** — `member` / `admin` / `owner`. There is no site-wide admin.
Loadouts are a separate axis: owner-only, with no admin override.

The only admin-gated surface is the **community layer editor**, under Communities → a
community you administer. Good demo: set a `ul_score` on an item as `gearhead`, switch to
`trailsponsor`, and watch the editor vanish while the value stays visible.

> [!NOTE]
> The in-memory store **reseeds with fresh random IDs on every backend restart**, so
> anything you create is lost and your stored profile ID goes stale. That self-heals —
> `SessionContext` falls back to the first profile when the saved one is gone.

---

## Keeping the servers alive

Both processes die with the shell that started them. To detach:

```bash
setsid /path/to/binary > /tmp/log 2>&1 < /dev/null &
disown
```

To stop them, **find the PID and kill it**:

```bash
ss -ltnp | grep -E ':8080|:5173'
kill <pid>
```

> [!CAUTION]
> Do not use `pkill -f <pattern>` here. `-f` matches against full command lines, including
> the command line of the shell you are typing into — so `pkill -f ldsrv` inside a script
> that mentions `ldsrv` kills the script, usually before it reaches the thing you meant to
> kill. The symptom is bizarre: the command exits 0 having run only part of itself.

---

## Troubleshooting

| Symptom | Cause | Fix |
| --- | --- | --- |
| "Cannot reach the Loadouts API at `http://localhost:8080/...`" | Absolute base URL; the browser is looking on its own machine | Base URL should be `/api/v1`. Unset `VITE_API_URL` |
| Vite returns **403** | Host not in `allowedHosts` | Add it in `vite.config.ts` |
| `bind: address already in use` | An older instance is still listening | `ss -ltnp \| grep :8080` then `kill <pid>` |
| Blank page, `/api` calls 404 through the proxy | Vite started before the proxy config existed | Restart Vite; config changes to `server.proxy` need a restart |
| Plugin embed never renders | `SANDBOX_BASE` points somewhere the browser cannot reach | Set `SANDBOX_BASE=/sandbox` |
| Loadouts you created are gone | Backend restarted; store is in memory | Expected. Use Postgres to persist |

---

## Postgres instead of in-memory

```bash
cd loadouts
docker compose up -d
# apply backend/migrations/*.sql in order, then:
DATABASE_URL="postgres://..." SEED=1 go run ./cmd/server
```

---

## TL;DR

```bash
cd loadouts/backend  && go run ./cmd/server   &
cd loadouts/frontend && npm run dev           &
# → http://localhost:5173   (or http://<host>:5173 from elsewhere)
```
