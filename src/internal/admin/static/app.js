const state = {
  config: null,
  icons: [],
  currentPage: "",
  currentButton: 0,
  previewDragFrom: null,
  buttonClipboard: null,
  dirty: false,
};

const dom = {
  list: q("#pageList"),
  currentTitle: q("#currentPageTitle"),
  status: q("#statusText"),
  preview: q("#previewGrid"),
  selectedButton: q("#selectedButtonLabel"),
  backupSelect: q("#backupSelect"),
  settingDevice: q("#settingDevice"),
  settingDeckModel: q("#settingDeckModel"),
  settingBrightness: q("#settingBrightness"),
  settingHoldMS: q("#settingHoldMS"),
  settingMediaPlayer: q("#settingMediaPlayer"),
  settingIconDir: q("#settingIconDir"),
  settingFontPath: q("#settingFontPath"),
  settingWeatherLocation: q("#settingWeatherLocation"),
  settingWeatherRefresh: q("#settingWeatherRefresh"),
  iconOptions: q("#iconOptions"),
  startPage: q("#startPage"),
  timeout: q("#pageTimeout"),
  backgroundType: q("#backgroundType"),
  backgroundColorPicker: q("#backgroundColorPicker"),
  backgroundColor: q("#backgroundColor"),
  backgroundPath: q("#backgroundPath"),
  backgroundMode: q("#backgroundMode"),
  backgroundPlayer: q("#backgroundPlayer"),
  backgroundModeWrap: q("#backgroundModeWrap"),
  backgroundPlayerWrap: q("#backgroundPlayerWrap"),
  backgroundColorWrap: q("#backgroundColorWrap"),
  backgroundPathWrap: q("#backgroundPathWrap"),
  backgroundEffect: q("#backgroundEffect"),
  buttonNumber: q("#buttonNumber"),
  pressAction: q("#pressAction"),
  holdAction: q("#holdAction"),
  layersList: q("#layersList"),
};

const backgroundTypes = ["solid", "color", "media_art", "image"];
const imageModes = ["fit", "fill", "center", "stretch"];
const positions = ["", "upper", "center", "lower", "top", "bottom"];
const layerTypes = ["empty", "color", "image", "animation", "icon", "media_play_pause", "text", "datetime", "weather"];
const effectTypes = ["", "blink", "pulse", "flash"];
const actionTypes = ["empty", "page", "media", "command", "display_mode", "brightness"];
const mediaCommands = ["previous", "play_pause", "next", "play", "pause", "stop"];
const brightnessCommands = ["up", "down", "increase", "decrease", "set"];
const deckModels = ["auto", "mini", "classic", "xl"];
const deckLayouts = {
  auto: { columns: 3, rows: 2 },
  mini: { columns: 3, rows: 2 },
  classic: { columns: 5, rows: 3 },
  xl: { columns: 8, rows: 4 },
};

q("#reloadBtn").addEventListener("click", load);
q("#addPageBtn").addEventListener("click", addPage);
q("#duplicatePageBtn").addEventListener("click", duplicatePage);
q("#renamePageBtn").addEventListener("click", renamePage);
q("#deletePageBtn").addEventListener("click", deletePage);
q("#backupBtn").addEventListener("click", createBackup);
q("#restoreBtn").addEventListener("click", restoreBackup);
q("#restartBtn").addEventListener("click", restartService);
q("#saveBtn").addEventListener("click", saveConfig);
q("#copyButtonBtn").addEventListener("click", copySelectedButton);
q("#pasteButtonBtn").addEventListener("click", pasteSelectedButton);
q("#clearButtonBtn").addEventListener("click", clearSelectedButton);
q("#addLayerBtn").addEventListener("click", addLayer);
q("#quickIconBtn").addEventListener("click", () => addPresetLayer("icon"));
q("#quickTextBtn").addEventListener("click", () => addPresetLayer("text"));
q("#quickImageBtn").addEventListener("click", () => addPresetLayer("image"));
q("#quickColorBtn").addEventListener("click", () => addPresetLayer("color"));
q("#quickAnimationBtn").addEventListener("click", () => addPresetLayer("animation"));
q("#quickWeatherBtn").addEventListener("click", () => addPresetLayer("weather"));

for (const value of backgroundTypes) dom.backgroundType.append(option(value));
for (const value of imageModes) dom.backgroundMode.append(option(value));
for (const value of deckModels) dom.settingDeckModel.append(option(value));

