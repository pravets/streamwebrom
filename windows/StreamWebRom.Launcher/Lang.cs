using System.Globalization;

namespace StreamWebRom.Launcher;

/// <summary>
/// Tiny UI dictionary (es = source and fallback, en/ru translated). Chosen once
/// from the Windows UI culture; no .resx or satellite assemblies to ship.
/// </summary>
public static class Lang
{
    private static readonly Dictionary<string, string> Es = new()
    {
        ["common.copy"] = "Copiar",
        ["common.optional"] = "opcional",
        ["main.tunnel.enable"] = "Publicar en internet con Cloudflare Tunnel",
        ["main.token.placeholder"] = "pega el token del túnel",
        ["main.token.saved"] = "token guardado (cifrado) · pega otro para cambiarlo",
        ["main.turnUser.placeholder"] = "usuario TURN",
        ["main.turnPassword.placeholder"] = "contraseña TURN",
        ["main.turnPassword.saved"] = "contraseña guardada (cifrada)",
        ["main.https"] = "HTTPS (necesario para cámaras de otros equipos/teléfonos)",
        ["main.start"] = "Iniciar",
        ["main.stop"] = "Detener",
        ["main.openStudio"] = "Abrir estudio",
        ["main.recordings"] = "Grabaciones",
        ["main.obsHelp"] = "Cómo usar con OBS",
        ["main.advancedCapture"] = "Captura avanzada",
        ["urls.local"] = "Este equipo",
        ["urls.lan"] = "Tu red (teléfonos, otros PCs)",
        ["urls.public"] = "Internet (invitados y OBS remotos)",
        ["urls.tunnelOff"] = "(túnel desactivado)",
        ["card.links"] = "Enlaces del estudio",
        ["card.server"] = "Servidor",
        ["card.network"] = "Red",
        ["card.publish"] = "Publicar en internet",
        ["card.log"] = "Registro",
        ["net.port"] = "Puerto web",
        ["net.udpPort"] = "Puerto UDP media",
        ["net.publicIp"] = "IP pública",
        ["net.turnRelay"] = "Relé TURN (redes móviles)",
        ["net.user"] = "Usuario",
        ["net.password"] = "Contraseña",
        ["net.testConnection"] = "Probar conexión",
        ["net.relays"] = "TURN local / Cloudflare",
        ["net.applyRestart"] = "Aplicar y reiniciar",
        ["tun.domain"] = "Dominio",
        ["tun.token"] = "Token",
        ["tun.tokenHelp"] = "¿Dónde está el token?",
        ["tray.open"] = "Abrir estudio",
        ["tray.capture"] = "Captura avanzada",
        ["tray.show"] = "Mostrar ventana",
        ["tray.quit"] = "Salir",
        ["tray.balloon"] = "Sigue funcionando en la bandeja. Clic derecho → Salir para cerrarlo.",
        ["status.online"] = "● en línea",
        ["status.onlineHost"] = "● en línea · {0}",
        ["status.stopped"] = "detenido",
        ["status.stoppedCode"] = "detenido (código {0})",
        ["status.error"] = "error",
        ["log.tunnelPasteToken"] = "Túnel: pega el token de Cloudflare y pulsa “Aplicar y reiniciar” para publicar en {0}",
        ["log.ffmpegMissing"] = "Aviso: ffmpeg no encontrado. El multistream RTMP quedará deshabilitado; OBS, invitados y grabación funcionan igual.",
        ["log.tunnelActive"] = "Túnel Cloudflare activo → https://{0}",
        ["log.tunnelError"] = "Túnel: {0}",
        ["log.errorPrefix"] = "Error: {0}",
        ["log.tunnelPrefix"] = "[túnel] ",
        ["tunnel.downloading"] = "Descargando cloudflared (una sola vez)…",
        ["tunnel.ready"] = "cloudflared listo.",
        ["help.obs.body"] =
            "1. Abre el estudio y crea tu programa.\n" +
            "2. Pestaña OBS → copia “Programa completo”.\n" +
            "3. En OBS: Fuentes → + → Navegador → pega la URL, 1920×1080, marca “Controlar audio vía OBS”.\n\n" +
            "Para enviar OBS al estudio: Ajustes → Emisión → Servicio WHIP, con el servidor y el Bearer Token de la pestaña OBS.\n\n" +
            "Multistream a YouTube/Twitch: instala ffmpeg (winget install ffmpeg) o copia ffmpeg.exe junto a StreamWebRom.exe.",
        ["help.token.body"] =
            "Cloudflare → Zero Trust → Networks → Tunnels → streamwebrom → Configure → copia el token de “Install and run a connector” (la parte larga después de --token).\n\n" +
            "Se guarda cifrado con tu usuario de Windows y nunca sale de este PC.\n\n" +
            "El túnel publica la web; el audio y video necesitan además UDP accesible o un servidor TURN. Para redes móviles con CGNAT o UDP bloqueado, configura un relé TURN con TCP/TLS en la tarjeta Red y compruébalo en Diagnóstico.",
        ["relay.title"] = "TURN local y Cloudflare",
        ["relay.emptySecret"] = "vacío conserva el secreto cifrado",
        ["relay.enable"] = "Activar relé local",
        ["relay.cloudflare"] = "Cloudflare con respaldo local por cuota",
        ["relay.row.localTurn"] = "TURN del equipo",
        ["relay.row.publicIp"] = "IP pública IPv4",
        ["relay.row.listen"] = "Escucha UDP y TCP",
        ["relay.row.tcpAlternate"] = "TCP adicional (opcional)",
        ["relay.row.advertised"] = "URLs del proxy TLS (opcional)",
        ["relay.row.ports"] = "Puertos UDP del relé",
        ["relay.portsHint"] = "{0}–{1} · abrir en router",
        ["relay.row.provider"] = "Proveedor externo",
        ["relay.row.account"] = "ID de cuenta Cloudflare",
        ["relay.row.keyId"] = "ID de clave TURN",
        ["relay.row.keyToken"] = "Token de clave TURN",
        ["relay.row.analytics"] = "Token Account Analytics Read",
        ["relay.row.budget"] = "Corte preventivo TURN (GB/mes)",
        ["relay.note"] = "El consumo se estima para toda la cuenta TURN. La cuota gratuita se comparte con Realtime SFU. Las estadísticas tienen retraso: este corte no garantiza una factura de cero. El equipo debe permanecer encendido y accesible desde internet.",
        ["relay.save"] = "Guardar",
        ["relay.error.invalidIp"] = "Indica una IP pública IPv4 válida.",
        ["relay.error.cloudflareNeedsLocal"] = "Cloudflare con control de cuota requiere activar el respaldo local.",
        ["program.alreadyRunning"] = "StreamWebRom ya está abierto (revisa la bandeja del sistema).",
        ["error.tunnelTokenMissing"] = "Falta el token del túnel de Cloudflare.",
        ["error.invalidTurnConfig"] = "Configuración TURN inválida",
        ["error.localTurnRequires"] = "TURN local requiere IP pública IPv4 y puerto UDP de medios fijo.",
        ["error.cloudflareCredentials"] = "Completa los identificadores y ambos tokens de Cloudflare.",
        ["error.serverNotIncluded"] = "streamwebrom.exe no está incluido en este build.",
    };

