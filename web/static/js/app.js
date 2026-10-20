window.ctfConfirm = function (message) {
  return new Promise(function (resolve) {
    const dlg = document.getElementById("ctf-dialog");
    const text = document.getElementById("ctf-dialog-text");
    const ok = document.getElementById("ctf-dialog-ok");
    const cancel = document.getElementById("ctf-dialog-cancel");
    if (!dlg || !text || !ok || !cancel || typeof dlg.showModal !== "function") {
      resolve(false);
      return;
    }
    text.textContent = message;
    let settled = false;
    function finish(value) {
      if (settled) return;
      settled = true;
      ok.removeEventListener("click", onOk);
      cancel.removeEventListener("click", onCancel);
      dlg.removeEventListener("cancel", onEsc);
      dlg.removeEventListener("close", onClose);
      if (dlg.open) dlg.close();
      resolve(value);
    }
    function onOk() { finish(true); }
    function onCancel() { finish(false); }
    function onEsc(e) { e.preventDefault(); finish(false); }
    function onClose() { finish(false); }
    ok.addEventListener("click", onOk);
    cancel.addEventListener("click", onCancel);
    dlg.addEventListener("cancel", onEsc);
    dlg.addEventListener("close", onClose);
    dlg.showModal();
    ok.focus();
  });
};

document.addEventListener("submit", function (e) {
  const form = e.target;
  if (!form || !form.getAttribute) return;
  const key = form.getAttribute("data-confirm");
  if (!key || form.getAttribute("data-confirmed") === "1") return;
  e.preventDefault();
  e.stopPropagation();
  const msg = window.t ? window.t(key) : key;
  window.ctfConfirm(msg).then(function (ok) {
    if (!ok) return;
    form.setAttribute("data-confirmed", "1");
    if (typeof form.requestSubmit === "function") form.requestSubmit();
    else form.submit();
  });
}, true);

