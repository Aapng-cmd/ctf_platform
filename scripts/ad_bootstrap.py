#!/usr/bin/env python3
"""Create an Attack-Defense admin, three teams, and a preparation window.

Also starts the bucket service once per team on its own host port and uploads
only the checker archive. The platform does not start the vulnerable container.

    python3 scripts/ad_bootstrap.py
    BASE=http://127.0.0.1:8000 PREP_MINUTES=10 python3 scripts/ad_bootstrap.py
    TASK_DIR=/path/to/bucket CHECKER_ZIP=/path/to/bucket.zip BASE_PORT=18101 python3 scripts/ad_bootstrap.py

CTF_TIMEZONE overrides the zone taken from /etc/localtime. CHECKER_HOST is the
address the checker dials; the checker container uses the host network, so
127.0.0.1 reaches the published ports.
"""

import html
import json
import os
import re
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone
from http.cookiejar import CookieJar

BASE = os.environ.get("BASE", "http://127.0.0.1:8000").rstrip("/")
PREP_MINUTES = os.environ.get("PREP_MINUTES", "10")
TASK_DIR = os.environ.get("TASK_DIR", "/home/kali/my_AD_examples/AD1/bucket")
CHECKER_ZIP = os.environ.get("CHECKER_ZIP", "/home/kali/my_AD_examples/AD1/bucket/bucket.zip")
BASE_PORT = int(os.environ.get("BASE_PORT", "18101"))
CHECKER_HOST = os.environ.get("CHECKER_HOST", "127.0.0.1")

ADMIN = {
    "name": os.environ.get("ADMIN_NAME", "adadmin"),
    "email": os.environ.get("ADMIN_EMAIL", "adadmin@ad.test"),
    "password": os.environ.get("ADMIN_PASSWORD", "Admintest1"),
}
TEAMS = [
    ("alpha", "alpha@ad.test", "Alpha-test1", "Alpha-team1"),
    ("bravo", "bravo@ad.test", "Bravo-test1", "Bravo-team1"),
    ("gamma", "gamma@ad.test", "Gamma-test1", "Gamma-team1"),
]


class Client:
    def __init__(self):
        self.jar = CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))

    def open(self, path, data=None, timeout=60):
        body = None if data is None else urllib.parse.urlencode(data).encode()
        return self.raw(path, body, timeout)

    def raw(self, path, body, timeout=60, content_type=None):
        req = urllib.request.Request(BASE + path, data=body)
        if content_type:
            req.add_header("Content-Type", content_type)
        try:
            res = self.opener.open(req, timeout=timeout)
        except urllib.error.HTTPError as err:
            res = err
        raw = res.read()
        return res.getcode(), res.geturl(), raw.decode("utf-8", "replace")

    def nonce(self, path):
        code, url, page = self.open(path)
        match = re.search(r'name="nonce" value="([^"]+)"', page)
        if not match:
            sys.exit("no CSRF nonce on %s (HTTP %s, landed on %s)" % (path, code, url))
        return match.group(1), page


def must(cond, message):
    if not cond:
        sys.exit(message)


def host_timezone():
    picked = os.environ.get("CTF_TIMEZONE", "").strip()
    if picked:
        return picked
    link = os.path.realpath("/etc/localtime")
    marker = "/zoneinfo/"
    if marker in link:
        return link.split(marker, 1)[1]
    return "UTC"


def setup(admin):
    guest = Client()
    _, url, page = guest.open("/setup")
    if "/setup" not in url or 'name="nonce"' not in page:
        print("setup already finished")
        return False
    nonce = re.search(r'name="nonce" value="([^"]+)"', page)
    must(nonce, "setup page has no form; is the site up?")
    _, final, body = guest.open("/setup", {
        "nonce": nonce.group(1),
        "ctf_name": "AD",
        "ctf_description": "Attack-Defense test",
        "game_mode": "attack_defense",
        "user_mode": "teams",
        "timezone": host_timezone(),
        "name": ADMIN["name"],
        "email": ADMIN["email"],
        "password": ADMIN["password"],
    })
    must("/setup" not in final, "setup failed, still on /setup:\n" + body[:500])
    print("admin created and logged in via setup")
    return True


def login_admin():
    admin = Client()
    nonce, _ = admin.nonce("/login")
    _, final, body = admin.open("/login", {
        "nonce": nonce,
        "name": ADMIN["name"],
        "password": ADMIN["password"],
    })
    must("/login" not in final, "admin login failed:\n" + body[:400])
    return admin


