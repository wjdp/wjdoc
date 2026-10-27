"use strict";

const $ = (id) => document.getElementById(id);

const state = {
  docs: [],
  vocabulary: { types: [], statuses: [], inactive: [], priorities: [] },
  current: null,
  editing: false,
};

const ACTIVE = "active";
const ALL = "all";
const NONE = "none";

async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: options.body ? { "Content-Type": "application/json" } : {},
  });
  const body = await response.json();
  if (!response.ok) {
    const error = new Error(body.error || response.statusText);
    error.status = response.status;
    throw error;
  }
  return body;
}

function element(tag, attributes = {}, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(attributes)) {
    if (key === "class") node.className = value;
    else node.setAttribute(key, value);
  }
  node.append(...children.filter((child) => child !== null && child !== undefined));
  return node;
}

function numberLabel(doc) {
  const match = doc.name.match(/^\d+/);
  return match ? match[0] : "—";
}

function isInactive(status) {
  return state.vocabulary.inactive.includes(status);
}

function relativeAge(iso) {
  if (!iso) return "";
  const seconds = (Date.now() - new Date(iso).getTime()) / 1000;
  const steps = [[60, "s", 1], [3600, "m", 60], [86400, "h", 3600], [86400 * 30, "d", 86400], [86400 * 365, "mo", 86400 * 30]];
  for (const [limit, unit, size] of steps) {
    if (seconds < limit) return `${Math.max(0, Math.floor(seconds / size))}${unit} ago`;
  }
  return `${Math.floor(seconds / (86400 * 365))}y ago`;
}

// Filters live in the query string so a filtered view can be bookmarked; the
// open document lives in the hash.
function filters() {
  const params = new URLSearchParams(location.search);
  return {
    status: params.get("status") || ACTIVE,
    type: params.get("type") || ALL,
    priority: params.get("priority") || ALL,
    tag: params.get("tag") || ALL,
  };
}

function setFilter(key, value, fallback) {
  const params = new URLSearchParams(location.search);
  if (value === fallback) params.delete(key);
  else params.set(key, value);
  const query = params.toString();
  history.replaceState(null, "", (query ? `?${query}` : location.pathname) + location.hash);
  renderList();
}

function matches(doc, { status, type, priority, tag }, needle) {
  if (status === ACTIVE && isInactive(doc.status)) return false;
  if (status === NONE && doc.status) return false;
  if (![ACTIVE, ALL, NONE].includes(status) && doc.status !== status) return false;
  if (type !== ALL && doc.type !== type) return false;
  if (priority === NONE && doc.priority) return false;
  if (![ALL, NONE].includes(priority) && doc.priority !== priority) return false;
  if (tag !== ALL && !doc.tags.includes(tag)) return false;
  if (!needle) return true;
  return `${numberLabel(doc)} ${doc.title} ${doc.name} ${doc.tags.join(" ")}`.toLowerCase().includes(needle);
}

function visibleDocs() {
  const needle = $("search").value.trim().toLowerCase();
  const current = filters();
  return state.docs.filter((doc) => matches(doc, current, needle));
}

function fillSelect(select, options, value) {
  select.replaceChildren(...options.map(([optionValue, label]) => element("option", { value: optionValue }, label)));
  select.value = value;
}

function renderFilters() {
  const seenStatuses = state.docs.map((doc) => doc.status).filter(Boolean);
  const statuses = [...new Set([...state.vocabulary.statuses, ...seenStatuses])];
  const seenTypes = state.docs.map((doc) => doc.type).filter(Boolean);
  const types = [...new Set([...state.vocabulary.types, ...seenTypes])];
  const current = filters();

  fillSelect($("status-filter"), [
    [ACTIVE, "Active"],
    [ALL, "All statuses"],
    ...statuses.map((status) => [status, status]),
    [NONE, "No status"],
  ], current.status);
  fillSelect($("type-filter"), [[ALL, "All types"], ...types.map((type) => [type, type])], current.type);

  const seenPriorities = state.docs.map((doc) => doc.priority).filter(Boolean);
  const priorities = [...new Set([...state.vocabulary.priorities, ...seenPriorities])];
  fillSelect($("priority-filter"), [
    [ALL, "All priorities"],
    ...priorities.map((priority) => [priority, priority]),
    [NONE, "No priority"],
  ], current.priority);

  const tags = [...new Set(state.docs.flatMap((doc) => doc.tags))].sort();
  fillSelect($("tag-filter"), [[ALL, "All tags"], ...tags.map((tag) => [tag, tag])], current.tag);
}

