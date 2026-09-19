// Keyboard shortcuts, command palette, theme switching, and small interactions
// for the server-rendered pages. No framework; works across htmx swaps because
// every lookup happens at the moment a key is pressed.

const THEMES = [
  { name: "ledger", label: "Ledger", description: "Cool paper, pine ink. IBM Plex.", swatch: ["#f6f6f3", "#17201b", "#1e5a3e"] },
  { name: "console", label: "Console", description: "Blue-slate, amber signal. Schibsted Grotesk.", swatch: ["#141b23", "#e4e9ee", "#e3a73b"] },
  { name: "blueprint", label: "Blueprint", description: "Drafting paper, cobalt lines. Source Serif.", swatch: ["#edf1f6", "#0e2238", "#1f4fd8"] },
];
const THEME_KEY = "traicr:theme";

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

/** @typedef {{keys: string, label: string, group: string, run: (event?: KeyboardEvent) => void, allowInInput?: boolean, palette?: boolean, hidden?: boolean}} Shortcut */

/** @type {Shortcut[]} */
const registered = [];

/**
 * Registers shortcuts. Later registrations win when two bindings share keys,
 * so page scripts can override the defaults. Returns an unregister function.
 * @param {Shortcut[]} shortcuts
 */
export function registerShortcuts(shortcuts) {
  registered.push(...shortcuts);
  return () => {
    for (const shortcut of shortcuts) {
      const index = registered.indexOf(shortcut);
      if (index >= 0) registered.splice(index, 1);
    }
  };
}

function declarativeShortcuts() {
  return Array.from(document.querySelectorAll("[data-shortcut]")).map((element) => ({
    keys: element.dataset.shortcut,
    label: element.dataset.shortcutLabel || element.textContent.trim(),
    group: element.dataset.shortcutGroup || "Page",
    palette: element.dataset.shortcutPalette !== "false",
    run: () => {
      if (element.dataset.shortcutAction === "focus") {
        element.focus();
        if (element.select) element.select();
      } else {
        element.click();
      }
    },
  }));
}

