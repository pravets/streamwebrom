// Shared chat for the studio and the guest page: messages with time, host
// badge, links and @mentions, big emoji-only messages, an emoji picker (with
// recents) and stickers. Also the conversation layer: the room-wide "General"
// chat plus one direct conversation per peer, with unread badges.
import { el } from "./rtc.js";
import { t } from "./i18n.js";
import { STICKERS, stickerById } from "./stickers.js";

// Category names are i18n keys, resolved when the picker renders so a language
// switch relabels the groups without touching the emoji lists.
export const EMOJIS = [
  ["😀", "misc.emojiFaces", "😀 😃 😄 😁 😆 😅 😂 🤣 🙂 😉 😊 😇 🥰 😍 🤩 😘 😋 😛 😜 🤪 🤗 🤭 🤫 🤔 🤐 🤨 😐 😏 😒 🙄 😬 😌 😴 🤤 😷 🤒 🥵 🥶 🥴 😵 🤯 🤠 🥳 😎 🤓 🧐 😕 😟 🙁 😮 😲 😳 🥺 😢 😭 😱 😤 😡 🤬 💀 🤡 👻 👽 🤖 😺 🙈 🙉 🙊"],
  ["👍", "misc.emojiGestures", "👍 👎 👌 🤌 ✌️ 🤞 🤟 🤘 🤙 👈 👉 👆 👇 ☝️ ✋ 🤚 🖐️ 🖖 👋 👏 🙌 👐 🤲 🤝 🙏 💪 🫶 ✍️ 👀 🧠 🗣️ 🙋 🙆 🙅 🤷 🤦 💃 🕺"],
  ["❤️", "misc.emojiHearts", "❤️ 🧡 💛 💚 💙 💜 🖤 🤍 🤎 💔 ❣️ 💕 💞 💓 💗 💖 💘 💝 💯 💢 💥 💫 💦 💨 💬 💭"],
  ["🎉", "misc.emojiParty", "🎉 🎊 🎈 🎁 🎂 🍾 🥂 🍻 🏆 🥇 🏅 🎖️ ⭐ 🌟 ✨ 🔥 ⚡ 🌈 ☀️ 🎶 🎵 🎤 🎧 🎸 🥁 🎮 🎬 📸 🎯"],
  ["⚽", "misc.emojiMisc", "⚽ 🏀 🏈 ⚾ 🎾 🏐 🚀 ✈️ 🚗 🏠 💻 📱 ⌚ 💡 📌 📣 🔔 ⏰ ⏳ ✅ ❌ ❓ ❗ ⚠️ 🆗 🆕 🔴 🟢 🔵 ☕ 🍕 🍔 🌮 🍿 🍩 🍓 🥑 🐶 🐱 🦄 🐔 🇨🇱"],
].map(([icon, name, list]) => ({ icon, name, list: list.split(" ") }));

const RECENT_KEY = "swr.emoji.recent";
const recents = () => { try { return JSON.parse(localStorage.getItem(RECENT_KEY) || "[]").slice(0, 24); } catch { return []; } };
const pushRecent = (e) => { try { localStorage.setItem(RECENT_KEY, JSON.stringify([e, ...recents().filter((x) => x !== e)].slice(0, 24))); } catch {} };

const URL_RE = /\bhttps?:\/\/[^\s<>"']+/g;
// Built from a string: the repo's JS linter can't parse \p{…} regex literals.
const EMOJI_ONLY_RE = new RegExp("^(?:\\p{Extended_Pictographic}|\\p{Regional_Indicator}|\\u200d|\\ufe0f|\\p{Emoji_Modifier}|\\s)+$", "u");

/** A sticker drawn as an HTML element. */
export function stickerEl(st, big = false) {
  return el("div", { class: "sticker" + (big ? " big" : ""), title: st.text, style: `background:linear-gradient(135deg,${st.c[0]},${st.c[1]})` },
    el("span", { class: "se", text: st.emoji }), el("span", { class: "st", text: st.text }));
}

function richText(text, myName) {
  const out = [];
  let last = 0;
  for (const m of text.matchAll(URL_RE)) {
    const url = m[0].replace(/[).,;:!?]+$/, "");
    if (m.index > last) out.push(...mentions(text.slice(last, m.index), myName));
    out.push(el("a", { href: url, target: "_blank", rel: "noopener noreferrer nofollow", text: url }));
    last = m.index + url.length;
  }
  if (last < text.length) out.push(...mentions(text.slice(last), myName));
  return out;
}
function mentions(s, myName) {
  if (!myName) return [s];
  const at = "@" + myName.toLowerCase();
  const i = s.toLowerCase().indexOf(at);
  if (i < 0) return [s];
  return [s.slice(0, i), el("mark", { text: s.slice(i, i + at.length) }), ...mentions(s.slice(i + at.length), myName)];
}

