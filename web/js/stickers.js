// Chat stickers: drawn (emoji + text on a gradient), so there is nothing to
// download and they look the same in the chat and on the program output.
// Keep the ids in sync with internal/studio/chat.go (stickerIDs). The caption
// follows the interface language, read at draw time, so a Russian studio drops
// a Russian sticker.
import { t } from "./i18n.js";

const sticker = (id, emoji, c) => ({ id, emoji, c, get text() { return t("stickers." + id); } });

export const STICKERS = [
  sticker("hola", "👋", ["#2f6bff", "#7c5cff"]),
  sticker("bienvenidos", "🤗", ["#00b894", "#2f6bff"]),
  sticker("gracias", "🙏", ["#ff7a59", "#ff3d77"]),
  sticker("aplausos", "👏", ["#f7b500", "#ff6a3d"]),
  sticker("jaja", "😂", ["#ffcf3d", "#ff8a00"]),
  sticker("fuego", "🔥", ["#ff3d3d", "#ff9a00"]),
  sticker("wow", "😮", ["#00c6ff", "#0072ff"]),
  sticker("encanta", "❤️", ["#ff4d8d", "#c2185b"]),
  sticker("gg", "🏆", ["#ffd200", "#f7971e"]),
  sticker("like", "👍", ["#36d1dc", "#5b86e5"]),
  sticker("pregunta", "🙋", ["#8e2de2", "#4a00e0"]),
  sticker("felicidades", "🎉", ["#f953c6", "#b91d73"]),
  sticker("envivo", "🔴", ["#e52d27", "#b31217"]),
  sticker("saludos", "🇨🇱", ["#0039a6", "#d52b1e"]),
  sticker("cafe", "☕", ["#8d6e63", "#4e342e"]),
  sticker("yavuelvo", "⏳", ["#606c88", "#3f4c6b"]),
];
export const stickerById = (id) => STICKERS.find((s) => s.id === id) || null;
