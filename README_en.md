# CTF platform

A CTFd-like platform in Go and Gin: setup wizard, users and teams, Jeopardy challenges, scoreboard, pages, notifications, and Attack-Defense.

The navbar switches the interface between Russian and English. The choice is stored in the browser (`localStorage`, key `ctf_lang`). Russian is the default.

Russian guide: [README_ru.md](README_ru.md).

## Run

```bash
cp .env.example .env
# set SECRET_KEY in .env
docker compose up --build
```

Open [http://localhost:8000](http://localhost:8000) and finish `/setup`.

Accounts, challenges, rounds, checks and steals are in Postgres 16. Attack-Defense flags are in Redis 7. Jeopardy flags stay in Postgres.

## Time

One timezone is shared by the whole event. Pick it during setup and under **Admin → Config** from the city list. The label follows the interface language: "Москва UTC+3" or "Moscow UTC+3". The offset follows the current daylight-saving rules.

Every shown date and time uses that zone and the form **DD.MM.YYYY HH:MM**. A `datetime-local` field is still edited as wall-clock time in the selected zone; the stored value is an absolute instant.

## No full reload

Saving a form, including a file upload, goes through `fetch`. The tab does not reload: the navbar, page body, flashes and form token are replaced. Ordinary links still navigate as usual.

## Attack-Defense

Turn the mode on in config: **Attack-Defense** or **Mixed**. Teams are required.

Deploy chain:

1. Upload a checker archive. The platform starts the checker only. It does not start the vulnerable service from the archive; teams run that themselves.
2. Create a service: name, port, and a checker chosen from the list of checkers that are already running. The checker path is not typed in.
3. Set each team's host.
4. The service is attached to every team host. The port comes from the service, so every team uses the same port on a different host.

One checker serves that service for every team. The flag generator issues a different flag to each team.

The service name, port and checker can be changed after the service exists. Attack, defense, lost-flag, SLA and downtime points live in the shared Attack-Defense settings, not on an individual service.

The team captain can set the host on `/attack`. The port is not edited there.

An empty host is downtime every round: the team earns no SLA and pays the downtime penalty.

### Points

Defaults:

| Event | Points |
| --- | --- |
| Attack: another team's flag was stolen | +10 to the attacker |
| Lost flag: an own flag was stolen | −10 to the victim |
| Defense: an own flag lived its whole lifetime on a working service and was not stolen | +5 |
| SLA: a round with status OK | +2 |
| Downtime: DOWN, MUMBLE, CORRUPT, or no host | −20 × number of opponents |

A dead service costs more than being robbed by every opponent. ERROR means the platform could not reach its own checker and does not change the team's score.

### Preparation

**Preparation starts** and **Preparation (minutes)** keep the checkers off and the score still until the window ends. A zero duration disables the window. Clear the engine pause so rounds begin when preparation ends.

### Checker

The checker answers HTTP `POST` with JSON:

```json
{"action":"put","team_id":1,"host":"10.0.1.10","port":8080,"flag":"FLAG{...}","round":3,"service":"web"}
```

`action` is `put`, `get` or `check`. The answer is `{"status":"OK|DOWN|CORRUPT|MUMBLE","message":"..."}`.

Each round the platform plants a new flag and reads it back. The platform does not run `jury/spawn.py`.

### Submitting a flag

Use `/attack`, or a request with the team token. That submit does not check CSRF: the page session or the `X-Team-Token` header already identifies the team.

```bash
curl -s -X POST http://CTF_HOST:8000/api/v1/ad/flags \
  -H 'X-Team-Token: TEAM_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '["FLAG{...}"]'
```

An own flag returns `own_flag` and the text `Cannot submit own flag`. During preparation the submit returns 403.

`/dashboard` is public unless score visibility is hidden or limited to admins.

## Test stand

`scripts/ad_bootstrap.py` creates an admin and the teams alpha, bravo and gamma. When a task directory and a checker zip are set, the script starts the service on `127.0.0.1`, `127.0.0.2` and `127.0.0.3` at one port, uploads the checker, creates the service and saves the hosts.

```bash
python3 scripts/ad_bootstrap.py
```

Defaults: task `/home/kali/my_AD_examples/AD1/bucket`, archive `bucket.zip`, port `18101`. Variables: `BASE`, `PREP_MINUTES`, `ADMIN_NAME`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `TASK_DIR`, `CHECKER_ZIP`, `BASE_PORT`, `CHECKER_HOST`, `CTF_TIMEZONE`.

The script prints the credentials at the end. It does not guess the password of an admin that already exists.
