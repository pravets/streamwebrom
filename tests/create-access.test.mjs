import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';

const access = await import(await moduleURL('create-access.js'));

test('the code field follows the server flag and the engine', () => {
  // An open server (empty SWR_CREATE_TOKEN) never shows the field.
  assert.equal(access.codeVisible(false, 'local'), false);
  assert.equal(access.codeVisible(undefined, 'local'), false);
  // A protected server shows it only for rooms this server creates.
  assert.equal(access.codeVisible(true, 'local'), true);
  assert.equal(access.codeVisible(true, 'vdo'), false);
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
