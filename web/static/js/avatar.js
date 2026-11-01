(function () {
  "use strict";
  const input = document.getElementById("avatar-file");
  const dialog = document.getElementById("avatar-crop");
  if (!input || !dialog) return;
  const imgEl = dialog.querySelector("img");
  const box = dialog.querySelector(".avatar-box");
  const ok = dialog.querySelector("[data-crop-ok]");
  const cancel = dialog.querySelector("[data-crop-cancel]");
  let natW = 0;
  let natH = 0;
  let sel = { x: 0, y: 0, s: 0 };
  let drag = null;
  let accepted = false;

  function limits() {
    return { w: imgEl.clientWidth, h: imgEl.clientHeight };
  }

  function paint() {
    box.style.left = sel.x + "px";
    box.style.top = sel.y + "px";
    box.style.width = sel.s + "px";
    box.style.height = sel.s + "px";
  }

  function layout() {
    const view = limits();
    if (!view.w || !view.h) {
      requestAnimationFrame(layout);
      return;
    }
    const s = Math.min(view.w, view.h);
    sel = { x: (view.w - s) / 2, y: (view.h - s) / 2, s: s };
    paint();
  }

  input.addEventListener("change", function () {
    const file = input.files && input.files[0];
    if (!file) return;
    if (file.type !== "image/png" && file.type !== "image/jpeg") {
      input.value = "";
      return;
    }
    const url = URL.createObjectURL(file);
    imgEl.onload = function () {
      natW = imgEl.naturalWidth;
      natH = imgEl.naturalHeight;
      URL.revokeObjectURL(url);
      if (natW <= 1024 && natH <= 1024) return;
      accepted = false;
      dialog.showModal();
      layout();
    };
    imgEl.onerror = function () {
      URL.revokeObjectURL(url);
      input.value = "";
    };
    imgEl.src = url;
  });

  box.addEventListener("pointerdown", function (e) {
    const view = limits();
    drag = {
      x: e.clientX, y: e.clientY, ox: sel.x, oy: sel.y, os: sel.s,
      corner: e.target.classList.contains("avatar-handle"),
      w: view.w, h: view.h
    };
    box.setPointerCapture(e.pointerId);
  });
  box.addEventListener("pointermove", function (e) {
    if (!drag) return;
    const dx = e.clientX - drag.x;
    const dy = e.clientY - drag.y;
    if (drag.corner) {
      let s = Math.max(32, drag.os + Math.max(dx, dy));
      s = Math.min(s, drag.w - sel.x, drag.h - sel.y);
      sel.s = s;
    } else {
      sel.x = Math.min(Math.max(0, drag.ox + dx), drag.w - sel.s);
      sel.y = Math.min(Math.max(0, drag.oy + dy), drag.h - sel.s);
    }
    paint();
  });
  box.addEventListener("pointerup", function () { drag = null; });
  box.addEventListener("pointercancel", function () { drag = null; });

  cancel.addEventListener("click", function () {
    accepted = false;
    dialog.close();
  });
  dialog.addEventListener("close", function () {
    if (!accepted) input.value = "";
  });
  ok.addEventListener("click", function () {
    const view = limits();
    const scale = natW / view.w;
    const side = Math.max(1, Math.min(1024, Math.round(sel.s * scale)));
    const canvas = document.createElement("canvas");
    canvas.width = side;
    canvas.height = side;
    canvas.getContext("2d").drawImage(
      imgEl,
      sel.x * scale,
      sel.y * (natH / view.h),
      sel.s * scale,
      sel.s * (natH / view.h),
      0, 0, side, side
    );
    canvas.toBlob(function (blob) {
      if (!blob) return;
      const dt = new DataTransfer();
      dt.items.add(new File([blob], "avatar.png", { type: "image/png" }));
      input.files = dt.files;
      accepted = true;
      dialog.close();
    }, "image/png");
  });
})();
