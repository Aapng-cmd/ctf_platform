(function () {
  "use strict";
  if (window.__adAttack) {
    window.__adAttack.forEach(clearInterval);
    window.__adAttack = null;
  }
  if (window.__adAttackLang) {
    document.removeEventListener("ctf:lang", window.__adAttackLang);
    window.__adAttackLang = null;
  }
  const page = document.getElementById("attack-page");
  if (!page) return;
  const T = function (key) { return window.t ? window.t(key) : key; };
  const tr = function (msg) { return window.tr ? window.tr(msg) : msg; };
  const roundEl = document.getElementById("atkRound");
  const leftEl = document.getElementById("atkLeft");
  let state = { paused: false, during: true, preparing: false };
  let deadline = Date.now() + (parseInt(page.dataset.left, 10) || 0) * 1000;

  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function tickTimer() {
    if (state.preparing) {
      roundEl.textContent = "—";
      const left = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      leftEl.textContent = T("dash.prepLeft").replace("{n}", left);
      return;
    }
    if (state.paused) { leftEl.textContent = T("dash.paused"); return; }
    if (!state.during) { leftEl.textContent = T("dash.stopped"); return; }
    const left = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
    leftEl.textContent = T("attack.leftFmt").replace("{n}", left);
  }

  function paintStatus(el, status) {
    const kind = status === "OK" ? "up" : status ? "down" : "none";
    el.className = "st-badge " + kind;
    el.setAttribute("data-i18n", "st." + (status || "WAIT"));
    el.textContent = T("st." + (status || "WAIT"));
  }

  async function poll() {
    try {
      const res = await fetch("/api/v1/ad/dashboard?t=" + Date.now(), { cache: "no-store", credentials: "same-origin" });
      if (!res.ok) return;
      const body = await res.json();
      if (!body || !body.success) return;
      const d = body.data;
      state = { paused: d.paused, during: d.during, preparing: !!d.preparing };
      if (d.preparing) {
        roundEl.textContent = "—";
        deadline = Date.now() + (d.prep_seconds_left || 0) * 1000;
      } else if (d.round) {
        roundEl.textContent = d.round.number || "—";
        deadline = Date.now() + (d.round.seconds_left || 0) * 1000;
      }
      (d.teams || []).forEach(function (team) {
        (team.services || []).forEach(function (cell) {
          document.querySelectorAll('[data-st-team="' + team.account_id + '"][data-st-svc="' + cell.service_id + '"]').forEach(function (el) {
            paintStatus(el, cell.status);
          });
        });
      });
      tickTimer();
    } catch (e) {
      /* keep the last values on network errors */
    }
  }

  const form = document.getElementById("adFlagForm");
  const input = document.getElementById("adFlag");
  const box = document.getElementById("adFlagMsg");
  function show(kind, text) {
    box.innerHTML = "<div class='alert alert-" + kind + " mb-0'>" + esc(text) + "</div>";
  }
  form.addEventListener("submit", async function (e) {
    e.preventDefault();
    const submission = input.value.trim();
    if (!submission) return;
    try {
      const res = await fetch("/api/v1/ad/flags", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json", "CSRF-Token": window.CSRF || "" },
        body: JSON.stringify({ submission: submission })
      });
      const json = await res.json();
      if (!json.success) {
        show("danger", (json.errors || ["Error"]).map(tr).join(", "));
        return;
      }
      const d = json.data || {};
      if (d.status === "correct") {
        show("success", tr(d.message) + (d.points ? " (+" + d.points + ")" : ""));
        input.value = "";
        poll();
      } else if (d.status === "already_solved") {
        show("info", tr(d.message));
      } else {
        show("danger", tr(d.message || "Invalid flag"));
      }
    } catch (err) {
      show("danger", tr("Network error"));
    }
  });

  window.__adAttackLang = function () { tickTimer(); };
  document.addEventListener("ctf:lang", window.__adAttackLang);
  tickTimer();
  window.__adAttack = [setInterval(tickTimer, 250), setInterval(poll, 3000)];
  poll();
})();
