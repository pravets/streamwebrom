# Studio interface (1.10.0)

[Español](../INTERFAZ.md) · **English**

**Save room** is in the host's top bar. It also opens from **Room**, in the right rail. The dialog shows whether the room is temporary or persistent, the required password, expiry, saved personal/OBS links and the host's private access. Saving changes the button to Saved room; saving again lets you renew the term. It is not hidden in Brand. If the room has no password, the requirement is explained and it cannot be saved.

The **Visual theme** selector offers Day, Night and Automatic. It is stored in this browser, synchronised between its tabs and, in Automatic, follows the system appearance. It works on the landing page, the studio, guest entry, call settings and diagnostics; it does not change the broadcast branding and does not apply filters to cameras or OBS outputs.

**Collapse panels:** press the Scenes arrow to collapse the left column and its return button to open it. The × of the right panel leaves the tool rail visible; press any tool to reopen its panel. You can also use View → Show/Hide scenes or Show/Hide panel. Each side is controlled separately and the preference is kept in this browser.

View also groups Stage only, full screen and latency display. The program keeps the 16:9 frame, adapts to the available space and retains its production options. On phones the header is arranged in two rows and Save room opens the dialog without scrolling through the panels.

The landing page separates Saved rooms from Recent studios. The configuration uses no graphics library and no remote fonts: CSS variables for colours, local preferences and a ResizeObserver to fit the stage. No tracks, video workers or timers are created by changing the theme or collapsing panels.

**Chat** has a General conversation for the whole room plus one direct conversation per participant, with unread badges on the conversation switcher. The ✉ button in People (studio) or on a participant's tile (guest) opens the direct conversation. The host can delete any message (🗑) and chooses who may send direct messages in Brand → Direct messages: Everyone, Host only (guests may only write to a host) or Off. With Off the general chat keeps working and the direct composer is replaced by a note. History lives in the room's memory: it is restored on reconnect (guests keep their conversations through the rejoin pass; a reconnecting host gets a new peer id, so their direct history starts empty), and it is lost when the room closes or the server restarts.

**Chat server requirement:** direct messages, message deletion (🗑), the Direct messages policy and the restore of direct history on reconnect require the server-side chat implementation from `feat/chat-server` (PR #24), which is merged and deployed before this client. The server bundled with this revision only handles the General chat and its history; without the chat server the direct-message and deletion controls have no effect.

Product references reviewed:

- [StreamYard: light, dark and automatic appearance](https://support.streamyard.com/hc/en-us/articles/31567252131732-Does-StreamYard-have-a-Dark-Mode).
- [StreamYard: independent collapsible panels](https://new.streamyard.com/did-you-know-you-can-collapse-the-sidebar-for-improved-focus-44xMeA).
- [StreamYard: reusable studios](https://support.streamyard.com/hc/en-us/articles/30110539821076-Reusable-Studios).