(function () {
  document.addEventListener("input", function (e) {
    if (!e.target || e.target.id !== "chal-search") return;
    const q = e.target.value.toLowerCase();
    document.querySelectorAll(".chal-card").forEach(function (el) {
      el.style.display = el.dataset.name.toLowerCase().includes(q) ? "" : "none";
    });
  });

  let currentId = null;
  function showModal() {
    const el = document.getElementById("chalModal");
    if (!el || !window.bootstrap) return;
    window.bootstrap.Modal.getOrCreateInstance(el).show();
  }

  document.addEventListener("click", function (e) {
    const btn = e.target.closest && e.target.closest(".challenge-button");
    if (!btn) return;
    openChallenge(btn.dataset.id);
  });

  async function openChallenge(id) {
    currentId = id;
    const res = await fetch("/api/v1/challenges/" + id, { headers: { Accept: "application/json" } });
    const json = await res.json();
    if (!json.success) return;
    const d = json.data;
    document.getElementById("chalName").textContent = d.name;
    document.getElementById("chalValue").textContent = d.value + " " + t("challenges.points");
    document.getElementById("chalSolves").textContent = d.solves + " " + t("challenges.solves");
    document.getElementById("chalFB").textContent = d.first_blood ? t("challenges.firstBlood") + ": " + d.first_blood : "";
    document.getElementById("chalAttempts").textContent = d.max_attempts ? (d.attempts + "/" + d.max_attempts + " " + t("challenges.attempts")) : "";
    document.getElementById("chalDesc").innerHTML = d.description || "";
    const conn = document.getElementById("chalConn");
    if (d.connection_info) {
      conn.textContent = d.connection_info;
      conn.classList.remove("d-none");
    } else {
      conn.classList.add("d-none");
    }
    const files = document.getElementById("chalFiles");
    files.innerHTML = "";
    (d.files || []).forEach(function (f) {
      const a = document.createElement("a");
      a.href = f.location;
      a.className = "btn btn-sm btn-outline-light me-2 mb-2";
      a.textContent = f.name;
      files.appendChild(a);
    });
    const hints = document.getElementById("chalHints");
    hints.innerHTML = "";
    (d.hints || []).forEach(function (h) {
      const wrap = document.createElement("div");
      wrap.className = "mb-2";
      if (h.unlocked) {
        wrap.innerHTML = "<div class='alert alert-secondary mb-0'><strong>" + escapeHtml(h.title || t("challenges.hint")) + "</strong><div>" + escapeHtml(h.content || "") + "</div></div>";
      } else {
        const b = document.createElement("button");
        b.className = "btn btn-sm btn-outline-warning";
        b.textContent = t("challenges.unlock") + " (" + h.cost + " " + t("challenges.pts") + ")";
        b.addEventListener("click", function () { unlockHint(h.id); });
        wrap.appendChild(b);
      }
      hints.appendChild(wrap);
    });
    document.getElementById("flagInput").value = "";
    document.getElementById("flagMsg").innerHTML = "";
    if (d.solved_by_me) {
      document.getElementById("flagMsg").innerHTML = "<div class='alert alert-success'>" + escapeHtml(t("challenges.solved")) + "</div>";
    }
    showModal();
  }

  document.addEventListener("submit", async function (e) {
    if (!e.target || e.target.id !== "flagForm") return;
    e.preventDefault();
    {
      const submission = document.getElementById("flagInput").value;
      const res = await fetch("/api/v1/challenges/" + currentId + "/attempt", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json", "CSRF-Token": window.CSRF || "" },
        body: JSON.stringify({ submission: submission })
      });
      const json = await res.json();
      const box = document.getElementById("flagMsg");
      if (!json.success) {
        box.innerHTML = "<div class='alert alert-danger'>" + escapeHtml((json.errors || ["Error"]).map(function (m) { return window.tr ? window.tr(m) : m; }).join(", ")) + "</div>";
        return;
      }
      const status = json.data && json.data.status;
      if (status === "correct") {
        box.innerHTML = "<div class='alert alert-success'>" + escapeHtml(t("challenges.correct")) + "</div>";
        const btn = document.querySelector('.challenge-button[data-id="' + currentId + '"]');
        if (btn) btn.classList.add("solved");
      } else if (status === "already_solved") {
        box.innerHTML = "<div class='alert alert-info'>" + escapeHtml(t("challenges.already")) + "</div>";
      } else {
        box.innerHTML = "<div class='alert alert-danger'>" + escapeHtml(t("challenges.incorrect")) + "</div>";
      }
    }
  });

  async function unlockHint(id) {
    if (!(await window.ctfConfirm(t("challenges.confirmHint")))) return;
    const res = await fetch("/api/v1/unlocks", {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json", "CSRF-Token": window.CSRF || "" },
      body: JSON.stringify({ target: id, type: "hints" })
    });
    const json = await res.json();
    if (json.success) openChallenge(currentId);
  }

  function t(key) {
    return window.t ? window.t(key) : key;
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c];
    });
  }

  function loadChart() {
    if (window.Chart) return Promise.resolve();
    return new Promise(function (resolve, reject) {
      const s = document.createElement("script");
      s.src = "/static/vendor/chart/chart.umd.min.js";
      s.onload = resolve;
      s.onerror = reject;
      document.head.appendChild(s);
    });
  }

  function bindChart() {
    const canvas = document.getElementById("scoregraph");
    if (!canvas || canvas.dataset.chart === "1") return;
    canvas.dataset.chart = "1";
    Promise.all([loadChart(), fetch("/api/v1/scoreboard/top/10").then(function (r) { return r.json(); })]).then(function (res) {
      const json = res[1];
      if (!json.success || !json.data) return;
      const datasets = Object.keys(json.data).map(function (name, i) {
        const hue = (i * 47) % 360;
        return {
          label: name,
          data: json.data[name].map(function (p) { return { x: new Date(p.date).getTime(), y: p.score }; }),
          borderColor: "hsl(" + hue + " 80% 60%)",
          tension: 0.2,
          pointRadius: 2
        };
      });
      new Chart(document.getElementById("scoregraph"), {
        type: "line",
        data: { datasets: datasets },
        options: {
          parsing: false,
          scales: {
            x: {
              type: "linear",
              ticks: {
                color: "#9aa8bd",
                callback: function (value) {
                  const d = new Date(value);
                  if (isNaN(d.getTime())) return value;
                  const zone = (document.querySelector('meta[name="ctf-timezone"]') || {}).content || "UTC";
                  let parts;
                  try {
                    parts = new Intl.DateTimeFormat("en-GB", {
                      timeZone: zone, day: "2-digit", month: "2-digit", year: "numeric",
                      hour: "2-digit", minute: "2-digit", hourCycle: "h23"
                    }).formatToParts(d);
                  } catch (err) {
                    parts = new Intl.DateTimeFormat("en-GB", {
                      day: "2-digit", month: "2-digit", year: "numeric",
                      hour: "2-digit", minute: "2-digit", hourCycle: "h23"
                    }).formatToParts(d);
                  }
                  const g = {};
                  parts.forEach(function (p) { g[p.type] = p.value; });
                  return g.day + "." + g.month + "." + g.year + " " + g.hour + ":" + g.minute;
                }
              },
              grid: { color: "#2a3b55" }
            },
            y: { ticks: { color: "#9aa8bd" }, grid: { color: "#2a3b55" } }
          },
          plugins: { legend: { labels: { color: "#e8eef8" } } }
        }
      });
    }).catch(function () {});
  }

  function bindType() {
    const typeSel = document.getElementById("chalType");
    if (!typeSel || typeSel.dataset.bound === "1") return;
    typeSel.dataset.bound = "1";
    const toggle = function () {
      document.querySelectorAll(".dynamic-only").forEach(function (el) {
        el.style.display = typeSel.value === "dynamic" ? "" : "none";
      });
    };
    typeSel.addEventListener("change", toggle);
    toggle();
  }

  document.addEventListener("click", function (e) {
    const btn = e.target.closest && e.target.closest("[data-copy]");
    if (!btn) return;
    const src = document.querySelector(btn.getAttribute("data-copy"));
    if (!src) return;
    const text = src.textContent;
    const done = function () {
      btn.classList.add("copied");
      setTimeout(function () { btn.classList.remove("copied"); }, 1200);
    };
    const fallback = function () {
      const area = document.createElement("textarea");
      area.value = text;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.opacity = "0";
      document.body.appendChild(area);
      area.select();
      let ok = false;
      try { ok = document.execCommand("copy"); } catch (err) { ok = false; }
      area.remove();
      if (ok) {
        done();
        return;
      }
      const range = document.createRange();
      range.selectNodeContents(src);
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
    };
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(done, fallback);
    } else {
      fallback();
    }
  });

  // Native file inputs show "Choose File" in the browser language, so draw our own label.
  function enhanceFiles(root) {
    (root || document).querySelectorAll('input[type="file"].form-control').forEach(function (input) {
    const wrap = document.createElement("label");
    wrap.className = "file-pick form-control";
    const btn = document.createElement("span");
    btn.className = "file-pick-btn";
    btn.setAttribute("data-i18n", "file.choose");
    btn.textContent = t("file.choose");
    const name = document.createElement("span");
    name.className = "file-pick-name";
    name.setAttribute("data-i18n", "file.none");
    name.textContent = t("file.none");
    input.parentNode.insertBefore(wrap, input);
    input.classList.remove("form-control");
    input.classList.add("file-pick-input");
    wrap.appendChild(input);
    wrap.appendChild(btn);
    wrap.appendChild(name);
    input.addEventListener("change", function () {
      const file = input.files && input.files[0];
      if (file) {
        name.removeAttribute("data-i18n");
        name.textContent = file.name;
      } else {
        name.setAttribute("data-i18n", "file.none");
        name.textContent = t("file.none");
      }
    });
    });
  }
  window.ctfPageInit = function () {
    bindType();
    bindChart();
    enhanceFiles(document);
  };
  window.ctfPageInit();
})();