def configure(admin, start):
    nonce, _ = admin.nonce("/admin/ad")
    _, final, body = admin.open("/admin/ad/config", {
        "nonce": nonce,
        "ad_round_seconds": "60",
        "ad_flag_lifetime_rounds": "3",
        "ad_flag_prefix": "FLAG{",
        "ad_prep_start": start,
        "ad_prep_minutes": PREP_MINUTES,
    })
    must("/admin/ad" in final, "could not save AD settings:\n" + body[:400])
    print("preparation window saved, engine unpaused")


def input_value(page, name):
    match = re.search(r'name="%s" value="([^"]*)"' % name, page)
    return html.unescape(match.group(1)) if match else ""


def textarea_value(page, name):
    match = re.search(r'<textarea[^>]*name="%s"[^>]*>(.*?)</textarea>' % name, page, re.S)
    return html.unescape(match.group(1)) if match else ""


def selected_value(page, name):
    block = re.search(r'<select[^>]*name="%s"[^>]*>(.*?)</select>' % name, page, re.S)
    if not block:
        return ""
    match = re.search(r'<option[^>]*value="([^"]*)"[^>]*selected', block.group(1))
    return match.group(1) if match else ""


def save_timezone(admin, zone):
    nonce, page = admin.nonce("/admin/config")
    _, final, body = admin.open("/admin/config", {
        "nonce": nonce,
        "ctf_name": input_value(page, "ctf_name"),
        "ctf_description": textarea_value(page, "ctf_description"),
        "game_mode": selected_value(page, "game_mode"),
        "user_mode": selected_value(page, "user_mode"),
        "timezone": zone,
        "start": input_value(page, "start"),
        "end": input_value(page, "end"),
        "freeze": input_value(page, "freeze"),
        "challenge_visibility": selected_value(page, "challenge_visibility"),
        "score_visibility": selected_value(page, "score_visibility"),
        "account_visibility": selected_value(page, "account_visibility"),
        "registration_visibility": selected_value(page, "registration_visibility"),
        "max_team_size": input_value(page, "max_team_size"),
    })
    must("/admin/config" in final and "Unknown timezone" not in body, "could not save timezone:\n" + body[:400])
    print("timezone %s" % zone)


def create_team(user, email, user_password, team_password):
    player = Client()
    _, url, page = player.open("/register")
    if "/register" in url:
        nonce = re.search(r'name="nonce" value="([^"]+)"', page).group(1)
        _, final, _ = player.open("/register", {
            "nonce": nonce,
            "name": user,
            "email": email,
            "password": user_password,
        })
        if "/register" in final:
            player = Client()
            nonce, _ = player.nonce("/login")
            _, final, body = player.open("/login", {
                "nonce": nonce,
                "name": user,
                "password": user_password,
            })
            must("/login" not in final, "login %s failed:\n%s" % (user, body[:400]))
    nonce, _ = player.nonce("/teams/new")
    _, final, body = player.open("/teams/new", {
        "nonce": nonce,
        "name": user,
        "password": team_password,
    })
    must("/teams/" in final and "/teams/new" not in final, "team %s was not created:\n%s" % (user, body[:400]))
    print("team %s ready" % user)


def start_vulnbox(team, port):
    handle = tempfile.NamedTemporaryFile("w", suffix=".yml", delete=False)
    handle.write(
        "services:\n"
        "  bucket:\n"
        "    image: adtest-bucket\n"
        "    build: .\n"
        "    ports:\n"
        '      - "%d:80"\n'
        "    volumes:\n"
        "      - silo-data:/data\n"
        "    restart: unless-stopped\n"
        "volumes:\n"
        "  silo-data:\n" % port
    )
    handle.close()
    cmd = [
        "docker", "compose",
        "-p", "bucket-" + team,
        "--project-directory", TASK_DIR,
        "-f", handle.name,
        "up", "-d", "--build",
    ]
    print("starting bucket-%s on %s:%d" % (team, CHECKER_HOST, port))
    proc = subprocess.run(cmd, text=True)
    os.unlink(handle.name)
    must(proc.returncode == 0, "docker compose failed for %s" % team)


def upload_checker(admin):
    nonce, _ = admin.nonce("/admin/ad")
    boundary = "----adbootstrap7f3a1"
    filename = os.path.basename(CHECKER_ZIP)
    with open(CHECKER_ZIP, "rb") as fh:
        payload = fh.read()
    body = b"".join([
        ("--%s\r\nContent-Disposition: form-data; name=\"nonce\"\r\n\r\n%s\r\n" % (boundary, nonce)).encode(),
        ("--%s\r\nContent-Disposition: form-data; name=\"archive\"; filename=\"%s\"\r\nContent-Type: application/zip\r\n\r\n" % (boundary, filename)).encode(),
        payload,
        ("\r\n--%s--\r\n" % boundary).encode(),
    ])
    _, final, page = admin.raw(
        "/admin/ad/upload", body, timeout=180,
        content_type="multipart/form-data; boundary=" + boundary,
    )
    must("/admin/ad" in final, "checker upload failed:\n" + page[:500])
    print("checker archive uploaded")