    private static readonly Dictionary<string, string> En = new()
    {
        ["common.copy"] = "Copy",
        ["common.optional"] = "optional",
        ["main.tunnel.enable"] = "Publish to the internet with Cloudflare Tunnel",
        ["main.token.placeholder"] = "paste the tunnel token",
        ["main.token.saved"] = "saved token (encrypted) · paste another to replace it",
        ["main.turnUser.placeholder"] = "TURN user",
        ["main.turnPassword.placeholder"] = "TURN password",
        ["main.turnPassword.saved"] = "saved password (encrypted)",
        ["main.https"] = "HTTPS (required for cameras on other computers/phones)",
        ["main.start"] = "Start",
        ["main.stop"] = "Stop",
        ["main.openStudio"] = "Open studio",
        ["main.recordings"] = "Recordings",
        ["main.obsHelp"] = "How to use with OBS",
        ["main.advancedCapture"] = "Advanced capture",
        ["urls.local"] = "This computer",
        ["urls.lan"] = "Your network (phones, other PCs)",
        ["urls.public"] = "Internet (guests and remote OBS)",
        ["urls.tunnelOff"] = "(tunnel off)",
        ["card.links"] = "Studio links",
        ["card.server"] = "Server",
        ["card.network"] = "Network",
        ["card.publish"] = "Publish to the internet",
        ["card.log"] = "Log",
        ["net.port"] = "Web port",
        ["net.udpPort"] = "Media UDP port",
        ["net.publicIp"] = "Public IP",
        ["net.turnRelay"] = "TURN relay (mobile networks)",
        ["net.user"] = "User",
        ["net.password"] = "Password",
        ["net.testConnection"] = "Test connection",
        ["net.relays"] = "Local TURN / Cloudflare",
        ["net.applyRestart"] = "Apply and restart",
        ["tun.domain"] = "Domain",
        ["tun.token"] = "Token",
        ["tun.tokenHelp"] = "Where is the token?",
        ["tray.open"] = "Open studio",
        ["tray.capture"] = "Advanced capture",
        ["tray.show"] = "Show window",
        ["tray.quit"] = "Quit",
        ["tray.balloon"] = "Still running in the tray. Right-click → Quit to close it.",
        ["status.online"] = "● online",
        ["status.onlineHost"] = "● online · {0}",
        ["status.stopped"] = "stopped",
        ["status.stoppedCode"] = "stopped (code {0})",
        ["status.error"] = "error",
        ["log.tunnelPasteToken"] = "Tunnel: paste the Cloudflare token and press “Apply and restart” to publish at {0}",
        ["log.ffmpegMissing"] = "Warning: ffmpeg not found. RTMP multistream will be disabled; OBS, guests and recording work as usual.",
        ["log.tunnelActive"] = "Cloudflare tunnel active → https://{0}",
        ["log.tunnelError"] = "Tunnel: {0}",
        ["log.errorPrefix"] = "Error: {0}",
        ["log.tunnelPrefix"] = "[tunnel] ",
        ["tunnel.downloading"] = "Downloading cloudflared (one time only)…",
        ["tunnel.ready"] = "cloudflared ready.",
        ["help.obs.body"] =
            "1. Open the studio and create your program.\n" +
            "2. OBS tab → copy “Full program”.\n" +
            "3. In OBS: Sources → + → Browser → paste the URL, 1920×1080, tick “Control audio via OBS”.\n\n" +
            "To send OBS to the studio: Settings → Stream → Service WHIP, with the server and Bearer Token from the OBS tab.\n\n" +
            "Multistream to YouTube/Twitch: install ffmpeg (winget install ffmpeg) or copy ffmpeg.exe next to StreamWebRom.exe.",
        ["help.token.body"] =
            "Cloudflare → Zero Trust → Networks → Tunnels → streamwebrom → Configure → copy the token from “Install and run a connector” (the long part after --token).\n\n" +
            "It is stored encrypted with your Windows account and never leaves this PC.\n\n" +
            "The tunnel publishes the web app; audio and video also need reachable UDP or a TURN server. For mobile networks behind CGNAT or with UDP blocked, configure a TURN relay with TCP/TLS in the Network card and check it under Diagnostics.",
        ["relay.title"] = "Local TURN and Cloudflare",
        ["relay.emptySecret"] = "leave empty to keep the encrypted secret",
        ["relay.enable"] = "Enable local relay",
        ["relay.cloudflare"] = "Cloudflare with local quota fallback",
        ["relay.row.localTurn"] = "This computer's TURN",
        ["relay.row.publicIp"] = "Public IPv4",
        ["relay.row.listen"] = "Listen on UDP and TCP",
        ["relay.row.tcpAlternate"] = "Extra TCP (optional)",
        ["relay.row.advertised"] = "TLS proxy URLs (optional)",
        ["relay.row.ports"] = "Relay UDP ports",
        ["relay.portsHint"] = "{0}–{1} · open on the router",
        ["relay.row.provider"] = "External provider",
        ["relay.row.account"] = "Cloudflare account ID",
        ["relay.row.keyId"] = "TURN key ID",
        ["relay.row.keyToken"] = "TURN key token",
        ["relay.row.analytics"] = "Account Analytics Read token",
        ["relay.row.budget"] = "TURN safety cap (GB/month)",
        ["relay.note"] = "Usage is estimated for the whole TURN account. The free quota is shared with Realtime SFU. Statistics lag, so this cap does not guarantee a zero bill. This computer must stay on and reachable from the internet.",
        ["relay.save"] = "Save",
        ["relay.error.invalidIp"] = "Enter a valid public IPv4 address.",
        ["relay.error.cloudflareNeedsLocal"] = "Cloudflare with quota control requires the local fallback to be enabled.",
        ["program.alreadyRunning"] = "StreamWebRom is already open (check the system tray).",
        ["error.tunnelTokenMissing"] = "The Cloudflare tunnel token is missing.",
        ["error.invalidTurnConfig"] = "Invalid TURN configuration",
        ["error.localTurnRequires"] = "Local TURN requires a public IPv4 address and a fixed media UDP port.",
        ["error.cloudflareCredentials"] = "Fill in the identifiers and both Cloudflare tokens.",
        ["error.serverNotIncluded"] = "streamwebrom.exe is not included in this build.",
    };

