# CTF Platform

A CTFd-like Capture The Flag platform rewritten in **Go + Gin**. It keeps the familiar CTFd event flow: setup wizard, users or teams, challenge board, scoreboard, notifications, pages, and an admin panel.

Swagger / OpenAPI UI is **closed to the public** and only answers from **localhost**.

## Features

- Setup wizard (event name, users/teams mode, start/end, first admin)
- Registration, login, profiles, settings
- Team create/join with password and optional size limit
- Challenges: categories, markdown, files, connection info, tags, prerequisites
- Static and regex flags, optional case-insensitive matching
- Dynamic scoring (linear / logarithmic decay)
- Unlockable hints that cost points
- Attempt limits and per-user rate limiting
- First blood, awards, hidden/banned accounts
- Scoreboard with freeze and top-10 graph
- Attack-Defense mode (rounds, SLA checks, stolen flags)
- Pages CMS and broadcast notifications
- Admin: statistics, challenges, users, teams, submissions, scoreboard, pages, awards, config, reset
- Visibility controls for challenges, scores, accounts, and registration
- REST API compatible in spirit with CTFd (`/api/v1/...`)
- Swagger UI at `/swagger/index.html` **only from 127.0.0.1 / ::1**

## Quick start (Docker)

```bash
cp .env.example .env
# set SECRET_KEY in .env
docker compose up --build
```