def reset_checker_state():
    """Drop accounts remembered by a previous stand. A fresh vulnbox has none of them."""
    names = subprocess.check_output(["docker", "ps", "--format", "{{.Names}}"], text=True).split()
    checkers = [name for name in names if "_checker_" in name]
    must(checkers, "checker container is not running")
    for name in checkers:
        proc = subprocess.run(
            ["docker", "exec", name, "sh", "-c", "printf '%s\\n' '{\"teams\":{}}' > /var/lib/checker/bucket.json"],
            text=True,
        )
        must(proc.returncode == 0, "could not reset checker state in %s" % name)
        print("checker state reset in %s" % name)


def wait_checker(admin):
    last = ""
    for _ in range(180):
        _, url, page = admin.open("/admin/deploy/status")
        if "/login" in url:
            sys.exit("lost admin session while waiting for the checker")
        try:
            data = json.loads(page)["data"]["services"]
        except (json.JSONDecodeError, KeyError, TypeError):
            time.sleep(2)
            continue
        if not data:
            time.sleep(2)
            continue
        row = data[-1]
        last = row.get("status", "")
        if last == "running":
            print("checker is running")
            return
        if last == "failed":
            sys.exit("checker deploy failed:\n" + (row.get("log") or "")[-1500:])
        time.sleep(2)
    sys.exit("checker did not become running (last status %s)" % last)


def assign_hosts(admin):
    nonce, page = admin.nonce("/admin/ad")
    found = re.findall(
        r'<tr>\s*<td>([^<]+)</td>\s*<td>[^<]*</td>\s*<td><input[^>]*name="host_(\d+)"',
        page,
    )
    must(found, "no team host rows on the AD page")
    ports = {name: BASE_PORT + i for i, (name, *_) in enumerate(TEAMS)}
    form = {"nonce": nonce}
    used = []
    for team, inst in found:
        team = team.strip()
        must(team in ports, "no port assigned for team %s" % team)
        form["host_" + inst] = CHECKER_HOST
        form["port_" + inst] = str(ports[team])
        used.append((team, ports[team]))
    _, final, body = admin.open("/admin/ad/hosts", form)
    must("/admin/ad" in final, "could not save hosts:\n" + body[:400])
    for team, port in used:
        print("host %s -> %s:%d" % (team, CHECKER_HOST, port))


def main():
    fresh = setup(ADMIN)
    admin = login_admin()
    start = datetime.now(timezone.utc).replace(microsecond=0)
    if fresh:
        configure(admin, start.strftime("%Y-%m-%dT%H:%M:%S+00:00"))
    for user, email, user_password, team_password in TEAMS:
        create_team(user, email, user_password, team_password)
    save_timezone(admin, host_timezone())
    launched = False
    if os.path.isdir(TASK_DIR) and os.path.isfile(CHECKER_ZIP):
        for i, (team, *_) in enumerate(TEAMS):
            start_vulnbox(team, BASE_PORT + i)
        upload_checker(admin)
        wait_checker(admin)
        reset_checker_state()
        assign_hosts(admin)
        launched = True
    else:
        print("task dir or checker zip is missing, skipped service launch")
    end = start + timedelta(minutes=int(PREP_MINUTES))
    print()
    print("Admin")
    print("  url       %s/login" % BASE)
    print("  username  %s" % ADMIN["name"])
    print("  email     %s" % ADMIN["email"])
    print("  password  %s" % ADMIN["password"])
    print("Teams")
    for user, email, user_password, team_password in TEAMS:
        print("  %s" % user)
        print("    user      %s / %s" % (user, user_password))
        print("    email     %s" % email)
        print("    team      %s / %s" % (user, team_password))
    print("Timezone    %s" % host_timezone())
    if fresh:
        print("Preparation")
        print("  start     %s" % start.strftime("%Y-%m-%d %H:%M:%S UTC"))
        print("  minutes   %s" % PREP_MINUTES)
        print("  checkers  %s" % end.strftime("%Y-%m-%d %H:%M:%S UTC"))
    if launched:
        print("Bucket")
        print("  task      %s" % TASK_DIR)
        print("  checker   %s" % CHECKER_ZIP)
        for i, (team, *_) in enumerate(TEAMS):
            print("  %-8s  %s:%d" % (team, CHECKER_HOST, BASE_PORT + i))


if __name__ == "__main__":
    main()