function renderList() {
  const visible = visibleDocs();
  const selected = state.current && state.current.name;

  $("list").replaceChildren(...visible.map((doc) => {
    const link = element("a", { href: `#${encodeURIComponent(doc.name)}`, class: isInactive(doc.status) ? "inactive" : "" },
      element("span", { class: "number" }, numberLabel(doc)),
      element("span", { class: "title", title: doc.title }, doc.title),
      element("span", { class: "tags" },
        doc.status ? element("span", { class: `status status-${doc.status}` }, doc.status) : null,
        doc.priority ? element("span", { class: `priority priority-${doc.priority}` }, doc.priority) : null,
        doc.type ? element("span", { class: `type type-${doc.type}` }, doc.type) : null,
        ...doc.tags.map((tag) => element("span", { class: "tag" }, tag))),
    );
    if (doc.name === selected) link.setAttribute("aria-current", "page");
    return element("li", {}, link);
  }));

  document.querySelector("#list [aria-current]")?.scrollIntoView({ block: "nearest" });

  const hidden = state.docs.length - visible.length;
  $("count").textContent = `${visible.length} of ${state.docs.length}` + (hidden ? ` · ${hidden} hidden by filters` : "");
}

async function loadList() {
  const body = await api("/api/docs");
  state.docs = body.docs;
  state.vocabulary = body.vocabulary;
  $("project").textContent = body.project;
  document.title = `${body.project} docs`;
  renderFilters();
  renderList();
}

function isDirty() {
  return state.editing && state.current && $("editor").value !== state.current.source;
}

function showMessage(text, bad = false) {
  $("message").textContent = text;
  $("message").classList.toggle("bad", bad);
}

function renderDocument() {
  const doc = state.current;
  $("empty").hidden = Boolean(doc);
  $("document").hidden = !doc;
  if (!doc) return;

  $("doc-path").textContent = doc.path;
  $("doc-type").textContent = doc.type || "no type";
  $("doc-type").className = `type type-${doc.type}`;

  const statuses = [...new Set([...state.vocabulary.statuses, doc.status].filter(Boolean))];
  fillSelect($("doc-status"), [["", "no status"], ...statuses.map((status) => [status, status])], doc.status);
  $("doc-status").className = `status status-${doc.status}`;

  const priorities = [...new Set([...state.vocabulary.priorities, doc.priority].filter(Boolean))];
  fillSelect($("doc-priority"), [["", "no priority"], ...priorities.map((priority) => [priority, priority])], doc.priority);
  $("doc-priority").className = `priority priority-${doc.priority}`;
  $("doc-tags").replaceChildren(...doc.tags.map((tag) => element("span", { class: "tag" }, tag)));

  const dates = [];
  if (doc.created) dates.push(`created ${relativeAge(doc.created)}`);
  if (doc.modified) dates.push(`modified ${relativeAge(doc.modified)}`);
  $("doc-dates").textContent = dates.join(" · ");

  $("doc-error").hidden = !doc.error;
  $("doc-error").textContent = doc.error || "";

  $("view").innerHTML = doc.html;
  fixRelativeImages($("view"));

  $("view").hidden = state.editing;
  $("editor").hidden = !state.editing;
  $("save").hidden = !state.editing;
  $("save").classList.toggle("primary", state.editing);
  $("toggle").textContent = state.editing ? "View" : "Edit";
  $("doc-status").disabled = state.editing;
  $("doc-priority").disabled = state.editing;
}

// Rendered markdown is relative to the document's own folder, and the server
// serves docs/ and its folders at /docs/.
function docsURL(href) {
  const folder = state.current ? state.current.path.replace(/[^/]*$/, "") : "docs/";
  return new URL(href, `${location.origin}/${folder}`);
}

function fixRelativeImages(container) {
  for (const image of container.querySelectorAll("img[src]")) {
    image.src = docsURL(image.getAttribute("src")).href;
  }
}

async function openDocument(name, { keepEditing = false } = {}) {
  if (state.current && state.current.name !== name && isDirty() && !confirm("Discard unsaved changes?")) {
    history.replaceState(null, "", `#${encodeURIComponent(state.current.name)}`);
    return;
  }
  try {
    state.current = await api(`/api/docs/${encodeURIComponent(name)}`);
  } catch (error) {
    state.current = null;
    renderDocument();
    $("empty").textContent = error.status === 404 ? `No document ${name}` : error.message;
    return;
  }
  if (!keepEditing) state.editing = false;
  $("editor").value = state.current.source;
  showMessage("");
  renderDocument();
  renderList();
  $("view").scrollTop = 0;
}

function route() {
  const name = decodeURIComponent(location.hash.slice(1));
  if (name) openDocument(name);
  else {
    state.current = null;
    renderDocument();
    renderList();
  }
}

function setEditing(editing) {
  if (!state.current) return;
  if (!editing && isDirty() && !confirm("Discard unsaved changes?")) return;
  if (!editing) $("editor").value = state.current.source;
  state.editing = editing;
  renderDocument();
  if (editing) $("editor").focus();
}

