import { $, el, api, toast, readStored, writeStored } from "./rtc.js";
import { configErrorVisible, configFailed, configForbidden, configLoaded, codeVisible, createHeaders, creationReady, initialCreateState, validCreateCode } from "./create-access.js";
import { newVdoRoom, vdoHostPath, vdoRoomID } from "./vdo.js";
import { installI18n, t, onLangChange } from "./i18n.js";

installI18n();

const roomLabelKey = () => ($("#roomEngine").value === "vdo" ? "home.roomPrefix" : "home.roomIdOptional");
const helpKey = () => ($("#roomEngine").value === "vdo" ? "home.roomEngineHelpVdo" : "home.roomEngineHelpLocal");

// The server may demand an access code before creating local rooms. Unknown is
// not open: until /api/config answers, local creation stays disabled, and a
// failure offers a retry instead of silently opening the form.
let createState = initialCreateState();
let submitting = false;

function applyCreateState() {
  const engine = $("#roomEngine").value;
  $("#createCodeRow").classList.toggle("hidden", !codeVisible(createState, engine));
  $("#createConfigError").classList.toggle("hidden", !configErrorVisible(createState, engine));
  $("#create").disabled = submitting || !creationReady(createState, engine);
}

async function loadCreateConfig() {
  createState = initialCreateState();
  applyCreateState();
  try { createState = configLoaded(await api("/api/config")); }
  catch { createState = configFailed(createState); }
  applyCreateState();
}

function syncEngineField() {
  const vdo = $("#roomEngine").value === "vdo";
  $("#room").maxLength = vdo ? 32 : 48;
  document.querySelector('label[for="room"]').textContent = t(roomLabelKey());
  $("#roomEngineHelp").textContent = t(helpKey());
  $("#lobby").disabled = vdo;
  applyCreateState();
}

$("#roomEngine").onchange = syncEngineField;
// The help text lives in #roomEngineHelp, which installI18n() already rewrote:
// keep it in sync with the selected engine, not just with the language.
onLangChange(syncEngineField);
$("#retryCreateConfig").addEventListener("click", loadCreateConfig);

const saved = () => {
  try {
    const rooms = JSON.parse(localStorage.getItem("swr.rooms") || "{}");
    return rooms && typeof rooms === "object" && !Array.isArray(rooms) ? rooms : {};
  } catch { return {}; }
};

$("#create").addEventListener("click", async () => {
  const engine = $("#roomEngine").value;
  submitting = true;
  applyCreateState();
  try {
    if (engine === "vdo") {
      const config = newVdoRoom($("#room").value.trim().toLowerCase(), $("#pw").value, $("#title").value.trim());
      const rooms = readStored("swr.vdoRooms",{},true); rooms[config.room] = config;
      writeStored("swr.vdoRooms",rooms,true);
      location.href = vdoHostPath(config); return;
    }
    if (!creationReady(createState, engine)) throw new Error(t("home.createConfigFailed"));
    const code = $("#createCode").value;
    if (createState.protected && !validCreateCode(code)) throw new Error(t("home.accessCodeInvalid"));
    if (new TextEncoder().encode($("#pw").value).length > 72) throw new Error(t("home.passwordTooLong"));
    const keys = await api("/api/rooms", {
      room: $("#room").value.trim().toLowerCase(),
      title: $("#title").value.trim(),
      password: $("#pw").value,
      lobby: $("#lobby").checked,
    }, createHeaders(code));
    $("#createCode").value = "";
    const all = saved();
    all[keys.room] = { ...keys, title: $("#title").value.trim(), created: Date.now() };
    try { localStorage.setItem("swr.rooms", JSON.stringify(all)); }
    catch { toast(t("home.saveStudioFailed")); }
    // The host key travels in the fragment: never sent to servers or logged.
    location.href = `/studio?room=${encodeURIComponent(keys.room)}#key=${encodeURIComponent(keys.hostKey)}`;
  } catch (e) {
    if (e.code === "room_create_forbidden") {
      // The server wants a code even if the last config answer said otherwise.
      createState = configForbidden();
      applyCreateState();
      $("#createCode").focus();
    }
    toast(e.message, "err");
  } finally {
    submitting = false;
    applyCreateState();
  }
});

const join = () => {
  const r = $("#joinRoom").value.trim().toLowerCase();
  if (r) location.href = `/guest?room=${encodeURIComponent(r)}`;
};
$("#join").addEventListener("click", join);
$("#joinRoom").addEventListener("keydown", (e) => e.key === "Enter" && join());

const rooms = Object.values(saved()).filter(r=>r && typeof r.room === "string" && typeof r.hostKey === "string").sort((a,b)=>b.created-a.created);
function studioCard(r) {
  const expired = r.persistent && r.expires && r.expires < Date.now();
  const state = expired ? t("home.expired") : r.persistent
    ? (r.expires ? t("home.savedUntil", { date: new Date(r.expires).toLocaleDateString() }) : t("home.noExpiry"))
    : t("home.temporary");
  return el("div",{class:"studio-card"},
    el("div",{class:"grow"},el("strong",{text:r.title || r.room}),el("span",{class:"muted",text:state})),
    el("a",{class:"btn small",href:`/studio?room=${encodeURIComponent(r.room)}#key=${encodeURIComponent(r.hostKey)}`,text:t("home.openStudio")}));
}

const vdoRecent = Object.values(readStored("swr.vdoRooms",{},true)).filter(r => {
  try { return !!(r && r.directorKey && r.password && vdoRoomID(r.room) && vdoHostPath(r)); } catch { return false; }
}).sort((a,b) => b.created-a.created).slice(0,5);

// Rebuilt on every language change: these lists are plain DOM, not data-i18n nodes.
function renderLists() {
  $("#savedStudios").replaceChildren();
  $("#recent").replaceChildren();
  const persistent = rooms.filter(r=>r.persistent);
  if (persistent.length) $("#savedStudios").append(el("h3",{text:t("home.savedRooms")}),...persistent.map(studioCard));
  const recent = rooms.filter(r=>!r.persistent).slice(0,5);
  if (recent.length) $("#recent").append(el("h3",{text:t("home.recentStudios")}),...recent.map(studioCard));
  if (vdoRecent.length) $("#recent").append(el("h3",{text:t("home.vdoSession")}),...vdoRecent.map(r => el("div",{class:"studio-card"},el("strong",{class:"grow",text:r.title || r.room}),el("a",{class:"btn small",href:vdoHostPath(r),text:t("home.openVdoRoom")}))));
}

renderLists();
onLangChange(renderLists);
loadCreateConfig();