(function () {
  function paint(kind, row) {
    if (!row || row.id == null) return;
    const id = String(row.id);
    const status = row.status || "—";
    document.querySelectorAll('.deploy-status[data-deploy-kind="' + kind + '"][data-deploy-id="' + id + '"]').forEach(function (badge) {
      if (row.status) {
        badge.setAttribute("data-i18n", "deploy." + row.status);
        badge.textContent = window.t ? window.t("deploy." + row.status) : status;
      } else {
        badge.removeAttribute("data-i18n");
        badge.textContent = status;
      }
      badge.className = "deploy-status badge-status" + (row.status ? " badge-" + row.status : "");
    });
    const next = row.log || "";
    document.querySelectorAll('.deploy-log[data-deploy-log="' + kind + '"][data-deploy-id="' + id + '"]').forEach(function (pre) {
      if (pre.textContent !== next) {
        const nearBottom = pre.scrollHeight - pre.scrollTop - pre.clientHeight < 48;
        pre.textContent = next;
        if (nearBottom) pre.scrollTop = pre.scrollHeight;
      }
      const details = pre.closest("details");
      if (details && (row.status === "building" || row.status === "queued")) details.open = true;
    });
  }
  function tick() {
    if (!document.querySelector("[data-deploy-poll]")) return;
    fetch("/admin/deploy/status?t=" + Date.now(), {
      cache: "no-store",
      credentials: "same-origin",
      headers: { Accept: "application/json" }
    }).then(function (r) {
      if (!r.ok) return null;
      return r.json();
    }).then(function (json) {
      if (!json || !json.success || !json.data) return;
      (json.data.challenges || []).forEach(function (row) { paint("challenge", row); });
      (json.data.services || []).forEach(function (row) { paint("service", row); });
      (json.data.checkers || []).forEach(function (row) { paint("checker", row); });
    }).catch(function () {});
  }
  tick();
  setInterval(tick, 1000);
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden) tick();
  });
})();