async function write(method, body) {
  try {
    state.current = await api(`/api/docs/${encodeURIComponent(state.current.name)}`, {
      method,
      body: JSON.stringify({ ...body, version: state.current.version }),
    });
  } catch (error) {
    if (error.status === 409 && confirm(`${error.message}. Reload it and lose your edits?`)) {
      await openDocument(state.current.name, { keepEditing: state.editing });
      return false;
    }
    showMessage(error.message, true);
    return false;
  }
  $("editor").value = state.current.source;
  renderDocument();
  loadList();
  return true;
}

async function save() {
  if (!state.current || !state.editing) return;
  if (await write("PUT", { source: $("editor").value })) showMessage("Saved");
}

async function setStatus(status) {
  if (await write("PATCH", { fields: { status } })) showMessage(status ? `Status ${status}` : "Status removed");
  else $("doc-status").value = state.current.status;
}

async function setPriority(priority) {
  if (await write("PATCH", { fields: { priority } })) showMessage(priority ? `Priority ${priority}` : "Priority removed");
  else $("doc-priority").value = state.current.priority;
}

function step(offset) {
  const visible = visibleDocs();
  if (!visible.length) return;
  const index = visible.findIndex((doc) => state.current && doc.name === state.current.name);
  const next = visible[Math.min(visible.length - 1, Math.max(0, index + offset))];
  location.hash = encodeURIComponent(next.name);
}

function onViewClick(event) {
  const link = event.target.closest("a[href]");
  if (!link) return;
  const href = link.getAttribute("href");

  if (href.startsWith("#")) {
    event.preventDefault();
    document.getElementById(decodeURIComponent(href.slice(1)))?.scrollIntoView();
    return;
  }

  const url = docsURL(href);
  if (url.origin !== location.origin) {
    link.target = "_blank";
    link.rel = "noopener";
    return;
  }
  const linkedPath = decodeURIComponent(url.pathname.replace(/^\//, ""));
  const linked = state.docs.find((doc) => doc.path === linkedPath);
  if (linked) {
    event.preventDefault();
    location.hash = encodeURIComponent(linked.name);
    if (url.hash) setTimeout(() => document.getElementById(decodeURIComponent(url.hash.slice(1)))?.scrollIntoView(), 50);
  }
}

function onKey(event) {
  const typing = ["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement.tagName);
  if ((event.ctrlKey || event.metaKey) && event.key === "s") {
    event.preventDefault();
    save();
  } else if (event.key === "Escape" && state.editing) {
    setEditing(false);
  } else if (event.key === "Escape" && document.activeElement === $("search")) {
    $("search").blur();
  } else if (typing || event.ctrlKey || event.metaKey || event.altKey) {
    return;
  } else if (event.key === "/") {
    event.preventDefault();
    $("search").focus();
  } else if (event.key === "e") {
    event.preventDefault();
    setEditing(true);
  } else if (event.key === "j" || event.key === "ArrowDown") {
    event.preventDefault();
    step(1);
  } else if (event.key === "k" || event.key === "ArrowUp") {
    event.preventDefault();
    step(-1);
  }
}

// Pick up changes made outside the browser, such as wj set or an editor,
// whenever the tab regains focus.
async function refresh() {
  await loadList();
  if (!state.current || isDirty()) return;
  const latest = await api(`/api/docs/${encodeURIComponent(state.current.name)}`).catch(() => null);
  if (latest && latest.version !== state.current.version) {
    state.current = latest;
    $("editor").value = latest.source;
    renderDocument();
  }
}

$("search").addEventListener("input", renderList);
$("search").addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    const first = visibleDocs()[0];
    if (first) location.hash = encodeURIComponent(first.name);
  }
});
$("status-filter").addEventListener("change", (event) => setFilter("status", event.target.value, ACTIVE));
$("type-filter").addEventListener("change", (event) => setFilter("type", event.target.value, ALL));
$("priority-filter").addEventListener("change", (event) => setFilter("priority", event.target.value, ALL));
$("tag-filter").addEventListener("change", (event) => setFilter("tag", event.target.value, ALL));
$("doc-status").addEventListener("change", (event) => setStatus(event.target.value));
$("doc-priority").addEventListener("change", (event) => setPriority(event.target.value));
$("toggle").addEventListener("click", () => setEditing(!state.editing));
$("save").addEventListener("click", save);
$("view").addEventListener("click", onViewClick);
$("editor").addEventListener("input", () => showMessage(isDirty() ? "Unsaved" : ""));
window.addEventListener("hashchange", route);
window.addEventListener("keydown", onKey);
window.addEventListener("focus", () => refresh().catch(() => {}));
window.addEventListener("beforeunload", (event) => {
  if (isDirty()) event.preventDefault();
});

loadList().then(route).catch((error) => {
  $("empty").textContent = error.message;
});