for (const input of [
  dom.settingDevice,
  dom.settingDeckModel,
  dom.settingBrightness,
  dom.settingHoldMS,
  dom.settingMediaPlayer,
  dom.settingIconDir,
  dom.settingFontPath,
  dom.settingWeatherLocation,
  dom.settingWeatherRefresh,
  dom.startPage,
  dom.timeout,
  dom.backgroundType,
  dom.backgroundPath,
  dom.backgroundMode,
  dom.backgroundPlayer,
]) {
  input.addEventListener("change", () => {
    syncSimpleFields();
    markDirty();
    if (input === dom.settingIconDir) loadIcons();
    if (input === dom.settingDeckModel) {
      state.currentButton = 0;
      render();
      return;
    }
    refresh();
  });
}

bindColorPair(dom.backgroundColorPicker, dom.backgroundColor, () => {
  syncSimpleFields();
  markDirty();
  refresh();
});

function q(selector, root = document) {
  return root.querySelector(selector);
}

function el(tag, className = "", text = "") {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text) node.textContent = text;
  return node;
}

function option(value, label = value || "none") {
  const node = document.createElement("option");
  node.value = value;
  node.textContent = label;
  return node;
}

function pageIds() {
  return Object.keys(state.config?.pages || {});
}

function currentPage() {
  return state.config.pages[state.currentPage] || { background: {}, buttons: {} };
}

function currentButton() {
  const page = currentPage();
  ensureButtons(page);
  return page.buttons[String(state.currentButton)];
}

function ensureButtons(page) {
  page.buttons ||= {};
  for (let i = 0; i < keyCount(); i += 1) {
    page.buttons[String(i)] ||= emptyButton();
    page.buttons[String(i)].layers ||= [];
    page.buttons[String(i)].press ||= { type: "" };
  }
}

function selectedDeckModel() {
  const model = state.config?.settings?.model || "auto";
  return deckLayouts[model] ? model : "auto";
}

function deckLayout() {
  return deckLayouts[selectedDeckModel()] || deckLayouts.auto;
}

function keyCount() {
  const layout = deckLayout();
  return layout.columns * layout.rows;
}

function configuredButtonCount(page) {
  return Object.keys(page.buttons || {}).length;
}

function emptyButton() {
  return { layers: [], press: { type: "" } };
}

async function load() {
  setStatus("Loading", "statusWarn");
  state.config = await requestJSON("/api/config");
  state.currentPage = state.config.settings.start_page || pageIds()[0];
  state.currentButton = 0;
  state.dirty = false;
  await loadIcons();
  await loadBackups();
  render();
  setStatus("Ready", "statusOk");
}

async function requestJSON(url, options = {}) {
  const response = await fetch(url, options);
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || response.statusText);
  return data;
}

function render() {
  if (state.currentButton >= keyCount()) state.currentButton = 0;
  renderPageList();
  renderSettingsEditor();
  renderPageEditor();
  renderButtonEditor();
  updatePreview();
}

function renderPageList() {
  dom.list.textContent = "";
  for (const id of pageIds()) {
    const item = el("button", "pageItem" + (id === state.currentPage ? " active" : ""));
    const name = el("span", "pageName", id);
    const count = el("span", "pageCount", `${configuredButtonCount(state.config.pages[id])}/${keyCount()}`);
    item.append(name, count);
    item.addEventListener("click", () => {
      state.currentPage = id;
      state.currentButton = 0;
      render();
    });
    dom.list.append(item);
  }
  dom.currentTitle.textContent = state.currentPage;
}

function renderPageEditor() {
  const page = currentPage();
  page.background ||= {};
  replaceOptions(dom.startPage, pageIds());
  dom.startPage.value = state.config.settings.start_page;
  dom.timeout.value = page.timeout_seconds || 0;
  dom.backgroundType.value = page.background.type || "solid";
  dom.backgroundColor.value = page.background.color || "";
  setColorPickerValue(dom.backgroundColorPicker, dom.backgroundColor.value, "#111827");
  dom.backgroundPath.value = page.background.path || "";
  dom.backgroundMode.value = page.background.mode || "fill";
  dom.backgroundPlayer.value = page.background.player || "";
  updateBackgroundVisibility();
  renderEffectEditor(dom.backgroundEffect, page.background.effect || {}, (effect) => {
    page.background.effect = effect;
    markDirty();
    updatePreview();
  });
}

function renderSettingsEditor() {
  const settings = state.config.settings;
  settings.font ||= {};
  settings.media ||= {};
  settings.weather ||= {};
  dom.settingDevice.value = settings.device || "";
  dom.settingDeckModel.value = selectedDeckModel();
  dom.settingBrightness.value = settings.brightness ?? "";
  dom.settingHoldMS.value = settings.hold_ms || "";
  dom.settingMediaPlayer.value = settings.media.player || "";
  dom.settingIconDir.value = settings.icon_dir || "";
  dom.settingFontPath.value = settings.font.path || "";
  dom.settingWeatherLocation.value = settings.weather.location || "";
  dom.settingWeatherRefresh.value = settings.weather.refresh_minutes || "";
}