const hhmm = (ts) => ts ? new Date(ts).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";

/**
 * Render one chat message. opts: { myId, myName, actions: [elements] }.
 * Returns the element (it also tells whether it mentions me in .dataset.mention).
 */
export function chatMessage(c, opts = {}) {
  const mine = opts.myId && c.peerId === opts.myId;
  const st = c.sticker ? stickerById(c.sticker) : null;
  const text = c.text || "";
  const body = st ? stickerEl(st) : el("div", { class: "body" + (EMOJI_ONLY_RE.test(text) && [...text.replace(/\s/g, "")].length <= 8 ? " bigemoji" : "") }, ...richText(text, opts.myName));
  const mention = !!(opts.myName && !mine && text.toLowerCase().includes("@" + opts.myName.toLowerCase()));
  const msg = el("div", { class: "msg" + (mine ? " mine" : "") + (mention ? " mention" : "") + (st ? " hassticker" : "") },
    ...(opts.actions || []),
    el("div", { class: "meta" },
      el("b", { text: c.from }),
      c.role === "host" ? el("span", { class: "badge", text: t("misc.chatHostBadge") }) : null,
      c.source && c.source !== "chat" ? el("span", { class: "badge src", text: c.source }) : null,
      el("span", { class: "time", text: hhmm(c.ts) })),
    body);
  msg.dataset.mention = mention ? "1" : "";
  return msg;
}

/** Append to a chat log, keeping the view pinned to the bottom only if it was. */
export function appendChat(box, msg) {
  const atBottom = box.scrollHeight - box.scrollTop - box.clientHeight < 60;
  box.append(msg);
  while (box.children.length > 300) box.firstChild.remove();
  if (atBottom || msg.classList.contains("mine")) box.scrollTop = box.scrollHeight;
}

/**
 * Add emoji and sticker buttons to a composer row (input + send button).
 * send(payload) is called with { text } or { sticker }.
 */
export function mountComposer(row, input, send) {
  const picker = el("div", { class: "chatpicker hidden", role: "dialog", "aria-label": t("misc.chatPickerAria") });
  let tab = "emoji";
  const insert = (e) => {
    const s = input.selectionStart ?? input.value.length, t = input.selectionEnd ?? s;
    input.value = (input.value.slice(0, s) + e + input.value.slice(t)).slice(0, Number(input.maxLength) > 0 ? input.maxLength : 500);
    input.focus();
    const p = s + e.length; input.setSelectionRange(p, p);
    pushRecent(e);
  };
  const render = () => {
    const tabs = el("div", { class: "pickertabs" },
      el("button", { type: "button", class: tab === "emoji" ? "on" : "", onclick: () => { tab = "emoji"; render(); }, text: t("misc.chatTabEmojis") }),
      el("button", { type: "button", class: tab === "stickers" ? "on" : "", onclick: () => { tab = "stickers"; render(); }, text: t("misc.chatTabStickers") }));
    let content;
    if (tab === "emoji") {
      const groups = [];
      const rec = recents();
      if (rec.length) groups.push({ name: "misc.emojiRecent", list: rec });
      groups.push(...EMOJIS);
      content = el("div", { class: "emojigrid" }, ...groups.flatMap((g) => [
        el("div", { class: "grouptitle", text: t(g.name) }),
        ...g.list.map((e) => el("button", { type: "button", class: "emo", title: e, onclick: () => insert(e), text: e })),
      ]));
    } else {
      content = el("div", { class: "stickergrid" }, ...STICKERS.map((st) => {
        const b = el("button", { type: "button", class: "stk", title: st.text, onclick: () => { send({ sticker: st.id }); close(); } });
        b.append(stickerEl(st));
        return b;
      }));
    }
    picker.replaceChildren(tabs, content);
  };
  const open = (which) => { tab = which; render(); picker.classList.remove("hidden"); };
  const close = () => picker.classList.add("hidden");
  const toggle = (which) => (!picker.classList.contains("hidden") && tab === which ? close() : open(which));
  const emojiBtn = el("button", { type: "button", class: "small chattool", title: t("misc.chatEmojis"), "aria-label": t("misc.chatEmojis"), onclick: () => toggle("emoji"), text: "😊" });
  const stickBtn = el("button", { type: "button", class: "small chattool", title: t("misc.chatStickers"), "aria-label": t("misc.chatStickers"), onclick: () => toggle("stickers"), text: "🏷️" });
  row.classList.add("chatcomposer");
  row.prepend(emojiBtn, stickBtn);
  row.after(picker);
  document.addEventListener("pointerdown", (e) => { if (!picker.contains(e.target) && e.target !== emojiBtn && e.target !== stickBtn) close(); });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") close(); });
  // :) style shortcuts become emoji when sent
  const SHORT = { ":)": "🙂", ":D": "😄", ";)": "😉", ":(": "🙁", "<3": "❤️", ":P": "😛", ":o": "😮", "xD": "😆" };
  const doSend = () => {
    let v = input.value.trim();
    if (!v) return;
    v = v.replace(/(^|\s)(:\)|:D|;\)|:\(|<3|:P|:o|xD)(?=\s|$)/g, (_, sp, k) => sp + SHORT[k]);
    send({ text: v });
    input.value = "";
    close();
  };
  return { send: doSend, close };
}

