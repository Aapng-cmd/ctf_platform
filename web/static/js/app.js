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
  const search = document.getElementById("chal-search");
  if (search) {
    search.addEventListener("input", function () {
      const q = this.value.toLowerCase();
      document.querySelectorAll(".chal-card").forEach(function (el) {
        el.style.display = el.dataset.name.toLowerCase().includes(q) ? "" : "none";
      });
    });
  }

  let currentId = null;
  const modalEl = document.getElementById("chalModal");
  const modal = modalEl && window.bootstrap ? new bootstrap.Modal(modalEl) : null;

  document.querySelectorAll(".challenge-button").forEach(function (btn) {
    btn.addEventListener("click", function () {
      openChallenge(btn.dataset.id);
    });
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
    modal.show();
  }

  const form = document.getElementById("flagForm");
  if (form) {
    form.addEventListener("submit", async function (e) {
      e.preventDefault();
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
    });
  }

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

  if (document.getElementById("scoregraph")) {
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
                  const lang = document.documentElement.lang || "ru";
                  try { return d.toLocaleString(lang, { timeZone: zone }); } catch (e) { return d.toLocaleString(); }
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

  const typeSel = document.getElementById("chalType");
  if (typeSel) {
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
  document.querySelectorAll('input[type="file"].form-control').forEach(function (input) {
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
})();

(function () {
  if (!document.querySelector("[data-deploy-poll]")) return;
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
    }).catch(function () {});
  }
  tick();
  setInterval(tick, 1000);
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden) tick();
  });
})();