function renderButtonEditor() {
  const button = currentButton();
  dom.buttonNumber.textContent = `Button ${state.currentButton}`;
  dom.selectedButton.textContent = `Button ${state.currentButton}`;
  renderActionEditor(dom.pressAction, button.press || { type: "" }, (action) => {
    button.press = action;
    markDirty();
  });
  renderActionEditor(dom.holdAction, button.hold || { type: "" }, (action) => {
    if (action.type) button.hold = action;
    else delete button.hold;
    markDirty();
  });
  renderLayers();
}

function replaceOptions(select, values) {
  select.textContent = "";
  for (const value of values) select.append(option(value));
}

function syncSimpleFields() {
  const settings = state.config.settings;
  const page = currentPage();
  settings.font ||= {};
  settings.media ||= {};
  settings.weather ||= {};
  setText(settings, "device", dom.settingDevice.value);
  setText(settings, "model", dom.settingDeckModel.value);
  setNumber(settings, "brightness", dom.settingBrightness.value);
  setNumber(settings, "hold_ms", dom.settingHoldMS.value);
  setText(settings, "icon_dir", dom.settingIconDir.value);
  setText(settings.font, "path", dom.settingFontPath.value);
  setText(settings.media, "player", dom.settingMediaPlayer.value);
  setText(settings.weather, "location", dom.settingWeatherLocation.value);
  setNumber(settings.weather, "refresh_minutes", dom.settingWeatherRefresh.value);
  page.timeout_seconds = numberValue(dom.timeout, 0);
  page.background ||= {};
  setText(page.background, "type", dom.backgroundType.value);
  setText(page.background, "color", dom.backgroundColor.value);
  setText(page.background, "path", dom.backgroundPath.value);
  setText(page.background, "mode", dom.backgroundMode.value);
  setText(page.background, "player", dom.backgroundPlayer.value);
  state.config.settings.start_page = dom.startPage.value;
  updateBackgroundVisibility();
}

function renderEffectEditor(container, effect, onChange) {
  container.textContent = "";
  const type = select(effectTypes, effect.type || "");
  const blink = numberInput(effect.blink_ms || "", "Blink ms");
  const duration = numberInput(effect.duration_ms || "", "Duration ms");
  const repeat = numberInput(effect.repeat || "", "Repeat");
  const grid = el("div", "fieldGrid effectGrid");
  const color = colorField("Color", effect.color || "", "#f59e0b", (value) => {
    setText(effect, "color", value);
    save();
  });
  grid.append(wrap("Type", type), color, wrap("Blink", blink), wrap("Duration", duration), wrap("Repeat", repeat));
  container.append(grid);

  const save = () => {
    const next = {};
    setText(next, "type", type.value);
    setText(next, "color", getColorValue(color));
    setNumber(next, "blink_ms", blink.value);
    setNumber(next, "duration_ms", duration.value);
    setNumber(next, "repeat", repeat.value);
    onChange(next);
  };
  for (const input of [type, blink, duration, repeat]) {
    input.addEventListener("change", save);
  }
}

function renderActionEditor(container, action, onChange) {
  container.textContent = "";
  const type = select(actionTypes, action.type || "empty");
  const target = select(["", ...pageIds()], action.page || action.command || "");
  const mediaCommand = select(mediaCommands, action.command || "play_pause");
  const shellCommand = textInput(action.command || "", "wtype -M win -P M");
  const displayCommand = select(["cycle"], action.command || "cycle");
  const brightnessCommand = select(brightnessCommands, action.command || "up");
  const player = textInput(action.player || "", "spotify");
  const step = numberInput(action.step || "", "10");
  const value = numberInput(action.value ?? "", "25");

  const grid = el("div", "actionFields");
  const rows = {
    type: wrap("Type", type),
    target: wrap("Target page", target),
    mediaCommand: wrap("Media command", mediaCommand),
    shellCommand: wrap("Shell command", shellCommand),
    displayCommand: wrap("Display", displayCommand),
    brightnessCommand: wrap("Brightness", brightnessCommand),
    player: wrap("Player", player),
    step: wrap("Step", step),
    value: wrap("Value", value),
  };
  grid.append(...Object.values(rows));
  container.append(grid);

  const updateVisibility = () => {
    const value = type.value;
    rows.target.hidden = value !== "page";
    rows.mediaCommand.hidden = value !== "media";
    rows.shellCommand.hidden = value !== "command";
    rows.displayCommand.hidden = value !== "display_mode";
    rows.brightnessCommand.hidden = value !== "brightness";
    rows.player.hidden = value !== "media";
    rows.step.hidden = value !== "brightness";
    rows.value.hidden = value !== "brightness" || brightnessCommand.value !== "set";
  };

  const save = () => {
    const next = {};
    const kind = type.value === "empty" ? "" : type.value;
    setText(next, "type", kind);
    if (kind === "page") setText(next, "page", target.value);
    if (kind === "media") {
      setText(next, "command", mediaCommand.value);
      setText(next, "player", player.value);
    }
    if (kind === "command") setText(next, "command", shellCommand.value);
    if (kind === "display_mode") setText(next, "command", displayCommand.value);
    if (kind === "brightness") {
      setText(next, "command", brightnessCommand.value);
      setNumber(next, "step", step.value);
      setNumber(next, "value", value.value);
    }
    onChange(next);
    updatePreview();
  };

  for (const input of [type, target, mediaCommand, shellCommand, displayCommand, brightnessCommand, player, step, value]) {
    input.addEventListener("change", () => {
      updateVisibility();
      save();
    });
  }
  updateVisibility();
}