document.addEventListener("submit", function (e) {
  const form = e.target;
  if (!(form instanceof HTMLFormElement)) return;
  const method = (form.getAttribute("method") || "get").toLowerCase();
  if (method !== "post") return;
  if (form.id === "flagForm" || form.id === "adFlagForm") return;
  if (form.getAttribute("data-reload") === "1") return;
  e.preventDefault();
  if (form.getAttribute("data-busy") === "1") return;
  form.setAttribute("data-busy", "1");
  const action = form.getAttribute("action") || location.pathname;
  const button = e.submitter || form.querySelector("button[type='submit'], button:not([type])");
  if (button) button.disabled = true;
  fetch(action, {
    method: "POST",
    body: new FormData(form),
    credentials: "same-origin",
    redirect: "follow",
    headers: { Accept: "text/html", "X-Requested-With": "fetch" }
  }).then(function (res) {
    const type = res.headers.get("content-type") || "";
    if (type.indexOf("text/html") === -1) {
      location.href = res.url || action;
      return null;
    }
    return res.text().then(function (html) { return { html: html, url: res.url }; });
  }).then(function (pack) {
    if (!pack) return;
    const doc = new DOMParser().parseFromString(pack.html, "text/html");
    const csrf = pack.html.match(/window\.CSRF\s*=\s*"((?:\\.|[^"\\])*)"/);
    if (csrf) window.CSRF = JSON.parse('"' + csrf[1] + '"');
    const zone = doc.querySelector('meta[name="ctf-timezone"]');
    const curZone = document.querySelector('meta[name="ctf-timezone"]');
    if (zone && curZone) curZone.setAttribute("content", zone.getAttribute("content") || "UTC");
    function swap(sel) {
      const next = doc.querySelector(sel);
      const cur = document.querySelector(sel);
      if (next && cur) cur.replaceWith(document.importNode(next, true));
    }
    swap("nav.ctfd-navbar");
    document.querySelectorAll("body > .container").forEach(function (n) { n.remove(); });
    const anchor = document.querySelector(".admin-wrap, main, footer");
    doc.querySelectorAll("body > .container").forEach(function (n) {
      document.body.insertBefore(document.importNode(n, true), anchor);
    });
    if (doc.querySelector(".admin-wrap")) {
      if (!document.querySelector(".admin-wrap")) {
        const main = document.querySelector("main");
        if (main) main.replaceWith(document.importNode(doc.querySelector(".admin-wrap"), true));
      } else {
        swap(".admin-wrap");
      }
    } else if (doc.querySelector("main")) {
      const admin = document.querySelector(".admin-wrap");
      if (admin) admin.replaceWith(document.importNode(doc.querySelector("main"), true));
      else swap("main");
    }
    if (doc.title) document.title = doc.title;
    try {
      const next = new URL(pack.url || location.href, location.href);
      if (next.pathname + next.search !== location.pathname + location.search) {
        history.pushState({}, "", next.pathname + next.search);
      }
    } catch (err) {}
    if (window.__adDash) {
      window.__adDash.forEach(clearInterval);
      window.__adDash = null;
    }
    if (window.__adAttack) {
      window.__adAttack.forEach(clearInterval);
      window.__adAttack = null;
    }
    document.querySelectorAll("script[data-page-script]").forEach(function (s) { s.remove(); });
    const pageSrc = doc.querySelector("script[data-page-script]");
    if (pageSrc && pageSrc.getAttribute("src")) {
      const s = document.createElement("script");
      s.src = pageSrc.getAttribute("src").split("?")[0] + "?t=" + Date.now();
      s.setAttribute("data-page-script", "1");
      document.body.appendChild(s);
    }
    if (window.ctfApply) window.ctfApply();
    if (window.ctfPageInit) window.ctfPageInit();
  }).catch(function () {
    form.removeAttribute("data-busy");
    if (button) button.disabled = false;
    location.href = action;
  });
});