function allShortcuts() {
  // Later entries win: page scripts override template bindings, which override the defaults.
  return [...globalShortcuts, ...declarativeShortcuts(), ...registered];
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

const namedKeys = new Set(["escape", "enter", "tab", "backspace", "delete", "arrowup", "arrowdown", "arrowleft", "arrowright", "home", "end", "pageup", "pagedown", " "]);

function tokenFor(event) {
  const key = event.key;
  if (key === "Shift" || key === "Control" || key === "Alt" || key === "Meta") return null;
  const parts = [];
  if (event.metaKey || event.ctrlKey) parts.push("mod");
  if (event.altKey) parts.push("alt");
  const lower = key.toLowerCase();
  if (namedKeys.has(lower)) {
    if (event.shiftKey) parts.push("shift");
    parts.push(lower === " " ? "space" : lower);
    return parts.join("+");
  }
  if (key.length === 1) {
    if (parts.length && event.shiftKey && /[a-z]/i.test(key)) parts.push("shift");
    parts.push(/[a-z]/i.test(key) ? lower : key);
    return parts.join("+");
  }
  return null;
}

const parseKeys = (keys) => keys.trim().toLowerCase().split(/\s+/);

export function isEditableTarget(target) {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  return target.isContentEditable || Boolean(target.closest('[contenteditable="true"]'));
}

function allowedWhileTyping(shortcut) {
  if (shortcut.allowInInput) return true;
  const steps = parseKeys(shortcut.keys);
  return steps.length === 1 && (steps[0].includes("mod+") || steps[0] === "escape");
}

let pending = [];
let pendingTimer = 0;

function resetPending() {
  pending = [];
  window.clearTimeout(pendingTimer);
  pendingTimer = 0;
}

function dispatch(event) {
  if (event.defaultPrevented || event.isComposing) return;
  const token = tokenFor(event);
  if (!token) return;
  const typing = isEditableTarget(event.target);
  const inOverlay = event.target instanceof HTMLElement && Boolean(event.target.closest("dialog[open], .dropdown:focus-within"));
  const attempt = [...pending, token];
  const shortcuts = allShortcuts();
  let exact;
  let longer = false;
  for (let index = shortcuts.length - 1; index >= 0; index--) {
    const shortcut = shortcuts[index];
    if (typing && !allowedWhileTyping(shortcut)) continue;
    if (inOverlay && !parseKeys(shortcut.keys)[0].includes("mod+")) continue;
    const steps = parseKeys(shortcut.keys);
    if (steps.length < attempt.length) continue;
    if (!attempt.every((step, position) => steps[position] === step)) continue;
    if (steps.length === attempt.length) exact ??= shortcut;
    else longer = true;
  }
  if (longer) {
    pending = attempt;
    window.clearTimeout(pendingTimer);
    pendingTimer = window.setTimeout(resetPending, 900);
    return;
  }
  resetPending();
  if (exact) {
    event.preventDefault();
    exact.run(event);
  }
}

const isApple = /Mac|iPhone|iPad/.test(navigator.platform);

/** Key caps for one binding: "mod+k" becomes [["⌘","K"]]. */
export function keyCaps(keys) {
  return parseKeys(keys).map((step) =>
    step.split("+").map((part) => {
      switch (part) {
        case "mod":
          return isApple ? "⌘" : "Ctrl";
        case "shift":
          return "⇧";
        case "alt":
          return isApple ? "⌥" : "Alt";
        case "escape":
          return "Esc";
        case "enter":
          return "↵";
        case "arrowup":
          return "↑";
        case "arrowdown":
          return "↓";
        case "arrowleft":
          return "←";
        case "arrowright":
          return "→";
        case "space":
          return "Space";
        default:
          return part.length === 1 ? part.toUpperCase() : part;
      }
    }),
  );
}

export function renderKeys(keys) {
  const group = document.createElement("span");
  group.className = "inline-flex items-center gap-1";
  keyCaps(keys).forEach((step, index) => {
    if (index > 0) {
      const then = document.createElement("span");
      then.className = "text-base-content/50 text-[10px]";
      then.textContent = "then";
      group.appendChild(then);
    }
    for (const cap of step) {
      const kbd = document.createElement("kbd");
      kbd.className = "kbd kbd-xs";
      kbd.textContent = cap;
      group.appendChild(kbd);
    }
  });
  return group;
}

// ---------------------------------------------------------------------------
// Theme
// ---------------------------------------------------------------------------

export function currentTheme() {
  const attribute = document.documentElement.getAttribute("data-theme");
  return THEMES.some((theme) => theme.name === attribute) ? attribute : "ledger";
}

export function applyTheme(name) {
  document.documentElement.setAttribute("data-theme", name);
  try {
    window.localStorage.setItem(THEME_KEY, name);
  } catch {
    // Private browsing may block storage; the theme still applies for this page.
  }
  syncThemeMenu();
}

export function cycleTheme() {
  const index = THEMES.findIndex((theme) => theme.name === currentTheme());
  applyTheme(THEMES[(index + 1) % THEMES.length].name);
  toast(`Theme: ${THEMES[(index + 1) % THEMES.length].label}`);
}

function swatch(colors) {
  const span = document.createElement("span");
  span.className = "border-base-300 inline-flex h-4 w-6 overflow-hidden rounded-[3px] border";
  for (const color of colors) {
    const part = document.createElement("span");
    part.className = "flex-1";
    part.style.background = color;
    span.appendChild(part);
  }
  return span;
}

function syncThemeMenu() {
  const theme = currentTheme();
  const info = THEMES.find((entry) => entry.name === theme);
  for (const label of document.querySelectorAll("[data-theme-label]")) label.textContent = info.label;
  for (const button of document.querySelectorAll("[data-set-theme]")) {
    const active = button.dataset.setTheme === theme;
    button.classList.toggle("menu-active", active);
    button.setAttribute("aria-current", active ? "true" : "false");
  }
  for (const holder of document.querySelectorAll("[data-swatch]")) {
    if (holder.childElementCount) continue;
    const entry = THEMES.find((candidate) => candidate.name === holder.dataset.swatch);
    if (entry) holder.replaceWith(swatch(entry.swatch));
  }
}

// ---------------------------------------------------------------------------
// Toast
// ---------------------------------------------------------------------------

let toastTimer = 0;

export function toast(message, tone = "") {
  const holder = document.getElementById("toast");
  if (!holder) return;
  holder.replaceChildren();
  const alert = document.createElement("div");
  alert.className = `alert alert-soft py-2 text-sm shadow-lg ${tone}`;
  alert.setAttribute("role", "status");
  alert.textContent = message;
  holder.appendChild(alert);
  holder.classList.remove("hidden");
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => holder.classList.add("hidden"), 1800);
}