function renderLayers() {
  const layers = currentButton().layers || [];
  dom.layersList.textContent = "";
  if (layers.length === 0) {
    dom.layersList.append(el("div", "emptyState", "No layers yet. Add a visual layer."));
  }
  layers.forEach((layer, index) => dom.layersList.append(layerCard(layer, index)));
}

function layerCard(layer, index) {
  const card = el("article", "layerCard");
  const header = el("div", "layerHeader");
  const badge = el("span", "layerBadge", layerBadge(layer.type));
  const typeWrap = el("div", "layerType");
  const type = select(layerTypes, layer.type || "empty");
  const summary = el("span", "layerSummary", layerSummary(layer));
  const tools = el("div", "layerTools");
  const up = el("button", "miniBtn", "↑");
  const down = el("button", "miniBtn", "↓");
  const remove = el("button", "miniBtn danger", "×");
  typeWrap.append(badge, type);
  tools.append(up, down, remove);
  header.append(typeWrap, summary, tools);
  const fields = el("div", "layerForm");
  card.append(header, fields);

  const rerender = () => {
    layer.type = type.value;
    renderLayerFields(fields, layer, () => {
      summary.textContent = layerSummary(layer);
    });
    summary.textContent = layerSummary(layer);
    badge.textContent = layerBadge(layer.type);
    markDirty();
    updatePreview();
  };
  type.addEventListener("change", rerender);
  up.addEventListener("click", () => moveLayer(index, -1));
  down.addEventListener("click", () => moveLayer(index, 1));
  remove.addEventListener("click", () => deleteLayer(index));
  renderLayerFields(fields, layer, () => {
    summary.textContent = layerSummary(layer);
  });
  return card;
}

