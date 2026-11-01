(function () {
  "use strict";
  if (window.__adDash) {
    window.__adDash.forEach(clearInterval);
    window.__adDash = null;
  }
  if (window.__adPlay) {
    clearTimeout(window.__adPlay);
    window.__adPlay = 0;
  }
  if (window.__adPlayTick) {
    clearInterval(window.__adPlayTick);
    window.__adPlayTick = 0;
  }
  if (window.__adDashLang) {
    document.removeEventListener("ctf:lang", window.__adDashLang);
    window.__adDashLang = null;
  }
  const root = document.getElementById("ad-dashboard");
  if (!root) return;
  const boardEl = document.getElementById("adBoard");
  const numEl = document.getElementById("adRoundNum");
  const leftEl = document.getElementById("adRoundLeft");
  const barEl = document.getElementById("adRoundBar");
  const T = function (key) { return window.t ? window.t(key) : key; };
  const tr = function (msg) { return window.tr ? window.tr(msg) : msg; };

  let state = null;
  let deadline = 0;
  let frames = null;
  let coeffs = [];
  let frameIndex = 0;
  let playSpeed = 0;
  let playUntil = 0;
  let playTotal = 0;
  let boardRaw = null;
  const seen = {};
  const foldKey = "ctf-fold:" + location.pathname;
  let folded = {};
  try { folded = JSON.parse(sessionStorage.getItem(foldKey) || "{}"); } catch (e) { folded = {}; }
  function saveFold() {
    try { sessionStorage.setItem(foldKey, JSON.stringify(folded)); } catch (e) {}
  }
  if (boardEl && !boardEl.dataset.foldBound) {
    boardEl.dataset.foldBound = "1";
    boardEl.addEventListener("click", function (e) {
      if (e.target.closest("a")) return;
      const card = e.target.closest("[data-fold]");
      if (!card) return;
      const id = card.getAttribute("data-fold");
      if (folded[id]) delete folded[id];
      else folded[id] = 1;
      saveFold();
      render();
    });
    boardEl.addEventListener("keydown", function (e) {
      if (e.key !== "Enter" && e.key !== " ") return;
      const card = e.target.closest("[data-fold]");
      if (!card || e.target.closest("a")) return;
      e.preventDefault();
      card.click();
    });
  }

  try {
    boardRaw = document.getElementById("ad-dashboard-data");
    state = boardRaw ? JSON.parse(boardRaw.textContent || "null") : null;
  } catch (e) {
    state = null;
  }
  try {
    const packEl = document.getElementById("ad-archive-data");
    if (packEl) {
      const pack = JSON.parse(packEl.textContent || "{}");
      frames = Array.isArray(pack.frames) ? pack.frames : [];
      coeffs = Array.isArray(pack.coeffs) ? pack.coeffs : [];
      frameIndex = frames.length ? frames.length - 1 : 0;
      const want = Number(new URLSearchParams(location.search).get("round"));
      if (want) {
        frames.forEach(function (frame, i) {
          if (frame && frame.round && Number(frame.round.number) === want) frameIndex = i;
        });
      }
      if (frames.length) state = frames[frameIndex];
    }
  } catch (e) {
    frames = [];
  }

  function esc(value) {
    return String(value == null ? "" : value).replace(/[&<>"']/g, function (ch) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch];
    });
  }

  function kind(status) {
    if (status === "OK") return "up";
    if (!status) return "none";
    return "down";
  }

  function label(status) {
    return T("st." + (status || "WAIT"));
  }

  function num(n) {
    if (n > 0) return "+" + n;
    if (n < 0) return "−" + Math.abs(n);
    return "0";
  }

  function scoreText(team) {
    const score = Number(team.score) || 0;
    const extra = Number(team.adjust) || 0;
    const total = score + extra;
    if (!extra) return String(total);
    if (extra > 0) return total + " (+ " + extra + ")";
    return total + " (- " + Math.abs(extra) + ")";
  }

  function slaText(cell) {
    if (!cell.rounds) return "—";
    return Math.round((cell.ok_rounds * 100) / cell.rounds) + "%";
  }

  function stat(icon, value, key) {
    return '<span class="ad-stat" title="' + esc(T(key)) + '"><i class="bi ' + icon + '"></i>' + esc(value) + "</span>";
  }

  function cellHTML(team, cell) {
    const k = kind(cell.status);
    const id = team.account_id + ":" + cell.service_id;
    const changed = seen[id] !== undefined && seen[id] !== k;
    seen[id] = k;
    const tip = [label(cell.status)];
    if (cell.message) tip.push(tr(cell.message));
    let h = '<td class="ad-col-svc"><div class="ad-cell ' + k + (changed ? " changed" : "") + '" title="' + esc(tip.join(" — ")) + '">';
    h += '<div class="ad-cell-top"><span class="ad-state">' + esc(label(cell.status)) + "</span>";
    h += '<span class="ad-cell-pts">' + esc(num(cell.score)) + "</span></div>";
    h += '<div class="ad-cell-stats">';
    h += stat("bi-flag-fill", cell.captured, "dash.captured");
    h += stat("bi-shield-check", cell.defended, "dash.defended");
    h += stat("bi-shield-x", cell.lost, "dash.lostFlags");
    h += stat("bi-activity", slaText(cell), "dash.slaRatio");
    h += "</div>";
    h += '<div class="ad-history">';
    (cell.history || []).forEach(function (st) {
      h += '<i class="' + (st ? kind(st) : "empty") + '"></i>';
    });
    h += "</div></div></td>";
    return h;
  }

  function breakdown(team) {
    return [
      T("score.attack") + " " + num(team.attack),
      T("score.defense") + " " + num(team.defense),
      "SLA " + num(team.sla),
      T("score.lost") + " " + num(-team.lost),
      T("score.down") + " " + num(-team.down)
    ].join(" · ");
  }

  function render() {
    if (!state || !boardEl) return;
    if (state.preparing) {
      boardEl.innerHTML = '<div class="panel p-4">' + esc(T("dash.prep")) + "</div>";
      return;
    }
    const services = state.services || [];
    const teams = state.teams || [];
    if (numEl) numEl.textContent = state.round && state.round.number ? state.round.number : "—";
    if (!teams.length) {
      boardEl.innerHTML = '<div class="panel p-4 text-muted">' + esc(T("dash.empty")) + "</div>";
      return;
    }
    if (!services.length) {
      boardEl.innerHTML = '<div class="panel p-4 text-muted">' + esc(T("dash.noServices")) + "</div>";
      return;
    }
    let h = '<table class="ad-grid" style="--svc:' + services.length + '"><thead><tr>';
    h += '<th class="ad-col-team">' + esc(T("score.team")) + "</th>";
    h += '<th class="ad-col-score">' + esc(T("score.score")) + "</th>";
    services.forEach(function (svc) {
      h += '<th class="ad-col-svc" title="' + esc(svc.name) + '">' + esc(svc.name) + "</th>";
    });
    h += "</tr></thead><tbody>";
    teams.forEach(function (team) {
      const mine = state.me && team.account_id === state.me;
      const closed = !!folded[String(team.account_id)];
      const foldLabel = T(closed ? "dash.unfold" : "dash.fold");
      h += '<tr class="' + (mine ? "is-mine" : "") + (closed ? " is-collapsed" : "") + '">';
      h += '<td class="ad-col-team"><div class="ad-team" data-fold="' + team.account_id + '" role="button" tabindex="0" aria-expanded="' + (closed ? "false" : "true") + '" aria-label="' + esc(foldLabel) + '">';
      h += '<span class="ad-place p' + team.pos + '">' + team.pos + "</span>";
      h += '<span class="ad-team-text">';
      h += '<a class="ad-team-name" href="/teams/' + team.account_id + '" title="' + esc(team.name) + '">' + esc(team.name) + "</a>";
      if (!state.archive) h += '<span class="ad-team-ip">' + esc(team.host || "—") + "</span>";
      h += "</span>";
      if (mine) h += '<span class="ad-you">' + esc(T("dash.you")) + "</span>";
      h += '<i class="bi bi-chevron-' + (closed ? "right" : "down") + ' ad-fold-icon" aria-hidden="true"></i>';
      h += "</div></td>";
      h += '<td class="ad-col-score"><div class="ad-total" title="' + esc(breakdown(team)) + '">' + esc(scoreText(team)) + "</div></td>";
      (team.services || []).forEach(function (cell) { h += cellHTML(team, cell); });
      h += "</tr>";
    });
    h += "</tbody></table>";
    boardEl.innerHTML = h;
  }

  function setDeadline() {
    if (state && state.preparing) {
      deadline = Date.now() + (state.prep_seconds_left || 0) * 1000;
      return;
    }
    const left = state && state.round ? state.round.seconds_left || 0 : 0;
    deadline = Date.now() + left * 1000;
  }

  function tickTimer() {
    if (!state || !numEl || !leftEl || !barEl) return;
    if (state.archive) {
      numEl.textContent = state.round && state.round.number ? state.round.number : "—";
      if (playSpeed && playUntil) {
        const leftMs = Math.max(0, playUntil - Date.now());
        const left = Math.ceil(leftMs / 1000);
        leftEl.textContent = playSpeed + "x · " + T("attack.leftFmt").replace("{n}", left);
        barEl.style.width = playTotal ? Math.min(100, ((playTotal - leftMs) / playTotal) * 100) + "%" : "0%";
      } else {
        leftEl.textContent = "";
        barEl.style.width = "0%";
      }
      return;
    }
    if (state.preparing) {
      numEl.textContent = "—";
      const left = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      leftEl.textContent = T("dash.prepLeft").replace("{n}", left);
      barEl.style.width = "0%";
      return;
    }
    const total = (state.round && state.round.round_seconds) || 0;
    if (state.paused || !state.during) {
      leftEl.textContent = state.paused ? T("dash.paused") : T("dash.stopped");
      barEl.style.width = "0%";
      return;
    }
    const left = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
    leftEl.textContent = T("attack.leftFmt").replace("{n}", left);
    barEl.style.width = total ? Math.min(100, ((total - left) / total) * 100) + "%" : "0%";
    // The 3s poll used to leave the clock on zero until the next interval.
    if (left === 0) {
      if (!zeroSince) zeroSince = Date.now();
      if (Date.now() - zeroSince < 8000) pollSoon();
    } else {
      zeroSince = 0;
    }
  }

  function roundSeconds(frame) {
    const n = frame && frame.round ? Number(frame.round.round_seconds) : 0;
    return n > 0 ? n : 15;
  }

  function sameCoeff(a, b) {
    return Number(a.attack) === Number(b.attack) && Number(a.defense) === Number(b.defense) && Number(a.lost) === Number(b.lost) && Number(a.sla) === Number(b.sla) && Number(a.down) === Number(b.down) && Number(a.round_seconds) === Number(b.round_seconds) && Number(a.flag_life) === Number(b.flag_life);
  }

  function derivedCoeffs() {
    const out = [];
    (frames || []).forEach(function (frame) {
      if (!frame || !frame.rewards) return;
      const row = {
        round: frame.round ? frame.round.number : 0,
        attack: frame.rewards.attack,
        defense: frame.rewards.defense,
        lost: frame.rewards.lost,
        sla: frame.rewards.sla,
        down: frame.rewards.down,
        round_seconds: frame.round ? frame.round.round_seconds : 0,
        flag_life: frame.flag_life || 0
      };
      const prev = out[out.length - 1];
      if (prev && sameCoeff(prev, row)) return;
      out.push(row);
    });
    return out;
  }

  function coeffList() {
    return coeffs && coeffs.length ? coeffs : derivedCoeffs();
  }

  function paintRounds() {
    const box = document.getElementById("ad-rounds");
    if (!box || !frames) return;
    if (!frames.length) {
      box.innerHTML = '<span class="text-muted">' + esc(T("archive.noRounds")) + "</span>";
      return;
    }
    let h = "";
    frames.forEach(function (frame, i) {
      const n = frame && frame.round ? frame.round.number : i + 1;
      const on = i === frameIndex;
      h += '<button type="button" class="btn btn-sm ' + (on ? "btn-primary" : "btn-outline-light") + '" data-frame="' + i + '">' + esc(n) + "</button>";
    });
    box.innerHTML = h;
  }

  function paintPlay() {
    document.querySelectorAll("#ad-play [data-speed]").forEach(function (btn) {
      const on = playSpeed && Number(btn.getAttribute("data-speed")) === playSpeed;
      btn.classList.toggle("btn-primary", !!on);
      btn.classList.toggle("btn-outline-light", !on);
    });
  }

  function paintCoeffs() {
    const box = document.getElementById("ad-coeffs");
    if (!box) return;
    const rows = coeffList();
    if (!rows.length) {
      box.innerHTML = "";
      return;
    }
    const current = state && state.round ? Number(state.round.number) || 0 : 0;
    let active = null;
    rows.forEach(function (row) {
      if (Number(row.round) <= current) active = row;
    });
    let h = '<div class="ad-coeffs-title">' + esc(T("archive.coeffs")) + "</div>";
    h += '<div class="table-responsive"><table class="table table-sm ad-coeff-table mb-0"><thead><tr>';
    h += "<th>" + esc(T("attack.round")) + "</th>";
    h += "<th>" + esc(T("score.attack")) + "</th>";
    h += "<th>" + esc(T("score.defense")) + "</th>";
    h += "<th>" + esc(T("score.lost")) + "</th>";
    h += "<th>SLA</th>";
    h += "<th>" + esc(T("score.down")) + "</th>";
    h += "<th>" + esc(T("admin.roundSec")) + "</th>";
    h += "<th>" + esc(T("admin.flagLife")) + "</th>";
    h += "</tr></thead><tbody>";
    rows.forEach(function (row) {
      const label = Number(row.round) > 0 ? String(row.round) : T("archive.fromStart");
      h += '<tr class="' + (row === active ? "is-on" : "") + '">';
      h += "<td>" + esc(label) + "</td>";
      h += "<td>" + esc(row.attack) + "</td>";
      h += "<td>" + esc(row.defense) + "</td>";
      h += "<td>" + esc(row.lost) + "</td>";
      h += "<td>" + esc(row.sla) + "</td>";
      h += "<td>" + esc(row.down) + "</td>";
      h += "<td>" + esc(row.round_seconds) + "</td>";
      h += "<td>" + esc(row.flag_life) + "</td>";
      h += "</tr>";
    });
    h += "</tbody></table></div>";
    box.innerHTML = h;
  }

  function showFrame(i) {
    if (!frames || !frames.length) return;
    if (i < 0) i = 0;
    if (i >= frames.length) i = frames.length - 1;
    frameIndex = i;
    state = frames[i];
    setDeadline();
    render();
    tickTimer();
    paintRounds();
    paintCoeffs();
    const n = state && state.round ? state.round.number : 0;
    if (n) history.replaceState(null, "", location.pathname + "?round=" + n);
  }

  function stopPlay() {
    playSpeed = 0;
    playUntil = 0;
    playTotal = 0;
    if (window.__adPlay) {
      clearTimeout(window.__adPlay);
      window.__adPlay = 0;
    }
    if (window.__adPlayTick) {
      clearInterval(window.__adPlayTick);
      window.__adPlayTick = 0;
    }
    paintPlay();
    tickTimer();
  }

  function startPlay(speed) {
    if (!frames || !frames.length || !speed) return;
    stopPlay();
    playSpeed = speed;
    paintPlay();
    window.__adPlayTick = setInterval(tickTimer, 200);
    function schedule() {
      if (!playSpeed || !root.isConnected) return;
      playTotal = Math.max(250, Math.round((roundSeconds(frames[frameIndex]) * 1000) / playSpeed));
      playUntil = Date.now() + playTotal;
      tickTimer();
      window.__adPlay = setTimeout(function () {
        if (!root.isConnected || !playSpeed) return;
        if (frameIndex >= frames.length - 1) {
          stopPlay();
          return;
        }
        showFrame(frameIndex + 1);
        schedule();
      }, playTotal);
    }
    schedule();
  }

  function bindArchive() {
    const rounds = document.getElementById("ad-rounds");
    if (rounds && !rounds.dataset.bound) {
      rounds.dataset.bound = "1";
      rounds.addEventListener("click", function (e) {
        const btn = e.target.closest("[data-frame]");
        if (!btn) return;
        stopPlay();
        showFrame(Number(btn.getAttribute("data-frame")));
      });
    }
    const play = document.getElementById("ad-play");
    if (play && !play.dataset.bound) {
      play.dataset.bound = "1";
      play.addEventListener("click", function (e) {
        const btn = e.target.closest("[data-speed]");
        if (!btn) return;
        const speed = Number(btn.getAttribute("data-speed"));
        if (!speed) stopPlay();
        else startPlay(speed);
      });
    }
  }

  let polling = false;
  let zeroSince = 0;
  let lastSoon = 0;
  function pollSoon() {
    const now = Date.now();
    if (now - lastSoon < 300) return;
    lastSoon = now;
    poll();
  }

  async function poll() {
    if (polling) return;
    polling = true;
    try {
      const res = await fetch("/api/v1/ad/dashboard?t=" + Date.now(), { cache: "no-store", credentials: "same-origin" });
      if (!res.ok) return;
      const body = await res.json();
      if (!body || !body.success) return;
      state = body.data;
      setDeadline();
      render();
      tickTimer();
    } catch (e) {
      /* keep the last board on network errors */
    } finally {
      polling = false;
    }
  }

  const live = !document.getElementById("ad-archive-data");
  try {
    setDeadline();
    render();
    tickTimer();
    if (!live) {
      bindArchive();
      paintRounds();
      paintCoeffs();
      paintPlay();
      const play = document.getElementById("ad-play");
      if (play) play.hidden = !(frames && frames.length);
    }
  } catch (e) {
    /* a bad first paint must not stop the clock */
  }
  if (live) {
    window.__adDash = [setInterval(tickTimer, 250), setInterval(poll, 3000)];
    poll();
  }
  window.__adDashLang = function () { render(); tickTimer(); paintCoeffs(); paintRounds(); };
  document.addEventListener("ctf:lang", window.__adDashLang);
})();