// ---------------------------------------------------------------------------
// Command palette and help
// ---------------------------------------------------------------------------

const palette = document.getElementById("palette");
const paletteInput = document.getElementById("palette-input");
const paletteList = document.getElementById("palette-list");
const help = document.getElementById("shortcuts-help");
let paletteItems = [];
let paletteIndex = 0;

function paletteCommands() {
  const query = paletteInput.value.trim();
  const items = [];
  if (query) items.push({ group: "Search", label: `Search sessions for “${query}”`, run: () => window.location.assign(`/?q=${encodeURIComponent(query)}`) });
  const seen = new Set();
  for (const shortcut of allShortcuts()) {
    if (shortcut.palette === false || shortcut.hidden) continue;
    const key = `${shortcut.group}:${shortcut.label}`;
    if (seen.has(key)) continue;
    seen.add(key);
    items.push({ group: shortcut.group, label: shortcut.label, keys: shortcut.keys, run: shortcut.run });
  }
  for (const [label, keys, href] of [["Sessions", "g s", "/"], ["Imports", "g i", "/imports"], ["Machines", "g m", "/machines"]]) {
    if (!items.some((item) => item.group === "Go to" && item.keys === keys)) items.push({ group: "Go to", label, keys, run: () => window.location.assign(href) });
  }
  for (const theme of THEMES) {
    items.push({ group: "Theme", label: `${theme.label}: ${theme.description}`, swatch: theme.swatch, current: theme.name === currentTheme(), run: () => applyTheme(theme.name) });
  }
  items.push({ group: "Help", label: "Keyboard shortcuts", keys: "?", run: openHelp });
  // Page actions first, then navigation, theme, and help.
  const rank = (group) => ({ Search: 0, "Go to": 2, Theme: 3, Help: 4 })[group] ?? 1;
  items.sort((a, b) => rank(a.group) - rank(b.group));
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  return items.filter((item) => item.group === "Search" || words.every((word) => `${item.group} ${item.label}`.toLowerCase().includes(word)));
}

function renderPalette() {
  paletteItems = paletteCommands();
  paletteIndex = Math.min(paletteIndex, Math.max(0, paletteItems.length - 1));
  paletteList.replaceChildren();
  if (!paletteItems.length) {
    const empty = document.createElement("li");
    empty.className = "text-base-content/60 px-3 py-6 text-center text-sm";
    empty.textContent = "Nothing matches.";
    paletteList.appendChild(empty);
    return;
  }
  let lastGroup = null;
  paletteItems.forEach((item, index) => {
    if (item.group !== lastGroup) {
      const title = document.createElement("li");
      title.className = "menu-title text-[11px]";
      title.textContent = item.group;
      paletteList.appendChild(title);
      lastGroup = item.group;
    }
    const li = document.createElement("li");
    const button = document.createElement("button");
    button.type = "button";
    button.setAttribute("role", "option");
    button.setAttribute("aria-selected", String(index === paletteIndex));
    button.className = `flex items-center gap-3 ${index === paletteIndex ? "palette-active" : ""}`;
    if (item.swatch) button.appendChild(swatch(item.swatch));
    const label = document.createElement("span");
    label.className = "grow truncate";
    label.textContent = item.label;
    button.appendChild(label);
    if (item.current) {
      const current = document.createElement("span");
      current.className = "text-base-content/50 text-xs";
      current.textContent = "Current";
      button.appendChild(current);
    }
    if (item.keys) button.appendChild(renderKeys(item.keys));
    button.addEventListener("click", () => runPaletteItem(item));
    button.addEventListener("mousemove", () => {
      if (paletteIndex !== index) {
        paletteIndex = index;
        renderPalette();
      }
    });
    li.appendChild(button);
    paletteList.appendChild(li);
  });
  paletteList.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
}