function renderLayerFields(container, layer, onFieldChange = () => {}) {
  container.textContent = "";
  const row = el("div", "formRow");
  const fields = [];
  const addInput = (label, input, key, numeric = false) => {
    input.addEventListener("change", () => {
      if (numeric) setNumber(layer, key, input.value);
      else setText(layer, key, input.value);
      onFieldChange();
      markDirty();
      updatePreview();
    });
    fields.push(wrap(label, input));
  };
  const addColor = (label, key, value, fallback) => {
    fields.push(colorField(label, value, fallback, (next) => {
      setText(layer, key, next);
      onFieldChange();
      markDirty();
      updatePreview();
    }));
  };
  const addIcon = () => {
    fields.push(wrap("Icon", iconPicker(layer.icon || "", (next) => {
      setText(layer, "icon", next);
      onFieldChange();
      markDirty();
      updatePreview();
    })));
  };

  if (["icon"].includes(layer.type)) {
    row.classList.add("iconRow");
    addIcon();
    addInput("Size", numberInput(layer.size || "", "50"), "size", true);
    addInput("Position", select(positions, layer.position || ""), "position");
    addInput("Offset X", numberInput(layer["offset-x"] || "", "0"), "offset-x", true);
    addInput("Offset Y", numberInput(layer["offset-y"] || "", "0"), "offset-y", true);
    addColor("Outline", "outline_color", layer.outline_color || "", "#111827");
    addInput("Width", numberInput(layer.outline_width ?? "", "2"), "outline_width", true);
  } else if (["image"].includes(layer.type)) {
    row.classList.add("imageRow");
    addInput("Image path", textInput(layer.path || "", "/path/to/image.png"), "path");
    addInput("Mode", select(imageModes, layer.mode || "fit"), "mode");
  } else if (["animation"].includes(layer.type)) {
    row.classList.add("imageRow");
    addInput("GIF path", textInput(layer.path || "", "/path/to/animation.gif"), "path");
    addInput("Mode", select(imageModes, layer.mode || "fit"), "mode");
    addInput("Offset X", numberInput(layer["offset-x"] || "", "0"), "offset-x", true);
    addInput("Offset Y", numberInput(layer["offset-y"] || "", "0"), "offset-y", true);
  } else if (["color"].includes(layer.type)) {
    row.classList.add("colorRow");
    addColor("Color", "color", layer.color || "", "#111827");
    const effect = el("div", "effectInline");
    renderEffectEditor(effect, layer.effect || {}, (next) => {
      layer.effect = next;
      onFieldChange();
      markDirty();
      updatePreview();
    });
    fields.push(effect);
  } else if (["media_play_pause"].includes(layer.type)) {
    row.classList.add("iconRow");
    addInput("Player", textInput(layer.player || "", "spotify"), "player");
    addInput("Size", numberInput(layer.size || "", "50"), "size", true);
    addInput("Position", select(positions, layer.position || ""), "position");
    addInput("Offset X", numberInput(layer["offset-x"] || "", "0"), "offset-x", true);
    addInput("Offset Y", numberInput(layer["offset-y"] || "", "0"), "offset-y", true);
    addColor("Outline", "outline_color", layer.outline_color || "", "#111827");
    addInput("Width", numberInput(layer.outline_width ?? "", "2"), "outline_width", true);
  } else if (["text", "datetime", "weather"].includes(layer.type)) {
    row.classList.add("textRow");
    if (layer.type === "text") addInput("Text", textInput(layer.text || "", "Living"), "text");
    if (layer.type === "datetime") addInput("Format", textInput(layer.format || "", "ddd DD HH:mm"), "format");
    if (layer.type === "weather") addInput("Location", textInput(layer.location || "", "Buenos Aires"), "location");
    addInput("Size", numberInput(layer.font_size || "", "14"), "font_size", true);
    addInput("Position", select(positions, layer.position || "center"), "position");
    addInput("Offset X", numberInput(layer["offset-x"] || "", "0"), "offset-x", true);
    addInput("Offset Y", numberInput(layer["offset-y"] || "", "0"), "offset-y", true);
    addColor("Text color", "color", layer.color || "", "#ffffff");
    addColor("Outline", "outline_color", layer.outline_color || "", "#111827");
    addInput("Width", numberInput(layer.outline_width ?? "", "2"), "outline_width", true);
  } else {
    fields.push(el("div", "emptyState", "This layer is intentionally empty."));
  }

  row.append(...fields);
  container.append(row);
}

function addLayer() {
  currentButton().layers.push({ type: "text", text: "Label", position: "center", font_size: 14 });
  markDirty();
  renderButtonEditor();
  updatePreview();
}

function addPresetLayer(type) {
  const layers = currentButton().layers;
  if (type === "icon") {
    layers.push({ type: "icon", icon: preferredIcon("play.png"), size: 50, position: "center" });
  } else if (type === "text") {
    layers.push({ type: "text", text: "Label", font_size: 14, position: "center", color: "#ffffff", outline_color: "#000000" });
  } else if (type === "image") {
    layers.push({ type: "image", path: "", mode: "fit" });
  } else if (type === "color") {
    layers.unshift({ type: "color", color: "#111827" });
  } else if (type === "animation") {
    layers.push({ type: "animation", path: "", mode: "fit" });
  } else if (type === "weather") {
    layers.push({ type: "weather", font_size: 14, position: "center", color: "#ffffff", outline_color: "#000000" });
  }
  markDirty();
  renderButtonEditor();
  updatePreview();
}

function preferredIcon(name) {
  return state.icons.includes(name) ? name : state.icons[0] || name;
}

function moveLayer(index, direction) {
  const layers = currentButton().layers;
  const next = index + direction;
  if (next < 0 || next >= layers.length) return;
  [layers[index], layers[next]] = [layers[next], layers[index]];
  markDirty();
  renderButtonEditor();
  updatePreview();
}

function deleteLayer(index) {
  currentButton().layers.splice(index, 1);
  markDirty();
  renderButtonEditor();
  updatePreview();
}

function copySelectedButton() {
  state.buttonClipboard = structuredClone(currentButton() || emptyButton());
  setStatus(`Button ${state.currentButton} copied`, "statusOk");
}

function pasteSelectedButton() {
  if (!state.buttonClipboard) {
    setStatus("No copied button", "statusBad");
    return;
  }
  const page = currentPage();
  ensureButtons(page);
  page.buttons[String(state.currentButton)] = structuredClone(state.buttonClipboard);
  markDirty();
  renderButtonEditor();
  updatePreview();
}

