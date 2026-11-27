import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';

const access = await import(await moduleURL('create-access.js'));

test('an unknown server state is not treated as open', () => {
  const loading = access.initialCreateState();
  assert.equal(loading.status, access.states.LOADING);
  assert.equal(access.creationReady(loading, 'local'), false);
  assert.equal(access.codeVisible(loading, 'local'), false);
  // VDO.Ninja rooms are created by another service and stay available.
  assert.equal(access.creationReady(loading, 'vdo'), true);
});

test('/api/config decides the field and the button', () => {
  const open = access.configLoaded({ createProtected: false });
  assert.equal(access.creationReady(open, 'local'), true);
  assert.equal(access.codeVisible(open, 'local'), false);

  const protectedState = access.configLoaded({ createProtected: true });
  assert.equal(access.creationReady(protectedState, 'local'), true);
  assert.equal(access.codeVisible(protectedState, 'local'), true);
  // A VDO room is not created by this server: no code and no block.
  assert.equal(access.codeVisible(protectedState, 'vdo'), false);

  // A server too old to send the flag is open, as it was before the feature.
  assert.equal(access.creationReady(access.configLoaded({}), 'local'), true);
  assert.equal(access.codeVisible(access.configLoaded({}), 'local'), false);
});

test('a failed config never silently opens the form', () => {
  const failedOpen = access.configFailed(access.configLoaded({ createProtected: false }));
  assert.equal(failedOpen.status, access.states.ERROR);
  assert.equal(access.creationReady(failedOpen, 'local'), false);
  assert.equal(access.configErrorVisible(failedOpen, 'local'), true);
  assert.equal(access.configErrorVisible(failedOpen, 'vdo'), false);

  const failedProtected = access.configFailed(access.configLoaded({ createProtected: true }));
  assert.equal(access.creationReady(failedProtected, 'local'), false);
  assert.equal(access.codeVisible(failedProtected, 'local'), true);
});

test('a forbidden creation reveals the code even after a failed config', () => {
  const forbidden = access.configForbidden();
  assert.equal(forbidden.status, access.states.READY);
  assert.equal(forbidden.protected, true);
  assert.equal(access.codeVisible(forbidden, 'local'), true);
  assert.equal(access.creationReady(forbidden, 'local'), true);
  assert.equal(access.codeVisible(forbidden, 'vdo'), false);
});


test('the code field follows the server flag and the engine', () => {
  // An open server (empty SWR_CREATE_TOKEN) never shows the field.
  assert.equal(access.codeVisible(access.configLoaded({ createProtected: false }), 'local'), false);
  assert.equal(access.codeVisible(access.configLoaded(undefined), 'local'), false);
  // A protected server shows it only for rooms this server creates.
  assert.equal(access.codeVisible(access.configLoaded({ createProtected: true }), 'local'), true);
  assert.equal(access.codeVisible(access.configLoaded({ createProtected: true }), 'vdo'), false);
});

test('the code rule matches what the browser can send and the server accepts', () => {
  assert.equal(access.validCreateCode('a'.repeat(16)), true);
  assert.equal(access.validCreateCode('A_b-'.repeat(32)), true); // 128 exact
  assert.equal(access.validCreateCode('a'.repeat(15)), false);
  assert.equal(access.validCreateCode('a'.repeat(129)), false);
  assert.equal(access.validCreateCode('a'.repeat(15) + ' '), false);
  assert.equal(access.validCreateCode('a'.repeat(15) + 'й'), false);
  assert.equal(access.validCreateCode(''), false);
  assert.equal(access.validCreateCode(null), false);
  assert.equal(access.validCreateCode(undefined), false);
});

test('the code travels in a header and only when present', () => {
  assert.deepEqual(access.createHeaders(''), {});
  assert.deepEqual(access.createHeaders(null), {});
  assert.deepEqual(access.createHeaders(undefined), {});
  assert.deepEqual(access.createHeaders('  access-code-123456  '), { Authorization: 'Bearer access-code-123456' });
  assert.deepEqual(access.createHeaders('access-code-123456'), { Authorization: 'Bearer access-code-123456' });
});