Open [http://localhost:8000](http://localhost:8000) and complete `/setup`.

## Local development

MySQL/MariaDB or SQLite:

```bash
# SQLite
export DB_DRIVER=sqlite
export SQLITE_PATH=data/ctfd.db
export SECRET_KEY=dev-secret
go run ./cmd/server
```

The process listens on `:8000` by default (`PORT`).

## Swagger

From the same machine:

[http://127.0.0.1:8000/swagger/index.html](http://127.0.0.1:8000/swagger/index.html)

Any non-loopback client receives `403`.

## Typical operator flow

1. Finish setup and sign in as admin
2. Upload Jeopardy or Attack-Defense archives (below), or create a challenge by hand
3. Optionally set start/end/freeze and visibility under **Config**
4. Players register, solve challenges, and appear on the scoreboard

The site UI has an **EN / RU** switch in the navbar. The choice is stored in the browser (`localStorage`, key `ctf_lang`).

## Jeopardy task package

**Admin → Challenges → Upload archive.** The file is a `.zip` (up to 512 MB). After a successful import the platform reads the metadata, and if the archive contains Compose it builds and starts the task itself. A task with Compose stays hidden until the deploy succeeds. Status (`queued`, `building`, `running`, `failed`, `stopped`) and the full Compose log are on the challenge list and on the challenge edit page (**Redeploy** / **Stop**).

Put either `task.yml` (also `task.yaml` or `ctf.yml`) or a `README.md` in the archive:

```yaml
name: Hello
category: Web
value: 100
description: |
  Find the flag.
flag: flag{hello}
files:
  - dist/hello.zip
hints:
  - content: look at the headers
    cost: 10
connection_info: ""
```

`README.md` uses headings: `# title`, then `## Category`, `## Description`, `## Cost` (or `Value`), `## Flag`, `## Files` (paths separated by `;`), `## Hints` (`text|cost` per line), `## Attribution`, `## Connection`.

If `docker-compose.yml` (or `compose.yml`) is present, the platform runs `docker compose up -d --build` in that directory. The connection shown to players is `PUBLIC_HOST` plus the first published host port of a non-checker service. `PUBLIC_HOST` defaults to `127.0.0.1`.

Deploy needs Docker on the host that runs the app. The Compose file mounts `/var/run/docker.sock` into the web container, and the image installs `docker.io` and `docker-compose`. Give this socket only to an admin you trust: it can start containers on the host.

Stopping or removing the platform container (`docker stop`, `docker compose stop`, `docker compose down`) first stops and removes every task stack the platform started. The web service waits up to 2 minutes for that teardown (`stop_grace_period`) before Docker kills it. Task rows in the admin panel are marked `stopped`. Start them again with **Redeploy**.

## Attack-Defense task package

Set **Game Mode** to `Attack-Defense` or `Mixed` (teams are required). Upload the zip under **Admin → Attack-Defense**. Name, port, and checker are taken from the archive, not typed in the form. A checker is required.

Example layout (the `chat` directory is the service name):

```text
chat/
  DEPLOY.md
  Dockerfile
  PLAYER_DEPLOY.md
  checker/
    Dockerfile
    checker.py
  docker-compose.yml
  entrypoint.sh
  player-compose.yml
  player.Dockerfile
  src/
    main.cpp
jury/
  README.md
  spawn.py
```

What the platform uses:

- **Name** — directory that contains `docker-compose.yml` (`chat`). A player zip with the compose file at the archive root uses the compose service name (`bucket`). `player-compose.yml` is ignored. A checker directory is optional: without it the service still deploys, and SLA falls back to a TCP connect.
- **Port** — first published host port of a service that is not named `checker` (`"7777:7777"` → `7777`).
- **Checker** — Compose service named `checker`, or a `checker/` directory with `checker.py` or `Dockerfile`.
- **Description** — first paragraph of `DEPLOY.md`.
- `jury/` is not executed. `jury/spawn.py` stays in the archive for the organizers; this platform does not run it.

Import starts `docker compose up -d --build` in the service directory. Build errors and later checker output are stored on the service row in the admin panel. **Redeploy** and **Stop** are on that row.

### Players' machines

Teams run the player stack from `PLAYER_DEPLOY.md` / `player-compose.yml` on their own VM. The captain sets the host (and port, if it differs) on `/attack`, or an admin sets it in the host grid. An empty host is **DOWN** (`team host is not set`): the team gets no SLA points and pays the downtime penalty every round.

### Flag sender and automatic checks

Each round the jury (this platform) does two things for every team and service:

1. **Put** a fresh flag onto the team's service.
2. **Get** it back. If get fails, it falls back to **check**.

For an uploaded Compose checker the command is:

```text
docker compose run --rm -T --no-deps --network host checker put HOST:PORT FLAG_ID FLAG 1
docker compose run --rm -T --no-deps --network host checker get HOST:PORT FLAG_ID FLAG 1
docker compose run --rm -T --no-deps --network host checker check HOST:PORT
```

`HOST:PORT` is the team's vulnbox. `FLAG_ID` is the numeric id of the flag for this round. The last argument `1` is the vuln number. `--network host` lets the checker reach team machines.

Exit codes (ForcAD):

| Code | Status |
| --- | --- |
| 0 or 101 | OK |
| 102 | CORRUPT |
| 103 | MUMBLE |
| 104 | DOWN |

A failed put is stored on the flag and fails the SLA check for that round (`flag put failed`).

Scoring per service (defaults on import, editable per service under **Admin → Attack-Defense**):

| Event | Points |
| --- | --- |
| Attack: a flag of another team was stolen | +10 to the attacker |
| Lost flag: own flag was stolen | −10 to the victim |
| Defense: own flag lived its whole lifetime, the checker saw it in its round, the service never failed while the flag was alive, and nobody stole it | +5 |
| SLA: round with status OK | +2 |
| Down: round with DOWN, MUMBLE, CORRUPT or no host | −20 × number of opponents |

A dead service therefore costs more than being robbed by every other team. `ERROR` (the platform cannot reach its own checker) is not the team's fault and is not scored.

A service added by hand, without an archive, can still use an HTTP checker. The jury `POST`s JSON `{"action":"put"|"check","team_id":1,"host":"...","port":1337,"flag":"FLAG{...}","round":3,"service":"web"}` and expects `{"status":"OK|DOWN|CORRUPT|MUMBLE","message":"..."}`. With no checker URL and no Compose, SLA is a TCP connect to `host:port`.

Unpause the engine under **Admin → Attack-Defense** so rounds start.

### Submitting flags

Players submit stolen flags on `/attack`, or from scripts with the team token shown on that page (the captain can issue a new one):

```bash
curl -s -X POST http://CTF_HOST:8000/api/v1/ad/flags \
  -H 'X-Team-Token: TEAM_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '["FLAG{...}", "FLAG{...}"]'
```

The body can be a JSON array, `{"flags": [...]}`, `{"submission": "..."}` or plain text with one flag per line (up to 500 flags per request, 120 requests per minute per team). Each flag gets its own status: `correct`, `already_solved`, `incorrect` or `own_flag` (`Cannot submit own flag`, no points). A single own flag returns HTTP 400.

The `/dashboard` page (team × service grid with live status) is public, including for visitors who are not logged in, unless score visibility is set to hidden or admins only.

## Database

If `docker compose` used to fail on MySQL, this stack now uses **MariaDB 11** with a delayed healthcheck. Remove an old broken volume with `docker compose down -v` before the first start.

## API (session cookie)

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/api/v1/challenges` | Visible challenges |
| GET | `/api/v1/challenges/:id` | Detail, hints, files |
| POST | `/api/v1/challenges/:id/attempt` | `{ "submission": "flag{...}" }` |
| GET | `/api/v1/challenges/:id/solves` | Solve list |
| GET | `/api/v1/scoreboard` | Standings |
| GET | `/api/v1/scoreboard/top/:count` | Graph points |
| GET | `/api/v1/users`, `/api/v1/users/:id`, `/api/v1/users/me` | Accounts |
| GET | `/api/v1/teams`, `/api/v1/teams/:id` | Teams |
| POST | `/api/v1/unlocks` | `{ "target": hint_id, "type": "hints" }` |
| GET | `/api/v1/notifications` | Notifications |
| GET | `/api/v1/configs` | Public settings |
| POST | `/api/v1/ad/flags` | Submit stolen Attack-Defense flags (session or `X-Team-Token`) |
| GET | `/api/v1/ad/dashboard` | Dashboard grid: rounds, statuses, points |
| GET | `/api/v1/ad/scoreboard` | AD standings |
| GET | `/api/v1/ad/status` | Current round |

## Layout

```
cmd/server          entrypoint
internal/           config, db, models, scoring, handlers
web/templates       CTFd-like theme
web/static          CSS / JS
docs                Swagger spec (localhost-only)
```

The previous PHP application is replaced by this service. Challenge hosting (docker-compose per task) is no longer started by the platform; put connection strings in **Connection Info** the same way CTFd does.