function clearSelectedButton() {
  const page = currentPage();
  ensureButtons(page);
  page.buttons[String(state.currentButton)] = emptyButton();
  markDirty();
  renderButtonEditor();
  updatePreview();
}

function select(values, selected) {
  const node = document.createElement("select");
  for (const value of values) node.append(option(value));
  node.value = selected;
  return node;
}

function textInput(value, placeholder = "") {
  const node = document.createElement("input");
  node.value = value;
  node.placeholder = placeholder;
  return node;
}

function numberInput(value, placeholder = "") {
  const node = textInput(value, placeholder);
  node.type = "number";
  node.className = "numberInput";
  return node;
}

function wrap(label, child) {
  const node = document.createElement("label");
  node.append(el("span", "", label), child);
  return node;
}

function colorField(label, value, fallback, onChange) {
  const text = textInput(value, fallback);
  const picker = document.createElement("input");
  const pair = el("span", "colorPair");
  const node = wrap(label, pair);
  picker.type = "color";
  text.dataset.colorValue = value || "";
  setColorPickerValue(picker, value, fallback);
  bindColorPair(picker, text, (next) => {
    text.dataset.colorValue = next;
    onChange(next);
  });
  pair.append(picker, text);
  node.dataset.colorInput = "true";
  return node;
}

function iconPicker(value, onChange) {
  const root = el("div", "iconPicker");
  const preview = el("button", "iconPickerPreview");
  const input = textInput(value, "Search or icon.png");
  const popover = el("div", "iconPickerPopover");
  const grid = el("div", "iconPickerGrid");
  preview.type = "button";
  popover.hidden = true;
  popover.append(grid);
  root.append(preview, input);
  document.body.append(popover);

  const choose = (next) => {
    setValue(next);
    hide();
  };

  const setValue = (next, notify = true) => {
    input.value = next;
    renderIconPreview(preview, next);
    renderIconGrid(grid, input.value, next, choose);
    if (notify) onChange(next);
  };

  const show = () => {
    const rect = root.getBoundingClientRect();
    popover.style.left = `${Math.max(8, rect.left)}px`;
    popover.style.top = `${rect.bottom + 4}px`;
    popover.style.width = `${Math.min(430, Math.max(280, window.innerWidth - rect.left - 16))}px`;
    popover.hidden = false;
    renderIconGrid(grid, input.value, input.value, choose);
  };
  const hide = () => {
    popover.hidden = true;
  };

  preview.addEventListener("click", (event) => {
    event.stopPropagation();
    if (popover.hidden) show();
    else hide();
  });
  input.addEventListener("focus", show);
  input.addEventListener("input", () => {
    renderIconGrid(grid, input.value, input.value, choose);
    renderIconPreview(preview, input.value);
  });
  input.addEventListener("change", () => setValue(input.value));
  root.addEventListener("click", (event) => event.stopPropagation());
  popover.addEventListener("click", (event) => event.stopPropagation());
  document.addEventListener("click", hide);

  setValue(value, false);
  return root;
}

function renderIconPreview(button, name) {
  button.textContent = "";
  if (!name) {
    button.append(el("span", "", "IC"));
    return;
  }
  const image = document.createElement("img");
  image.alt = name;
  image.src = iconURL(name);
  image.addEventListener("error", () => {
    button.textContent = "";
    button.append(el("span", "", "IC"));
  });
  button.append(image);
}

function renderIconGrid(grid, query, selected, onSelect) {
  grid.textContent = "";
  const needle = String(query || "").trim().toLowerCase();
  const icons = state.icons.filter((icon) => !needle || icon.toLowerCase().includes(needle));
  if (icons.length === 0) {
    grid.append(el("div", "emptyState", "No icons found."));
    return;
  }
  for (const icon of icons.slice(0, 80)) {
    const item = el("button", "iconPickerItem" + (icon === selected ? " selected" : ""));
    item.type = "button";
    const image = document.createElement("img");
    image.alt = icon;
    image.src = iconURL(icon);
    item.title = icon;
    item.append(image, el("span", "", icon));
    item.addEventListener("click", () => onSelect(icon));
    grid.append(item);
  }
}

function iconURL(name) {
  const params = new URLSearchParams();
  params.set("name", name);
  const dir = state.config?.settings?.icon_dir || "";
  if (dir) params.set("dir", dir);
  return `/api/icon?${params.toString()}`;
}

function bindColorPair(picker, text, onChange) {
  picker.addEventListener("input", () => {
    text.value = picker.value;
    onChange(picker.value);
  });
  text.addEventListener("change", () => {
    const value = text.value.trim();
    setColorPickerValue(picker, value, picker.value);
    onChange(value);
  });
}

