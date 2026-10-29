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
- Attack-Defense mode (rounds, SLA checks, stolen flags). Round flags live in Redis
- Pages CMS and broadcast notifications
- Admin: statistics, challenges, users, teams, submissions, scoreboard, pages, awards, config, reset
- Visibility controls for challenges, scores, accounts, and registration
- REST API compatible in spirit with CTFd (`/api/v1/...`)
- Swagger UI at `/swagger/index.html` **only from 127.0.0.1 / ::1**

## Quick start (Docker)

```bash
cp .env.example .env
# set SECRET_KEY, POSTGRES_PASSWORD and REDIS_PASSWORD in .env
docker compose up --build
```

Docker serves HTTPS on the local name in `CERT_DOMAIN` (default `ctf.local`). Add the server IP to each machine's hosts file (`10.0.0.5  ctf.local`), then open [https://ctf.local](https://ctf.local) and complete `/setup`. Port 80 and any other Host redirect to that name. The certificate is self-signed; `docker compose cp nginx:/etc/nginx/certs/ca.crt ca.crt` copies it. Postgres and Redis have no published ports.

A process started with `go run` still listens on `http://127.0.0.1:8000`.

## Local development

Postgres and Redis, or SQLite for a database-only trial (Attack-Defense flags still need Redis):

```bash
export DB_DRIVER=postgres
export POSTGRES_HOST=127.0.0.1
export POSTGRES_USER=ctfd
export POSTGRES_PASSWORD=ctfd
export POSTGRES_DB=ctfd
export REDIS_ADDR=127.0.0.1:6379
export SECRET_KEY=dev-secret
go run ./cmd/server
```

SQLite replaces only Postgres (`DB_DRIVER=sqlite`, `SQLITE_PATH=data/ctfd.db`). Redis stays required.

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

## Attack-Defense

The operator guide is in [README_en.md](README_en.md) and [README_ru.md](README_ru.md).

Set **Game Mode** to `Attack-Defense` or `Mixed` (teams are required). The archive uploaded under **Admin → Attack-Defense** starts a checker only; the vulnerable service is not started from that zip. Then create a service with a name, a port and that running checker, and set one host per team. The service is placed on every host at the service port. One checker serves every team, and each team receives its own flags. Attack, defense, lost flag, SLA and downtime points are event-wide settings, not fields of a service. Name, port and checker can be changed after the service exists.

Clocks use the timezone from **Admin → Config** and are shown as `DD.MM.YYYY HH:MM`. Saving a form, including a file upload, updates the page without a reload.

Each round the platform asks the checker to put a fresh flag on every team's copy of the service and then to get it back. The checker is HTTP: `POST` JSON `{"action":"put"|"get"|"check","team_id":1,"host":"...","port":1337,"flag":"FLAG{...}","round":3,"service":"web"}` and answer `{"status":"OK|DOWN|CORRUPT|MUMBLE","message":"..."}`. `ERROR` means the platform could not reach its own checker and is not scored. A team with no host is DOWN every round.

Default points, edited in the Attack-Defense settings: attack +10, lost flag −10, defense +5, SLA +2 per OK round, downtime −20 multiplied by the number of opponents. A dead service costs more than being robbed by every other team.

**Preparation starts** plus **Preparation (minutes)** holds the checkers and the score until that moment. A zero duration disables the window. Unpause the engine so rounds begin when the window ends.

`scripts/ad_bootstrap.py` creates the admin and the teams alpha, bravo and gamma. When `TASK_DIR` and `CHECKER_ZIP` are set it starts the service on `127.0.0.1`, `127.0.0.2` and `127.0.0.3` at one port (`BASE_PORT`, default 18101), uploads the checker, creates the service and saves the hosts. Defaults: `/home/kali/my_AD_examples/AD1/bucket` and `bucket.zip`.

### Submitting flags

Players submit stolen flags on `/attack`, or from scripts with the team token shown on that page (the captain can issue a new one):

```bash
curl -s -X POST https://ctf.local/api/v1/ad/flags \
  -H 'X-Team-Token: TEAM_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '["FLAG{...}", "FLAG{...}"]'
```

The body can be a JSON array, `{"flags": [...]}`, `{"submission": "..."}` or plain text with one flag per line (up to 500 flags per request, 120 requests per minute per team). Each flag gets its own status: `correct`, `already_solved`, `incorrect` or `own_flag` (`Cannot submit own flag`, no points). A single own flag returns HTTP 400.

The `/dashboard` page (team × service grid with live status) is public, including for visitors who are not logged in, unless score visibility is set to hidden or admins only.

## Database

Accounts, challenges, rounds, checks and steals are in **Postgres 16**. Attack-Defense flags are in **Redis 7** (AOF, so they survive a restart): a submit looks the flag up by its string, and each round writes one flag per team and service. Jeopardy flags stay in Postgres with the challenge. An old MariaDB volume is not reused; `docker compose down` leaves it on disk, and the new database starts empty.

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
