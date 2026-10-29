(function () {
  "use strict";
  if (window.__adDash) {
    window.__adDash.forEach(clearInterval);
    window.__adDash = null;
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
  const seen = {};
  const foldKey = "ctf-fold:" + location.pathname;
  let folded = {};
  try { folded = JSON.parse(sessionStorage.getItem(foldKey) || "{}"); } catch (e) { folded = {}; }
  function saveFold() {
    try { sessionStorage.setItem(foldKey, JSON.stringify(folded)); } catch (e) {}
  }
  if (!boardEl.dataset.foldBound) {
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
    const raw = document.getElementById("ad-dashboard-data");
    state = raw ? JSON.parse(raw.textContent || "null") : null;
  } catch (e) {
    state = null;
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
    if (!state) return;
    if (state.preparing) {
      boardEl.innerHTML = '<div class="panel p-4">' + esc(T("dash.prep")) + "</div>";
      return;
    }
    const services = state.services || [];
    const teams = state.teams || [];
    numEl.textContent = state.round && state.round.number ? state.round.number : "—";
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
      h += '<span class="ad-team-ip">' + esc(team.host || "—") + "</span>";
      h += "</span>";
      if (mine) h += '<span class="ad-you">' + esc(T("dash.you")) + "</span>";
      h += '<i class="bi bi-chevron-' + (closed ? "right" : "down") + ' ad-fold-icon" aria-hidden="true"></i>';
      h += "</div></td>";
      h += '<td class="ad-col-score"><div class="ad-total" title="' + esc(breakdown(team)) + '">' + esc(num(team.score).replace("+", "")) + "</div></td>";
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
    if (!state) return;
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
  }

  async function poll() {
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
    }
  }

  setDeadline();
  render();
  tickTimer();
  window.__adDash = [setInterval(tickTimer, 250), setInterval(poll, 3000)];
  window.__adDashLang = function () { render(); tickTimer(); };
  document.addEventListener("ctf:lang", window.__adDashLang);
})();