function getColorValue(field) {
  const input = q(".colorPair input:not([type='color'])", field);
  return input?.value || "";
}

function setColorPickerValue(picker, value, fallback) {
  picker.value = isHexColor(value) ? value : normalizeColorFallback(fallback);
}

function isHexColor(value) {
  return /^#[0-9a-fA-F]{6}$/.test(String(value || "").trim());
}

function normalizeColorFallback(value) {
  return isHexColor(value) ? value : "#111827";
}

function updateBackgroundVisibility() {
  const type = dom.backgroundType.value;
  dom.backgroundModeWrap.hidden = !["image", "media_art"].includes(type);
  dom.backgroundPlayerWrap.hidden = type !== "media_art";
  dom.backgroundPathWrap.hidden = type !== "image";
  dom.backgroundColorWrap.hidden = type === "image";
}

function layerBadge(type) {
  return {
    color: "CL",
    image: "IM",
    animation: "AN",
    icon: "IC",
    media_play_pause: "MP",
    text: "TX",
    datetime: "DT",
    weather: "WX",
    empty: "--",
  }[type || "empty"] || String(type || "--").slice(0, 2).toUpperCase();
}

function layerSummary(layer) {
  if (!layer?.type || layer.type === "empty") return "empty";
  if (layer.type === "color") return [layer.color || "color", layer.effect?.type].filter(Boolean).join(", ");
  if (layer.type === "icon") return [layer.icon || "icon", layer.position || "center"].join(", ");
  if (layer.type === "media_play_pause") return [layer.player || "media", layer.position || "center"].join(", ");
  if (layer.type === "image" || layer.type === "animation") return [layer.path || "path", layer.mode || "fit"].join(", ");
  if (layer.type === "text") return [layer.text || "text", layer.position || "center"].join(", ");
  if (layer.type === "datetime") return [layer.format || "datetime", layer.position || "center"].join(", ");
  if (layer.type === "weather") return [layer.location || "weather", layer.position || "center"].join(", ");
  return layer.type;
}

function setText(target, key, value) {
  const text = String(value ?? "").trim();
  if (text) target[key] = text;
  else delete target[key];
}

function setNumber(target, key, value) {
  if (value === "" || value === null || value === undefined) delete target[key];
  else target[key] = Number(value);
}

function numberValue(input, fallback) {
  return input.value === "" ? fallback : Number(input.value);
}

function refresh() {
  renderPageList();
  updatePreview();
}

let previewToken = 0;
async function updatePreview() {
  const token = ++previewToken;
  try {
    const preview = await requestJSON("/api/preview", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ config: state.config, page_id: state.currentPage }),
    });
    if (token !== previewToken) return;
    dom.preview.textContent = "";
    const columns = preview.columns || deckLayout().columns;
    const keySize = columns >= 8 ? 58 : columns >= 5 ? 68 : 78;
    dom.preview.style.setProperty("--preview-columns", String(columns));
    dom.preview.style.setProperty("--preview-key-size", `${keySize}px`);
    preview.keys.forEach((src, index) => {
      const button = el("button", "keyPreview" + (index === state.currentButton ? " selected" : ""));
      const image = document.createElement("img");
      image.alt = `Button ${index}`;
      image.src = src;
      image.draggable = false;
      button.draggable = true;
      button.append(image, el("span", "", String(index)));
      button.addEventListener("click", () => {
        state.currentButton = index;
        renderButtonEditor();
        updatePreview();
      });
      button.addEventListener("dragstart", (event) => {
        state.previewDragFrom = index;
        button.classList.add("dragSource");
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData("text/plain", String(index));
      });
      button.addEventListener("dragover", (event) => {
        event.preventDefault();
        event.dataTransfer.dropEffect = "move";
        if (state.previewDragFrom !== null && state.previewDragFrom !== index) button.classList.add("dropTarget");
      });
      button.addEventListener("dragleave", () => {
        button.classList.remove("dropTarget");
      });
      button.addEventListener("drop", (event) => {
        event.preventDefault();
        button.classList.remove("dropTarget");
        const raw = event.dataTransfer.getData("text/plain");
        const from = raw === "" ? state.previewDragFrom : Number(raw);
        swapPreviewButtons(from, index);
      });
      button.addEventListener("dragend", () => {
        state.previewDragFrom = null;
        for (const key of dom.preview.querySelectorAll(".keyPreview")) key.classList.remove("dragSource", "dropTarget");
      });
      dom.preview.append(button);
    });
  } catch (error) {
    setStatus(error.message, "statusBad");
  }
}