// ---------- conversations: general chat + direct messages ----------

/** Conversation key of the room-wide chat. */
export const GENERAL = "general";

/** Sorted pair of peer ids: both sides of a dm share one key (server parity). */
export function conversationKey(a, b) {
  return a < b ? a + "|" + b : b + "|" + a;
}

/** The other participant of a dm envelope; works for the sender's echo too. */
export function dmPeer(m, myId) {
  return m.peerId === myId ? m.to : m.peerId;
}

/**
 * Unread reducer. state maps conversation key -> count; m is a "chat" or "dm"
 * signalling envelope. A message counts for its conversation unless that
 * conversation is open and actually visible (a closed drawer or an inactive
 * tab never marks it read) or the message is my own echo. Pure: returns a new
 * object and never mutates the input.
 */
export function unreadAfter(state, m, openKey, myId, visible = true) {
  const key = m.type === "dm" ? conversationKey(m.peerId, m.to) : GENERAL;
  if (visible && key === openKey) return state;
  if (myId && m.comment && m.comment.peerId === myId) return state;
  return { ...state, [key]: (state[key] || 0) + 1 };
}

/**
 * Wire a chat pane (log + composer + conversation switcher). Shared by the
 * studio tab and the guest drawer. opts:
 *   log, switcher, input, sendButton  DOM nodes
 *   send(msg)          signalling send
 *   myId(), myName()   getters (the peer id changes on reconnect)
 *   peers()            [{id,name,role}] currently in the room
 *   canDM(peer)        page policy: may I write to this peer?
 *   policy()           "all" | "host-only" | "off" ("" means all)
 *   actionsFor(c,key)  optional per-message buttons (show on screen, delete)
 *   onUnread(total)    badge callback (tab or drawer button)
 *   onMention(c)       optional mention notification
 *   offNote            optional node shown while dms are off
 *   visible()          is the pane on screen right now? (drawer open / tab
 *                      active) unread only skips the open conversation when
 *                      the user can actually see it; defaults to always true
 */