function runPaletteItem(item) {
  palette.close();
  item.run();
}

export function openPalette() {
  if (!palette) return;
  paletteInput.value = "";
  paletteIndex = 0;
  renderPalette();
  palette.showModal();
  paletteInput.focus();
}

function openHelp() {
  if (!help) return;
  const groups = new Map();
  for (const shortcut of allShortcuts()) {
    if (shortcut.hidden) continue;
    const list = groups.get(shortcut.group) ?? [];
    if (!list.some((existing) => existing.keys === shortcut.keys)) list.push(shortcut);
    groups.set(shortcut.group, list);
  }
  const holder = document.getElementById("shortcuts-groups");
  holder.replaceChildren();
  for (const [group, list] of groups) {
    const section = document.createElement("section");
    const heading = document.createElement("h3");
    heading.className = "text-base-content/60 mb-2 text-xs font-medium";
    heading.textContent = group;
    section.appendChild(heading);
    const ul = document.createElement("ul");
    ul.className = "divide-base-300/70 divide-y";
    for (const shortcut of list) {
      const li = document.createElement("li");
      li.className = "flex items-center justify-between gap-4 py-1.5 text-sm";
      const label = document.createElement("span");
      label.textContent = shortcut.label;
      li.append(label, renderKeys(shortcut.keys));
      ul.appendChild(li);
    }
    section.appendChild(ul);
    holder.appendChild(section);
  }
  help.showModal();
}

if (palette) {
  // A closed dialog can keep focus on its input, which would mute single-key shortcuts.
  palette.addEventListener("close", () => paletteInput.blur());
  help?.addEventListener("close", () => document.activeElement?.blur());
  paletteInput.addEventListener("input", () => {
    paletteIndex = 0;
    renderPalette();
  });
  paletteInput.addEventListener("keydown", (event) => {
    if (event.key === "ArrowDown" || (event.key === "n" && event.ctrlKey)) {
      event.preventDefault();
      paletteIndex = Math.min(paletteItems.length - 1, paletteIndex + 1);
      renderPalette();
    } else if (event.key === "ArrowUp" || (event.key === "p" && event.ctrlKey)) {
      event.preventDefault();
      paletteIndex = Math.max(0, paletteIndex - 1);
      renderPalette();
    } else if (event.key === "Enter") {
      event.preventDefault();
      const item = paletteItems[paletteIndex];
      if (item) runPaletteItem(item);
    }
  });
}

// ---------------------------------------------------------------------------
// List navigation: j/k over [data-keynav] items, Enter opens, o opens details
// ---------------------------------------------------------------------------

function keynavItems() {
  const list = document.querySelector("[data-keynav]");
  if (!list) return { list: null, items: [] };
  return { list, items: Array.from(list.querySelectorAll(list.dataset.keynav)) };
}

function selectedIndex(items) {
  return items.findIndex((item) => item.hasAttribute("data-selected"));
}

function selectItem(items, index) {
  items.forEach((item, position) => {
    if (position === index) item.setAttribute("data-selected", "");
    else item.removeAttribute("data-selected");
  });
  items[index]?.scrollIntoView({ block: "nearest" });
}

function moveSelection(delta) {
  const { items } = keynavItems();
  if (!items.length) return;
  const current = selectedIndex(items);
  const next = current < 0 ? (delta > 0 ? 0 : items.length - 1) : Math.min(items.length - 1, Math.max(0, current + delta));
  selectItem(items, next);
}

function openSelected(details) {
  const { items } = keynavItems();
  const item = items[selectedIndex(items)];
  if (!item) return;
  const primary = item.querySelector("[data-keynav-primary]") || item.querySelector("a");
  if (!primary) return;
  const href = details ? primary.dataset.details || primary.getAttribute("href") : primary.getAttribute("href");
  if (href) window.location.assign(href);
}

document.addEventListener("mouseover", (event) => {
  const { list, items } = keynavItems();
  if (!list || !(event.target instanceof Element)) return;
  const item = event.target.closest(list.dataset.keynav);
  if (item) selectItem(items, items.indexOf(item));
});

// ---------------------------------------------------------------------------
// Small interactions the templates rely on
// ---------------------------------------------------------------------------