    private static readonly Dictionary<string, string> Ru = new()
    {
        ["common.copy"] = "Копировать",
        ["common.optional"] = "необязательно",
        ["main.tunnel.enable"] = "Публиковать в интернете через Cloudflare Tunnel",
        ["main.token.placeholder"] = "вставьте токен туннеля",
        ["main.token.saved"] = "токен сохранён (зашифрован) · вставьте другой, чтобы заменить",
        ["main.turnUser.placeholder"] = "пользователь TURN",
        ["main.turnPassword.placeholder"] = "пароль TURN",
        ["main.turnPassword.saved"] = "пароль сохранён (зашифрован)",
        ["main.https"] = "HTTPS (нужен для камер на других компьютерах и телефонах)",
        ["main.start"] = "Запустить",
        ["main.stop"] = "Остановить",
        ["main.openStudio"] = "Открыть студию",
        ["main.recordings"] = "Записи",
        ["main.obsHelp"] = "Как работать с OBS",
        ["main.advancedCapture"] = "Расширенный захват",
        ["urls.local"] = "Этот компьютер",
        ["urls.lan"] = "Ваша сеть (телефоны, другие ПК)",
        ["urls.public"] = "Интернет (гости и удалённый OBS)",
        ["urls.tunnelOff"] = "(туннель отключён)",
        ["card.links"] = "Ссылки студии",
        ["card.server"] = "Сервер",
        ["card.network"] = "Сеть",
        ["card.publish"] = "Публикация в интернет",
        ["card.log"] = "Журнал",
        ["net.port"] = "Веб-порт",
        ["net.udpPort"] = "UDP-порт медиа",
        ["net.publicIp"] = "Публичный IP",
        ["net.turnRelay"] = "TURN-ретранслятор (мобильные сети)",
        ["net.user"] = "Пользователь",
        ["net.password"] = "Пароль",
        ["net.testConnection"] = "Проверить подключение",
        ["net.relays"] = "Локальный TURN / Cloudflare",
        ["net.applyRestart"] = "Применить и перезапустить",
        ["tun.domain"] = "Домен",
        ["tun.token"] = "Токен",
        ["tun.tokenHelp"] = "Где взять токен?",
        ["tray.open"] = "Открыть студию",
        ["tray.capture"] = "Расширенный захват",
        ["tray.show"] = "Показать окно",
        ["tray.quit"] = "Выход",
        ["tray.balloon"] = "Программа продолжает работать в трее. Правый клик → «Выход», чтобы закрыть.",
        ["status.online"] = "● в сети",
        ["status.onlineHost"] = "● в сети · {0}",
        ["status.stopped"] = "остановлен",
        ["status.stoppedCode"] = "остановлен (код {0})",
        ["status.error"] = "ошибка",
        ["log.tunnelPasteToken"] = "Туннель: вставьте токен Cloudflare и нажмите «Применить и перезапустить», чтобы опубликовать на {0}",
        ["log.ffmpegMissing"] = "Внимание: ffmpeg не найден. Мультистрим по RTMP будет отключён; OBS, гости и запись работают как обычно.",
        ["log.tunnelActive"] = "Туннель Cloudflare активен → https://{0}",
        ["log.tunnelError"] = "Туннель: {0}",
        ["log.errorPrefix"] = "Ошибка: {0}",
        ["log.tunnelPrefix"] = "[туннель] ",
        ["tunnel.downloading"] = "Скачиваем cloudflared (один раз)…",
        ["tunnel.ready"] = "cloudflared готов.",
        ["help.obs.body"] =
            "1. Откройте студию и создайте программу.\n" +
            "2. Вкладка OBS → скопируйте «Полная программа».\n" +
            "3. В OBS: Источники → + → Браузер → вставьте URL, 1920×1080, отметьте «Управлять звуком через OBS».\n\n" +
            "Чтобы отправить OBS в студию: Настройки → Трансляция → Сервис WHIP, с сервером и Bearer Token со вкладки OBS.\n\n" +
            "Мультистрим на YouTube/Twitch: установите ffmpeg (winget install ffmpeg) или положите ffmpeg.exe рядом с StreamWebRom.exe.",
        ["help.token.body"] =
            "Cloudflare → Zero Trust → Networks → Tunnels → streamwebrom → Configure → скопируйте токен из «Install and run a connector» (длинная часть после --token).\n\n" +
            "Он хранится зашифрованным под вашей учётной записью Windows и никогда не покидает этот ПК.\n\n" +
            "Туннель публикует веб-интерфейс; для звука и видео ещё нужен доступный UDP или сервер TURN. Для мобильных сетей за CGNAT или с заблокированным UDP настройте TURN-ретранслятор с TCP/TLS в карточке «Сеть» и проверьте его в «Диагностике».",
        ["relay.title"] = "Локальный TURN и Cloudflare",
        ["relay.emptySecret"] = "пустое поле сохраняет зашифрованный секрет",
        ["relay.enable"] = "Включить локальный ретранслятор",
        ["relay.cloudflare"] = "Cloudflare с локальным резервом по квоте",
        ["relay.row.localTurn"] = "TURN этого компьютера",
        ["relay.row.publicIp"] = "Публичный IPv4",
        ["relay.row.listen"] = "Слушать UDP и TCP",
        ["relay.row.tcpAlternate"] = "Дополнительный TCP (необязательно)",
        ["relay.row.advertised"] = "URL TLS-прокси (необязательно)",
        ["relay.row.ports"] = "UDP-порты ретранслятора",
        ["relay.portsHint"] = "{0}–{1} · открыть на роутере",
        ["relay.row.provider"] = "Внешний провайдер",
        ["relay.row.account"] = "ID аккаунта Cloudflare",
        ["relay.row.keyId"] = "ID ключа TURN",
        ["relay.row.keyToken"] = "Токен ключа TURN",
        ["relay.row.analytics"] = "Токен Account Analytics Read",
        ["relay.row.budget"] = "Лимит TURN (ГБ/мес)",
        ["relay.note"] = "Расход оценивается по всему аккаунту TURN. Бесплатная квота общая с Realtime SFU. Статистика приходит с задержкой: этот лимит не гарантирует нулевой счёт. Компьютер должен оставаться включённым и доступным из интернета.",
        ["relay.save"] = "Сохранить",
        ["relay.error.invalidIp"] = "Укажите корректный публичный IPv4-адрес.",
        ["relay.error.cloudflareNeedsLocal"] = "Для Cloudflare с контролем квоты нужно включить локальный резерв.",
        ["program.alreadyRunning"] = "StreamWebRom уже запущен (проверьте системный трей).",
        ["error.tunnelTokenMissing"] = "Не указан токен туннеля Cloudflare.",
        ["error.invalidTurnConfig"] = "Некорректная конфигурация TURN",
        ["error.localTurnRequires"] = "Локальному TURN нужен публичный IPv4 и фиксированный UDP-порт медиа.",
        ["error.cloudflareCredentials"] = "Заполните идентификаторы и оба токена Cloudflare.",
        ["error.serverNotIncluded"] = "streamwebrom.exe не входит в эту сборку.",
    };

    private static readonly Dictionary<string, string> Current;

    static Lang()
    {
        // First the process UI culture; if globalization is invariant (the
        // launcher sets InvariantGlobalization=true), ask Windows directly.
        Current = Pick(CultureInfo.CurrentUICulture.TwoLetterISOLanguageName)
                  ?? Pick(OsUiLanguage())
                  ?? Es;
    }

    private static Dictionary<string, string>? Pick(string code) => code switch
    {
        "ru" => Ru,
        "en" => En,
        "es" => Es,
        _ => null,
    };

    private static string OsUiLanguage()
    {
        try
        {
            return (GetUserDefaultUILanguage() & 0x3ff) switch // primary language id
            {
                0x19 => "ru",
                0x09 => "en",
                0x0a => "es",
                _ => "",
            };
        }
        catch { return ""; }
    }

    [System.Runtime.InteropServices.DllImport("kernel32.dll")]
    private static extern ushort GetUserDefaultUILanguage();

    public static string T(string key) =>
        Current.TryGetValue(key, out var value) ? value : Es.TryGetValue(key, out var fallback) ? fallback : key;

    public static string T(string key, params object[] args) => string.Format(T(key), args);
}
