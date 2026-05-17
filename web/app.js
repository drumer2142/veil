(() => {
  const $ = (sel, el = document) => el.querySelector(sel);

  const els = {
    search: $("#search"),
    content: $("#content"),
    btnAdd: $("#btn-add"),
    btnExport: $("#btn-export"),
    btnImportToggle: $("#btn-import-toggle"),
    importPanel: $("#import-panel"),
    importFile: $("#import-file"),
    importMode: $("#import-mode"),
    importConfirmReplace: $("#import-confirm-replace"),
    importReplaceToken: $("#import-replace-token"),
    btnImportRun: $("#btn-import-run"),
    modal: $("#modal"),
    form: $("#bookmark-form"),
    modalTitle: $("#modal-title"),
    editId: $("#edit-id"),
    fName: $("#f-name"),
    fUrl: $("#f-url"),
    fCategory: $("#f-category"),
    fSort: $("#f-sort"),
    fIcon: $("#f-icon"),
    clearIconWrap: $("#clear-icon-wrap"),
    fClearIcon: $("#f-clear-icon"),
    categoryList: $("#category-list"),
    btnCancel: $("#btn-cancel"),
    btnCategories: $("#btn-categories"),
    categoryOrderModal: $("#category-order-modal"),
    categoryOrderList: $("#category-order-list"),
    btnCategoryOrderCancel: $("#btn-category-order-cancel"),
    btnCategoryOrderSave: $("#btn-category-order-save"),
    themeSelect: $("#theme-select"),
    logoImg: document.querySelector(".brand img"),
    extrasBackdrop: $("#extras-backdrop"),
    extrasDrawer: $("#extras-drawer"),
    extrasDrawerTitle: $("#extras-drawer-title"),
    extrasList: $("#extras-list"),
    extrasForm: $("#extras-form"),
    extrasUrl: $("#extras-url"),
    extrasClose: $("#extras-close"),
  };

  const THEME_KEY = "veil-theme";
  const LEGACY_THEME_KEY = "overseer-theme";
  const THEME_IDS = new Set(["dark", "light-orange", "rose", "ember"]);

  /** Avoid stale bookmark JSON from HTTP cache after icon updates. */
  const fetchNoStore = { cache: "no-store" };

  function getTheme() {
    try {
      let v = localStorage.getItem(THEME_KEY);
      if (!v) {
        const legacy = localStorage.getItem(LEGACY_THEME_KEY);
        if (legacy && THEME_IDS.has(legacy)) {
          v = legacy;
          try {
            localStorage.setItem(THEME_KEY, v);
          } catch (e2) {
            /* ignore */
          }
        }
      }
      if (THEME_IDS.has(v)) return v;
    } catch (e) {
      /* ignore */
    }
    return "dark";
  }

  function applyTheme(id) {
    const t = THEME_IDS.has(id) ? id : "dark";
    document.documentElement.setAttribute("data-theme", t);
    try {
      localStorage.setItem(THEME_KEY, t);
    } catch (e) {
      /* ignore */
    }
    if (els.themeSelect) els.themeSelect.value = t;
    if (els.logoImg) {
      if (t === "light-orange") els.logoImg.src = "/static/logo-light-orange.svg";
      else if (t === "rose") els.logoImg.src = "/static/logo-rose.svg";
      else if (t === "ember") els.logoImg.src = "/static/logo-ember.svg";
      else els.logoImg.src = "/static/logo.svg";
    }
  }

  /** @type {{ id:number; name:string; url:string; category:string; hasIcon:boolean; sortOrder:number; createdAt:string; extras?:{id:number;url:string}[] }[]} */
  let bookmarks = [];
  /** @type {typeof bookmarks[0]|null} */
  let extrasBookmark = null;
  /** @type {string[]} */
  let categories = [];
  /** @type {string[]} */
  let categoryOrder = [];

  const UNC = "Uncategorized";

  function displayCategory(raw) {
    const t = String(raw ?? "").trim();
    return t === "" ? UNC : t;
  }

  /**
   * @param {string[]} keySetArray
   * @param {string[]|null|undefined} savedOrder
   */
  function orderCategoryKeys(keySetArray, savedOrder) {
    const keySet = new Set(keySetArray);
    const out = [];
    const used = new Set();
    for (const k of savedOrder || []) {
      if (!keySet.has(k) || used.has(k)) continue;
      out.push(k);
      used.add(k);
    }
    const rest = [...keySet].filter((k) => !used.has(k));
    rest.sort((a, b) => {
      if (a === UNC) return 1;
      if (b === UNC) return -1;
      return a.localeCompare(b, undefined, { sensitivity: "base" });
    });
    return [...out, ...rest];
  }

  function groupBookmarks(list) {
    /** @type {Map<string, typeof list>} */
    const map = new Map();
    for (const b of list) {
      const key = displayCategory(b.category);
      if (!map.has(key)) map.set(key, []);
      map.get(key).push(b);
    }
    for (const arr of map.values()) {
      arr.sort((a, b) => {
        if (a.sortOrder !== b.sortOrder) return a.sortOrder - b.sortOrder;
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      });
    }
    return map;
  }

  function matchesSearch(b, q) {
    if (!q) return true;
    const extraHay = (b.extras ?? []).map((e) => e.url).join("\n");
    const hay = `${b.name}\n${b.url}\n${b.category}\n${extraHay}`.toLowerCase();
    return hay.includes(q);
  }

  function makeCategorySection(key, map) {
    const items = map.get(key) ?? [];
    const section = document.createElement("section");
    section.className = "category-block";
    const h2 = document.createElement("h2");
    h2.textContent = key;
    const grid = document.createElement("div");
    grid.className = "tile-grid";
    for (const b of items) {
      grid.appendChild(tileEl(b));
    }
    section.append(h2, grid);
    return section;
  }

  function render() {
    const q = els.search.value.trim().toLowerCase();
    const filtered = bookmarks.filter((b) => matchesSearch(b, q));
    const map = groupBookmarks(filtered);
    const keys = orderCategoryKeys([...map.keys()], categoryOrder);

    if (filtered.length === 0 && bookmarks.length > 0) {
      els.content.innerHTML =
        '<p class="empty-state">No bookmarks match your search.</p>';
      return;
    }
    if (bookmarks.length === 0) {
      els.content.innerHTML =
        '<p class="empty-state">No apps yet. Use <strong>Add app</strong> to create your first tile.</p>';
      return;
    }

    const visibleKeys = keys.filter((k) => (map.get(k) ?? []).length > 0);
    const layout = document.createElement("div");
    layout.className = "categories-layout";

    for (let i = 0; i < visibleKeys.length; i += 2) {
      const row = document.createElement("div");
      row.className = "category-row";
      const left = document.createElement("div");
      left.className = "category-col";
      left.appendChild(makeCategorySection(visibleKeys[i], map));
      if (i + 1 < visibleKeys.length) {
        const right = document.createElement("div");
        right.className = "category-col";
        right.appendChild(makeCategorySection(visibleKeys[i + 1], map));
        row.append(left, right);
      } else {
        row.classList.add("category-row--single");
        row.appendChild(left);
      }
      layout.appendChild(row);
    }

    els.content.replaceChildren(layout);
  }

  function tileEl(b) {
    const wrap = document.createElement("div");
    wrap.className = "tile";

    const link = document.createElement("a");
    link.href = b.url;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.style.textDecoration = "none";
    link.style.color = "inherit";
    link.style.display = "flex";
    link.style.flexDirection = "column";
    link.style.alignItems = "center";
    link.style.gap = "0.5rem";
    link.style.flex = "1";

    if (b.hasIcon) {
      const img = document.createElement("img");
      img.className = "tile-icon";
      img.alt = "";
      img.loading = "lazy";
      const iconRev = typeof b.iconRev === "number" ? b.iconRev : Number(b.iconRev) || 0;
      img.src = `/api/bookmarks/${b.id}/icon?v=${iconRev}`;
      link.appendChild(img);
    } else {
      const ph = document.createElement("div");
      ph.className = "tile-icon placeholder";
      ph.textContent = (b.name || "?").slice(0, 1).toUpperCase();
      link.appendChild(ph);
    }

    const title = document.createElement("div");
    title.className = "tile-name";
    title.textContent = b.name;
    link.appendChild(title);

    const actions = document.createElement("div");
    actions.className = "tile-actions";
    actions.onclick = (e) => e.preventDefault();

    const btnExtra = document.createElement("button");
    btnExtra.type = "button";
    btnExtra.textContent = "Add extra";
    btnExtra.addEventListener("click", () => openExtrasDrawer(b));

    const btnEdit = document.createElement("button");
    btnEdit.type = "button";
    btnEdit.textContent = "Edit";
    btnEdit.addEventListener("click", () => openEdit(b));

    const btnDel = document.createElement("button");
    btnDel.type = "button";
    btnDel.textContent = "Delete";
    btnDel.addEventListener("click", async () => {
      if (!confirm(`Delete “${b.name}”?`)) return;
      const res = await fetch(`/api/bookmarks/${b.id}`, { method: "DELETE" });
      if (!res.ok) return toast("Delete failed", true);
      await load();
      toast("Deleted");
    });

    actions.append(btnExtra, btnEdit, btnDel);
    wrap.append(link, actions);
    return wrap;
  }

  function findBookmark(id) {
    return bookmarks.find((b) => b.id === id) ?? null;
  }

  function patchBookmarkExtras(id, extras) {
    const b = findBookmark(id);
    if (b) b.extras = extras;
    if (extrasBookmark && extrasBookmark.id === id) {
      extrasBookmark = { ...extrasBookmark, extras };
    }
  }

  function renderExtrasList(extras) {
    els.extrasList.replaceChildren();
    if (!extras || extras.length === 0) {
      const p = document.createElement("p");
      p.className = "extras-empty";
      p.textContent = "No extra URLs yet.";
      els.extrasList.appendChild(p);
      return;
    }
    for (const e of extras) {
      const row = document.createElement("div");
      row.className = "extras-item";
      row.setAttribute("role", "listitem");

      const a = document.createElement("a");
      a.className = "extras-item-link";
      a.href = e.url;
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      a.textContent = e.url;

      const btnDel = document.createElement("button");
      btnDel.type = "button";
      btnDel.textContent = "Delete";
      btnDel.addEventListener("click", () => deleteExtra(e.id));

      row.append(a, btnDel);
      els.extrasList.appendChild(row);
    }
  }

  async function refreshExtrasForBookmark(id) {
    const res = await fetch(`/api/bookmarks/${id}/extras`, fetchNoStore);
    if (!res.ok) {
      toast("Failed to load extra URLs", true);
      return null;
    }
    const extras = await res.json();
    patchBookmarkExtras(id, extras);
    if (extrasBookmark && extrasBookmark.id === id) {
      renderExtrasList(extras);
    }
    return extras;
  }

  function openExtrasDrawer(b) {
    extrasBookmark = b;
    els.extrasDrawerTitle.textContent = `Extra URLs — ${b.name}`;
    els.extrasUrl.value = "";
    renderExtrasList(b.extras ?? []);
    els.extrasBackdrop.hidden = false;
    els.extrasBackdrop.setAttribute("aria-hidden", "false");
    els.extrasDrawer.classList.add("is-open");
    els.extrasDrawer.setAttribute("aria-hidden", "false");
    document.body.classList.add("extras-open");
    els.extrasUrl.focus();
    if (!b.extras) {
      refreshExtrasForBookmark(b.id).catch(() => toast("Failed to load extra URLs", true));
    }
  }

  function closeExtrasDrawer() {
    extrasBookmark = null;
    els.extrasBackdrop.hidden = true;
    els.extrasBackdrop.setAttribute("aria-hidden", "true");
    els.extrasDrawer.classList.remove("is-open");
    els.extrasDrawer.setAttribute("aria-hidden", "true");
    document.body.classList.remove("extras-open");
  }

  async function addExtra(url) {
    if (!extrasBookmark) return;
    const res = await fetch(`/api/bookmarks/${extrasBookmark.id}/extras`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url }),
    });
    if (!res.ok) {
      const t = await res.text();
      return toast(t || "Could not add URL", true);
    }
    const created = await res.json();
    const current = extrasBookmark.extras ?? [];
    const next = [...current, created];
    patchBookmarkExtras(extrasBookmark.id, next);
    renderExtrasList(next);
    els.extrasUrl.value = "";
    els.extrasUrl.focus();
    toast("URL added");
  }

  async function deleteExtra(extraId) {
    if (!extrasBookmark) return;
    const res = await fetch(`/api/bookmarks/${extrasBookmark.id}/extras/${extraId}`, {
      method: "DELETE",
    });
    if (!res.ok) {
      const t = await res.text();
      return toast(t || "Delete failed", true);
    }
    const next = (extrasBookmark.extras ?? []).filter((e) => e.id !== extraId);
    patchBookmarkExtras(extrasBookmark.id, next);
    renderExtrasList(next);
    toast("URL removed");
  }

  async function onExtrasSubmit(e) {
    e.preventDefault();
    const url = els.extrasUrl.value.trim();
    if (!url) return;
    await addExtra(url);
  }

  async function load() {
    const [bRes, cRes, oRes] = await Promise.all([
      fetch("/api/bookmarks", fetchNoStore),
      fetch("/api/categories", fetchNoStore),
      fetch("/api/category-order", fetchNoStore),
    ]);
    if (!bRes.ok) return toast("Failed to load bookmarks", true);
    bookmarks = await bRes.json();
    if (cRes.ok) categories = await cRes.json();
    if (oRes.ok) {
      try {
        const j = await oRes.json();
        categoryOrder = Array.isArray(j.order) ? j.order : [];
      } catch {
        categoryOrder = [];
      }
    } else {
      categoryOrder = [];
    }
    refreshCategoryDatalist();
    if (extrasBookmark) {
      const fresh = findBookmark(extrasBookmark.id);
      if (!fresh) closeExtrasDrawer();
      else {
        extrasBookmark = fresh;
        renderExtrasList(fresh.extras ?? []);
      }
    }
    render();
  }

  function uniqueCategoryKeysFromBookmarks() {
    const s = new Set();
    for (const b of bookmarks) {
      s.add(displayCategory(b.category));
    }
    return [...s];
  }

  function openCategoryOrderModal() {
    const keys = uniqueCategoryKeysFromBookmarks();
    if (keys.length === 0) {
      toast("No categories yet — add an app with a category first.", true);
      return;
    }
    const ordered = orderCategoryKeys(keys, categoryOrder);
    const ul = els.categoryOrderList;
    ul.replaceChildren();
    for (const k of ordered) {
      const li = document.createElement("li");
      li.className = "category-order-item";
      li.draggable = true;
      li.dataset.cat = k;
      const grip = document.createElement("span");
      grip.className = "co-grip";
      grip.setAttribute("aria-hidden", "true");
      grip.textContent = "⠿";
      const lab = document.createElement("span");
      lab.className = "co-label";
      lab.textContent = k;
      const act = document.createElement("span");
      act.className = "co-actions";
      const up = document.createElement("button");
      up.type = "button";
      up.className = "btn co-btn";
      up.dataset.act = "up";
      up.textContent = "Up";
      const down = document.createElement("button");
      down.type = "button";
      down.className = "btn co-btn";
      down.dataset.act = "down";
      down.textContent = "Down";
      act.append(up, down);
      li.append(grip, lab, act);
      ul.appendChild(li);
    }
    els.categoryOrderModal.showModal();
  }

  let coDragEl = null;

  function wireCategoryOrderListOnce() {
    const ul = els.categoryOrderList;
    if (ul.dataset.wired === "1") return;
    ul.dataset.wired = "1";
    ul.addEventListener("dragstart", (e) => {
      const li = e.target.closest(".category-order-item");
      if (!li) return;
      coDragEl = li;
      li.classList.add("dragging");
      try {
        e.dataTransfer.effectAllowed = "move";
        e.dataTransfer.setData("text/plain", li.dataset.cat || "");
      } catch (_) {
        /* ignore */
      }
    });
    ul.addEventListener("dragend", (e) => {
      const li = e.target.closest(".category-order-item");
      if (li) li.classList.remove("dragging");
      coDragEl = null;
    });
    ul.addEventListener("dragover", (e) => {
      e.preventDefault();
      try {
        e.dataTransfer.dropEffect = "move";
      } catch (_) {
        /* ignore */
      }
      if (!coDragEl) return;
      const li = e.target.closest(".category-order-item");
      if (!li || li === coDragEl) return;
      const rect = li.getBoundingClientRect();
      const before = e.clientY - rect.top < rect.height / 2;
      if (before) ul.insertBefore(coDragEl, li);
      else ul.insertBefore(coDragEl, li.nextElementSibling);
    });
    ul.addEventListener("click", (e) => {
      const btn = e.target.closest("button[data-act]");
      if (!btn) return;
      const row = btn.closest(".category-order-item");
      if (!row) return;
      if (btn.dataset.act === "up" && row.previousElementSibling) {
        ul.insertBefore(row, row.previousElementSibling);
      }
      if (btn.dataset.act === "down" && row.nextElementSibling) {
        ul.insertBefore(row.nextElementSibling, row);
      }
    });
  }

  async function saveCategoryOrder() {
    const order = [...els.categoryOrderList.querySelectorAll(".category-order-item")].map(
      (li) => li.dataset.cat || ""
    );
    const res = await fetch("/api/category-order", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ order }),
    });
    if (!res.ok) {
      const t = await res.text();
      return toast(t || "Could not save category order", true);
    }
    const j = await res.json();
    categoryOrder = Array.isArray(j.order) ? j.order : order;
    els.categoryOrderModal.close();
    render();
    toast("Category order saved");
  }

  function refreshCategoryDatalist() {
    els.categoryList.replaceChildren();
    const seen = new Set();
    for (const c of categories) {
      const t = String(c).trim();
      if (!t || seen.has(t)) continue;
      seen.add(t);
      const opt = document.createElement("option");
      opt.value = t;
      els.categoryList.appendChild(opt);
    }
    for (const b of bookmarks) {
      const t = String(b.category ?? "").trim();
      if (!t || seen.has(t)) continue;
      seen.add(t);
      const opt = document.createElement("option");
      opt.value = t;
      els.categoryList.appendChild(opt);
    }
  }

  function openAdd() {
    els.modalTitle.textContent = "Add app";
    els.editId.value = "";
    els.fName.value = "";
    els.fUrl.value = "";
    els.fCategory.value = "";
    els.fSort.value = "0";
    els.fIcon.value = "";
    els.clearIconWrap.style.display = "none";
    els.fClearIcon.value = "no";
    els.modal.showModal();
    els.fName.focus();
  }

  function openEdit(b) {
    els.modalTitle.textContent = "Edit app";
    els.editId.value = String(b.id);
    els.fName.value = b.name;
    els.fUrl.value = b.url;
    els.fCategory.value = b.category ?? "";
    els.fSort.value = String(b.sortOrder ?? 0);
    els.fIcon.value = "";
    els.clearIconWrap.style.display = "block";
    els.fClearIcon.value = "no";
    els.modal.showModal();
    els.fName.focus();
  }

  function closeModal() {
    els.modal.close();
  }

  async function onSubmit(e) {
    e.preventDefault();
    const id = els.editId.value.trim();
    const name = els.fName.value;
    const url = els.fUrl.value;
    const category = els.fCategory.value;
    const sortOrder = parseInt(els.fSort.value.trim() || "0", 10) || 0;
    const iconFile = els.fIcon.files?.[0] ?? null;

    if (id) {
      const fd = new FormData();
      fd.append("name", name);
      fd.append("url", url);
      fd.append("category", category);
      fd.append("sortOrder", String(sortOrder));
      if (iconFile) fd.append("icon", iconFile);
      if (els.fClearIcon.value === "yes") fd.append("clearIcon", "true");
      const res = await fetch(`/api/bookmarks/${id}`, { method: "PUT", body: fd });
      if (!res.ok) {
        const t = await res.text();
        return toast(t || "Save failed", true);
      }
      closeModal();
      await load();
      toast("Saved");
      return;
    }

    if (iconFile) {
      const fd = new FormData();
      fd.append("name", name);
      fd.append("url", url);
      fd.append("category", category);
      fd.append("sortOrder", String(sortOrder));
      fd.append("icon", iconFile);
      const res = await fetch("/api/bookmarks", { method: "POST", body: fd });
      if (!res.ok) {
        const t = await res.text();
        return toast(t || "Create failed", true);
      }
    } else {
      const res = await fetch("/api/bookmarks", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, url, category, sortOrder }),
      });
      if (!res.ok) {
        const t = await res.text();
        return toast(t || "Create failed", true);
      }
    }
    closeModal();
    await load();
    toast("Created");
  }

  async function onExport() {
    const res = await fetch("/api/export");
    if (!res.ok) return toast("Export failed", true);
    const blob = await res.blob();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "veil-export.json";
    a.click();
    URL.revokeObjectURL(a.href);
    toast("Export started");
  }

  async function onImportRun() {
    const mode = els.importMode.value;
    const file = els.importFile.files?.[0];
    if (!file) return toast("Choose a JSON file", true);
    if (mode === "replace") {
      if (!els.importConfirmReplace.checked) return toast("Confirm replace with the checkbox", true);
      if (els.importReplaceToken.value.trim() !== "REPLACE") {
        return toast('Type REPLACE in the confirmation field', true);
      }
    }
    const fd = new FormData();
    fd.append("file", file);
    const res = await fetch(`/api/import?mode=${encodeURIComponent(mode)}`, {
      method: "POST",
      body: fd,
    });
    if (!res.ok) {
      const t = await res.text();
      return toast(t || "Import failed", true);
    }
    const out = await res.json();
    await load();
    toast(`Import done: ${out.imported} added, ${out.skipped} skipped`);
    els.importFile.value = "";
  }

  let toastTimer = 0;
  function toast(msg, isErr = false) {
    clearTimeout(toastTimer);
    const t = document.createElement("div");
    t.className = "toast" + (isErr ? " error" : "");
    t.textContent = msg;
    document.body.appendChild(t);
    toastTimer = window.setTimeout(() => t.remove(), 4200);
  }

  els.search.addEventListener("input", () => render());
  els.btnAdd.addEventListener("click", openAdd);
  els.btnExport.addEventListener("click", onExport);
  els.btnImportToggle.addEventListener("click", () => {
    els.importPanel.hidden = !els.importPanel.hidden;
  });
  els.btnImportRun.addEventListener("click", onImportRun);
  els.btnCancel.addEventListener("click", closeModal);
  els.form.addEventListener("submit", onSubmit);
  wireCategoryOrderListOnce();
  els.btnCategories.addEventListener("click", openCategoryOrderModal);
  els.btnCategoryOrderCancel.addEventListener("click", () => els.categoryOrderModal.close());
  els.btnCategoryOrderSave.addEventListener("click", () => saveCategoryOrder().catch(() => toast("Save failed", true)));
  els.extrasClose.addEventListener("click", closeExtrasDrawer);
  els.extrasBackdrop.addEventListener("click", closeExtrasDrawer);
  els.extrasForm.addEventListener("submit", (e) => onExtrasSubmit(e).catch(() => toast("Add failed", true)));
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && extrasBookmark) closeExtrasDrawer();
  });

  const initialTheme = getTheme();
  applyTheme(initialTheme);
  if (els.themeSelect) {
    els.themeSelect.value = initialTheme;
    els.themeSelect.addEventListener("change", () => applyTheme(els.themeSelect.value));
  }

  window.addEventListener("pageshow", (ev) => {
    if (ev.persisted) {
      load().catch(() => toast("Failed to load", true));
    }
  });

  load().catch(() => toast("Failed to load", true));
})();