document.addEventListener("click", (event) => {
  if (!(event.target instanceof Element)) return;
  const setTheme = event.target.closest("[data-set-theme]");
  if (setTheme) {
    applyTheme(setTheme.dataset.setTheme);
    setTheme.closest(".dropdown")?.querySelector("[tabindex]")?.blur();
    document.activeElement?.blur();
    return;
  }
  if (event.target.closest("[data-open-palette]")) {
    openPalette();
    return;
  }
  const toggle = event.target.closest("[data-toggle]");
  if (toggle) {
    const target = document.querySelector(toggle.dataset.toggle);
    if (target) {
      const hidden = target.classList.toggle("hidden");
      toggle.setAttribute("aria-expanded", String(!hidden));
      toggle.classList.toggle("btn-active", !hidden);
      toggle.classList.toggle("btn-ghost", hidden);
      if (!hidden) target.querySelector("input, select")?.focus();
    }
    return;
  }
  const open = event.target.closest("[data-open-dialog]");
  if (open) {
    const dialog = document.querySelector(open.dataset.openDialog);
    dialog?.showModal();
    dialog?.querySelector("input")?.focus();
    return;
  }
  if (event.target.closest("[data-close-dialog]")) {
    event.target.closest("dialog")?.close();
  }
});

document.addEventListener("input", (event) => {
  const input = event.target;
  if (!(input instanceof HTMLInputElement) || !input.dataset.confirmValue) return;
  const submit = input.closest("form")?.querySelector("[data-confirm-submit]");
  if (submit) submit.disabled = input.value !== input.dataset.confirmValue;
});

const relativeFormatter = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
const absoluteFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

export function formatRelativeTimes(root = document) {
  const now = Date.now();
  for (const element of root.querySelectorAll("time[data-relative]")) {
    const value = element.getAttribute("datetime");
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) continue;
    element.title = absoluteFormatter.format(date);
    const seconds = Math.round((date.getTime() - now) / 1000);
    const units = [
      ["year", 31536000],
      ["month", 2592000],
      ["week", 604800],
      ["day", 86400],
      ["hour", 3600],
      ["minute", 60],
    ];
    let text = "just now";
    for (const [unit, size] of units) {
      if (Math.abs(seconds) >= size) {
        text = relativeFormatter.format(Math.round(seconds / size), unit);
        break;
      }
    }
    element.textContent = text;
  }
}

// ---------------------------------------------------------------------------
// Global bindings
// ---------------------------------------------------------------------------

const globalShortcuts = [
  { keys: "mod+k", label: "Open command palette", group: "Global", run: () => (palette?.open ? palette.close() : openPalette()), palette: false },
  { keys: "?", label: "Show keyboard shortcuts", group: "Global", run: openHelp, palette: false },
  { keys: "g t", label: "Switch theme", group: "Global", run: cycleTheme, palette: false },
  { keys: "j", label: "Next item", group: "List", run: () => moveSelection(1), palette: false, hidden: !document.querySelector("[data-keynav]") },
  { keys: "k", label: "Previous item", group: "List", run: () => moveSelection(-1), palette: false, hidden: !document.querySelector("[data-keynav]") },
  { keys: "arrowdown", label: "Next item", group: "List", run: () => moveSelection(1), palette: false, hidden: true },
  { keys: "arrowup", label: "Previous item", group: "List", run: () => moveSelection(-1), palette: false, hidden: true },
  { keys: "enter", label: "Open selected item", group: "List", run: () => openSelected(false), palette: false, hidden: !document.querySelector("[data-keynav]") },
  { keys: "o", label: "Open details of selected item", group: "List", run: () => openSelected(true), palette: false, hidden: !document.querySelector("[data-keynav] [data-details]") },
  {
    keys: "escape",
    label: "Clear selection or leave the field",
    group: "Global",
    palette: false,
    hidden: true,
    run: () => {
      if (isEditableTarget(document.activeElement)) document.activeElement.blur();
      else selectItem(keynavItems().items, -1);
    },
  },
];

window.addEventListener("keydown", dispatch);
syncThemeMenu();
formatRelativeTimes();
document.addEventListener("htmx:afterSettle", () => formatRelativeTimes());
