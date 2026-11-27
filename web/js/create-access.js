// Access code for POST /api/rooms when the server runs with -create-token.
//
// The page owns the DOM; this module owns the rules: when the field must be
// visible, what a valid code looks like and how it travels. The code lives
// only in that input: never in storage, the URL or any room payload.

// The server accepts this exact set. Keeping the same rule here means the page
// refuses a value the browser could not transmit in an Authorization header.
export const CREATE_TOKEN_RE = /^[A-Za-z0-9_-]{16,128}$/;

// VDO.Ninja rooms are created by another service, so they never need the code.
export function codeVisible(createProtected, engine) {
  return !!createProtected && engine !== "vdo";
}

export function validCreateCode(code) {
  return CREATE_TOKEN_RE.test(String(code == null ? "" : code));
}

export function createHeaders(code) {
  const token = String(code == null ? "" : code).trim();
  return token ? { Authorization: "Bearer " + token } : {};
}
