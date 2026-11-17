// Access code for POST /api/rooms when the server runs with -create-token.
//
// The page owns the DOM; this module owns the rules: when the field must be
// visible, when creation may start, what a valid code looks like, and how a
// failed /api/config or a 401 must change the state. The code lives only in
// that input: never in storage, the URL or any room payload.

// The server accepts this exact set. Keeping the same rule here means the page
// refuses a value the browser could not transmit in an Authorization header.
export const CREATE_TOKEN_RE = /^[A-Za-z0-9_-]{16,128}$/;

export const states = { LOADING: "loading", READY: "ready", ERROR: "error" };

export function initialCreateState() {
  return { status: states.LOADING, protected: false };
}

export function configLoaded(cfg) {
  return { status: states.READY, protected: !!(cfg && cfg.createProtected) };
}

export function configFailed(state) {
  // An unreachable config must not look like "open": keep the last answer and
  // report the failure so the page can offer a retry.
  return { status: states.ERROR, protected: !!(state && state.protected) };
}

export function configForbidden() {
  // The server demanded a code, so show the field even if /api/config failed.
  return { status: states.READY, protected: true };
}

export function codeVisible(state, engine) {
  return !!state && state.protected && engine !== "vdo";
}

export function creationReady(state, engine) {
  return engine === "vdo" || (!!state && state.status === states.READY);
}

export function configErrorVisible(state, engine) {
  return engine !== "vdo" && !!state && state.status === states.ERROR;
}

export function validCreateCode(code) {
  return CREATE_TOKEN_RE.test(String(code == null ? "" : code));
}

export function createHeaders(code) {
  const token = String(code == null ? "" : code).trim();
  return token ? { Authorization: "Bearer " + token } : {};
}