function swapPreviewButtons(from, to) {
  if (!Number.isInteger(from) || !Number.isInteger(to) || from === to) return;
  const page = currentPage();
  ensureButtons(page);
  const fromKey = String(from);
  const toKey = String(to);
  const fromButton = page.buttons[fromKey] || emptyButton();
  const toButton = page.buttons[toKey] || emptyButton();
  page.buttons[fromKey] = toButton;
  page.buttons[toKey] = fromButton;
  state.currentButton = to;
  state.previewDragFrom = null;
  markDirty();
  renderButtonEditor();
  updatePreview();
}

function markDirty() {
  state.dirty = true;
  setStatus("Unsaved changes", "statusWarn");
}

function setStatus(text, className = "") {
  dom.status.textContent = text;
  dom.status.className = className;
}

async function saveConfig() {
  try {
    syncSimpleFields();
    const saved = await requestJSON("/api/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(state.config),
    });
    state.dirty = false;
    await loadBackups();
    setStatus(saved.restarted ? "Saved and restarted" : "Saved", "statusOk");
  } catch (error) {
    setStatus(error.message, "statusBad");
  }
}

async function createBackup() {
  try {
    await requestJSON("/api/backups", { method: "POST" });
    await loadBackups();
    setStatus("Backup created", "statusOk");
  } catch (error) {
    setStatus(error.message, "statusBad");
  }
}

async function loadIcons() {
  try {
    const dir = encodeURIComponent(state.config?.settings?.icon_dir || "");
    state.icons = await requestJSON(`/api/icons?dir=${dir}`);
    dom.iconOptions.textContent = "";
    for (const icon of state.icons) dom.iconOptions.append(option(icon));
  } catch (error) {
    state.icons = [];
    dom.iconOptions.textContent = "";
  }
}

async function loadBackups() {
  const backups = await requestJSON("/api/backups");
  dom.backupSelect.textContent = "";
  for (const backup of backups) dom.backupSelect.append(option(backup.name, backup.name));
}

async function restoreBackup() {
  const name = dom.backupSelect.value;
  if (!name || !confirm(`Restore ${name}? Current config will be backed up first.`)) return;
  try {
    await requestJSON("/api/restore", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    await load();
    setStatus("Restored and restarted", "statusOk");
  } catch (error) {
    setStatus(error.message, "statusBad");
  }
}

async function restartService() {
  try {
    await requestJSON("/api/restart", { method: "POST" });
    setStatus("Service restarted", "statusOk");
  } catch (error) {
    setStatus(error.message, "statusBad");
  }
}

function addPage() {
  const id = uniquePageName(prompt("New page id", "new_page"));
  if (!id) return;
  state.config.pages[id] = { background: { type: "solid", color: "#111827" }, buttons: {} };
  state.currentPage = id;
  state.currentButton = 0;
  markDirty();
  render();
}

function duplicatePage() {
  const id = uniquePageName(prompt("Duplicate page as", `${state.currentPage}_copy`));
  if (!id) return;
  state.config.pages[id] = structuredClone(currentPage());
  state.currentPage = id;
  markDirty();
  render();
}

function renamePage() {
  const next = uniquePageName(prompt("Rename page", state.currentPage), state.currentPage);
  if (!next || next === state.currentPage) return;
  const renamed = {};
  for (const id of pageIds()) {
    if (id === state.currentPage) renamed[next] = state.config.pages[id];
    else renamed[id] = state.config.pages[id];
  }
  state.config.pages = renamed;
  for (const p of Object.values(state.config.pages)) {
    for (const button of Object.values(p.buttons || {})) {
      for (const action of [button.press, button.hold]) {
        if (action?.type === "page" && (action.page === state.currentPage || action.command === state.currentPage)) action.page = next;
      }
    }
  }
  if (state.config.settings.start_page === state.currentPage) state.config.settings.start_page = next;
  state.currentPage = next;
  markDirty();
  render();
}

function deletePage() {
  if (pageIds().length <= 1) {
    setStatus("Cannot delete the last page", "statusBad");
    return;
  }
  if (!confirm(`Delete page ${state.currentPage}?`)) return;
  delete state.config.pages[state.currentPage];
  state.currentPage = pageIds()[0];
  if (!state.config.pages[state.config.settings.start_page]) state.config.settings.start_page = state.currentPage;
  markDirty();
  render();
}

function uniquePageName(raw, allow = "") {
  const id = (raw || "").trim().replace(/\s+/g, "_");
  if (!id) return "";
  if (id !== allow && state.config.pages[id]) {
    setStatus(`Page ${id} already exists`, "statusBad");
    return "";
  }
  return id;
}

load().catch((error) => setStatus(error.message, "statusBad"));
