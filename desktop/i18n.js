// Localisation for the capture shell. These are file:// pages loaded by the
// Electron main process, so there is no ES module graph and no server: the main
// process passes the OS language as ?lang= and the Spanish strings stay inline
// in the markup as the fallback.
(function () {
  const DICT = {
  es: {
    "picker.title": "Compartir · StreamWebRom",
    "picker.heading": "Compartir contenido",
    "picker.intro": "Elige qué mostrar. El audio de la sala queda fuera de la captura del sistema.",
    "picker.navAria": "Tipo de captura",
    "picker.window": "Ventana",
    "picker.screen": "Pantalla completa",
    "picker.tab": "Pestaña web",
    "picker.contentLabel": "Abrir contenido web en una pestaña de esta app",
    "picker.contentPlaceholder": "https://…",
    "picker.openTab": "Abrir pestaña",
    "picker.contentHint": "Para compartir una pestaña que ya tienes en Chrome, entra a la sala desde Chrome y usa su selector.",
    "picker.audioLabel": "Audio",
    "picker.audioApplication": "Solo la aplicación elegida",
    "picker.audioSystem": "Sistema sin las voces de la sala",
    "picker.audioNone": "Sin audio",
    "picker.refresh": "Actualizar",
    "picker.sourcesAria": "Fuentes disponibles",
    "picker.cancel": "Cancelar",
    "picker.availabilityWindow": "El sonido pertenece a la aplicación; varias ventanas del mismo programa pueden compartir audio.",
    "picker.availabilityTab": "La pestaña se captura con su sonido aislado.",
    "picker.availabilityScreen": "Se excluye el proceso de StreamWebRom y sus procesos de audio.",
    "picker.noNativeAudio": " La captura nativa de audio no está disponible en este equipo.",
    "picker.onlyTab": "Solo esta pestaña",
    "picker.openTabFirst": "Abre una pestaña de contenido y luego pulsa Actualizar.",
    "picker.noSources": "No hay fuentes disponibles. Pulsa Actualizar.",
    "home.title": "StreamWebRom · Captura avanzada",
    "home.heading1": "StreamWebRom",
    "home.heading2": "Captura avanzada",
    "home.intro": "Misma sala, cámaras y controles, con audio de aplicaciones y del sistema separado de la llamada.",
    "home.addressLabel": "Servidor o enlace de invitación",
    "home.open": "Abrir StreamWebRom",
    "home.hint": "Las pestañas de contenido se abren desde el selector de captura. La conexión, los permisos y el relé los sigue administrando tu servidor.",
  },
  en: {
    "picker.title": "Share · StreamWebRom",
    "picker.heading": "Share content",
    "picker.intro": "Pick what to show. The room audio stays out of the system capture.",
    "picker.navAria": "Capture type",
    "picker.window": "Window",
    "picker.screen": "Entire screen",
    "picker.tab": "Browser tab",
    "picker.contentLabel": "Open web content in a tab of this app",
    "picker.contentPlaceholder": "https://…",
    "picker.openTab": "Open tab",
    "picker.contentHint": "To share a tab you already have in Chrome, join the room from Chrome and use its picker.",
    "picker.audioLabel": "Audio",
    "picker.audioApplication": "Only the chosen application",
    "picker.audioSystem": "System without the room voices",
    "picker.audioNone": "No audio",
    "picker.refresh": "Refresh",
    "picker.sourcesAria": "Available sources",
    "picker.cancel": "Cancel",
    "picker.availabilityWindow": "The sound belongs to the application; several windows of the same program can share audio.",
    "picker.availabilityTab": "The tab is captured with its own isolated sound.",
    "picker.availabilityScreen": "The StreamWebRom process and its audio processes are excluded.",
    "picker.noNativeAudio": " Native audio capture is not available on this computer.",
    "picker.onlyTab": "Only this tab",
    "picker.openTabFirst": "Open a content tab and then press Refresh.",
    "picker.noSources": "No sources available. Press Refresh.",
    "home.title": "StreamWebRom · Advanced capture",
    "home.heading1": "StreamWebRom",
    "home.heading2": "Advanced capture",
    "home.intro": "Same room, cameras and controls, with application and system audio kept separate from the call.",
    "home.addressLabel": "Server or invite link",
    "home.open": "Open StreamWebRom",
    "home.hint": "Content tabs are opened from the capture picker. Your server still manages the connection, the permissions and the relay.",
  },
  ru: {
    "picker.title": "Поделиться · StreamWebRom",
    "picker.heading": "Поделиться содержимым",
    "picker.intro": "Выберите, что показывать. Звук комнаты в захват системы не попадает.",
    "picker.navAria": "Тип захвата",
    "picker.window": "Окно",
    "picker.screen": "Весь экран",
    "picker.tab": "Вкладка браузера",
    "picker.contentLabel": "Открыть веб-содержимое во вкладке этого приложения",
    "picker.contentPlaceholder": "https://…",
    "picker.openTab": "Открыть вкладку",
    "picker.contentHint": "Чтобы показать вкладку, которая уже открыта в Chrome, зайдите в комнату из Chrome и используйте его выбор источника.",
    "picker.audioLabel": "Звук",
    "picker.audioApplication": "Только выбранное приложение",
    "picker.audioSystem": "Система без голосов комнаты",
    "picker.audioNone": "Без звука",
    "picker.refresh": "Обновить",
    "picker.sourcesAria": "Доступные источники",
    "picker.cancel": "Отмена",
    "picker.availabilityWindow": "Звук принадлежит приложению; несколько окон одной программы могут делить звук.",
    "picker.availabilityTab": "Вкладка захватывается со своим изолированным звуком.",
    "picker.availabilityScreen": "Процесс StreamWebRom и его аудиопроцессы исключаются.",
    "picker.noNativeAudio": " Захват системного звука недоступен на этом компьютере.",
    "picker.onlyTab": "Только эта вкладка",
    "picker.openTabFirst": "Откройте вкладку с содержимым и нажмите «Обновить».",
    "picker.noSources": "Источников нет. Нажмите «Обновить».",
    "home.title": "StreamWebRom · Продвинутый захват",
    "home.heading1": "StreamWebRom",
    "home.heading2": "Продвинутый захват",
    "home.intro": "Та же комната, камеры и управление, но звук приложений и системы отделён от звонка.",
    "home.addressLabel": "Сервер или ссылка-приглашение",
    "home.open": "Открыть StreamWebRom",
    "home.hint": "Вкладки с содержимым открываются из окна выбора источника. Соединением, правами и ретранслятором по-прежнему управляет ваш сервер.",
  },
  };
  function initial() {
    let tag = "";
    try { tag = new URLSearchParams(location.search).get("lang") || ""; } catch (_e) {}
    tag = (tag || navigator.language || "es").toLowerCase();
    if (tag.indexOf("ru") === 0) return "ru";
    if (tag.indexOf("en") === 0) return "en";
    return "es";
  }
  const lang = initial();
  const dict = DICT[lang] || DICT.es;
  window.desktopLang = lang;
  window.t = (key) => dict[key] || DICT.es[key] || key;
  function apply() {
    document.documentElement.lang = lang;
    for (const node of document.querySelectorAll("[data-i18n]")) node.textContent = window.t(node.dataset.i18n);
    for (const node of document.querySelectorAll("[data-i18n-title]")) node.title = window.t(node.dataset.i18nTitle);
    for (const node of document.querySelectorAll("[data-i18n-placeholder]")) node.placeholder = window.t(node.dataset.i18nPlaceholder);
    for (const node of document.querySelectorAll("[data-i18n-aria]")) node.setAttribute("aria-label", window.t(node.dataset.i18nAria));
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", apply, { once: true });
  else apply();
})();