export function createChat(opts) {
  const { log, switcher, input, sendButton, send, myId, myName, peers, canDM, policy, actionsFor, onUnread, onMention, offNote, visible = () => true } = opts;
  const histories = new Map(); // key -> [comment]
  const drafts = new Map(); // key -> unsent composer text, kept per conversation
  let openKey = GENERAL;
  let unread = {};
  const total = () => Object.values(unread).reduce((a, b) => a + b, 0);
  const keyOf = (m) => (m.type === "dm" ? conversationKey(m.peerId, m.to) : GENERAL);
  const peerOf = (key) => key.split("|").find((id) => id !== myId()) || "";
  const nameOf = (id) => {
    const p = peers().find((x) => x.id === id);
    if (p) return p.name;
    for (const list of histories.values()) for (const c of list) if (c.peerId === id) return c.from;
    return id;
  };
  const node = (c, key) => {
    const n = chatMessage(c, { myId: myId(), myName: myName(), actions: actionsFor ? actionsFor(c, key) : [] });
    n.dataset.cid = c.id;
    return n;
  };
  // Switching conversations — including the automatic fallback when the
  // policy changes — must never carry a draft into another conversation:
  // each one keeps its own unsent text until it is sent or edited away.
  const setOpen = (key) => {
    if (input.value) drafts.set(openKey, input.value); else drafts.delete(openKey);
    openKey = key;
    input.value = drafts.get(key) || "";
  };
  // May I write to this dm right now? The peer must still be in the room and
  // the page policy (canDM) must allow it; history stays readable regardless.
  const dmAllowed = (key) => {
    const peer = peers().find((p) => p.id === peerOf(key));
    return policy() !== "off" && !!peer && canDM(peer);
  };
  const composerRow = input.parentElement;
  const composer = mountComposer(composerRow, input, (x) => {
    if (openKey === GENERAL) {
      send(x.sticker ? { type: "chat", kind: "sticker", text: x.sticker } : { type: "chat", text: x.text });
    } else {
      // Re-check at send time: the policy can change while the user types.
      const to = peerOf(openKey);
      if (!to || !dmAllowed(openKey)) return;
      send(x.sticker ? { type: "dm", to, kind: "sticker", text: x.sticker } : { type: "dm", to, text: x.text });
    }
    drafts.delete(openKey);
  });
  sendButton.onclick = composer.send;
  input.addEventListener("keydown", (e) => e.key === "Enter" && !e.isComposing && composer.send());

  function renderSwitcher() {
    const me = myId();
    const keys = new Set();
    for (const p of peers()) if (p.id !== me && canDM(p)) keys.add(conversationKey(me, p.id));
    for (const key of histories.keys()) if (key !== GENERAL) keys.add(key); // keep history of peers who left
    const btn = (key, label) => {
      const n = unread[key] || 0;
      return el("button", { type: "button", class: "small" + (key === openKey ? " on" : ""), onclick: () => open(key) },
        label, n ? el("span", { class: "badge", text: n > 99 ? "99+" : String(n) }) : null);
    };
    const dmKeys = [...keys].sort((a, b) => nameOf(peerOf(a)).localeCompare(nameOf(peerOf(b))));
    switcher.replaceChildren(btn(GENERAL, t("misc.chatGeneral")), ...dmKeys.map((key) => btn(key, nameOf(peerOf(key)))));
  }
  function renderLog() {
    log.replaceChildren(...(histories.get(openKey) || []).map((c) => node(c, openKey)));
    log.scrollTop = log.scrollHeight;
  }
  function syncComposer() {
    // "off" disables direct messages only: the general chat keeps working.
    // A forbidden (or departed) peer keeps its history readable, but the
    // composer follows canDM — not just the "off" policy — so a host-only
    // room never offers a guest-to-guest composer from a saved conversation.
    const off = openKey !== GENERAL && !dmAllowed(openKey);
    composerRow.classList.toggle("hidden", off);
    if (offNote) offNote.classList.toggle("hidden", !off);
  }
  function refresh() {
    // A policy change can forbid the open conversation: fall back to general.
    if (openKey !== GENERAL) {
      const other = peers().find((p) => p.id === peerOf(openKey));
      if (policy() === "off" || (other && !canDM(other))) { setOpen(GENERAL); renderLog(); }
    }
    renderSwitcher(); syncComposer(); if (onUnread) onUnread(total());
  }
  function open(key) {
    setOpen(key);
    delete unread[key];
    renderLog(); renderSwitcher(); syncComposer(); if (onUnread) onUnread(total());
  }
  function receive(m) {
    const c = m.comment;
    if (!c) return;
    const key = keyOf(m);
    const list = histories.get(key) || [];
    list.push(c);
    if (list.length > 300) list.splice(0, list.length - 300);
    histories.set(key, list);
    const n = node(c, key);
    if (key === openKey) appendChat(log, n);
    unread = unreadAfter(unread, m, openKey, myId(), visible());
    renderSwitcher(); if (onUnread) onUnread(total());
    if (n.dataset.mention && c.peerId !== myId() && onMention) onMention(c);
  }
  function remove(id) {
    for (const list of histories.values()) {
      const i = list.findIndex((c) => c.id === id);
      if (i >= 0) list.splice(i, 1);
    }
    log.querySelector(`[data-cid="${CSS.escape(id)}"]`)?.remove();
  }
  // welcome/reconnect: the server history is authoritative and replaces ours.
  function seed(chat, dms) {
    setOpen(GENERAL); // the old draft stays with its own conversation
    histories.clear(); unread = {};
    histories.set(GENERAL, (chat || []).slice(-300));
    for (const c of dms || []) {
      const key = conversationKey(c.peerId, c.to);
      const list = histories.get(key) || [];
      list.push(c);
      histories.set(key, list);
    }
    renderLog(); renderSwitcher(); syncComposer(); if (onUnread) onUnread(0);
  }
  renderSwitcher(); syncComposer();
  return { seed, receive, remove, open, refresh, focus: () => open(openKey), openDM: (peerId) => open(conversationKey(myId(), peerId)), unreadTotal: total };
}
