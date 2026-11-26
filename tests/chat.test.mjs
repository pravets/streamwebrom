import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';

// chat.js imports rtc.js, which reads location at module scope.
globalThis.location = { search: '', hash: '' };
const { GENERAL, conversationKey, dmPeer, unreadAfter } = await import(await moduleURL('chat.js'));

test('conversationKey is the sorted pair, symmetric and never the general key', () => {
  assert.equal(conversationKey('bob', 'ana'), conversationKey('ana', 'bob'));
  assert.equal(conversationKey('ana', 'bob'), 'ana|bob');
  assert.notEqual(conversationKey('ana', 'bob'), GENERAL);
  assert.notEqual(conversationKey('ana', 'ana'), GENERAL);
});

test('dmPeer resolves the other side for both the delivery and the echo', () => {
  const incoming = { type: 'dm', peerId: 'host', to: 'me' };
  const echo = { type: 'dm', peerId: 'me', to: 'host' };
  assert.equal(dmPeer(incoming, 'me'), 'host');
  assert.equal(dmPeer(echo, 'me'), 'host');
  assert.equal(conversationKey(incoming.peerId, incoming.to), conversationKey(echo.peerId, echo.to));
});

test('unreadAfter counts per conversation and skips the open one', () => {
  const chat = { type: 'chat', peerId: 'ana', comment: { peerId: 'ana', text: 'hola' } };
  const dm = { type: 'dm', peerId: 'ana', to: 'me', comment: { peerId: 'ana', text: 'psst' } };
  let s = {};
  s = unreadAfter(s, chat, GENERAL, 'me');
  assert.deepEqual(s, {}, 'the open general chat never badges');
  s = unreadAfter(s, chat, 'ana|me', 'me');
  assert.deepEqual(s, { [GENERAL]: 1 });
  s = unreadAfter(s, dm, GENERAL, 'me');
  assert.deepEqual(s, { [GENERAL]: 1, 'ana|me': 1 });
  s = unreadAfter(s, dm, 'ana|me', 'me');
  assert.deepEqual(s, { [GENERAL]: 1, 'ana|me': 1 }, 'the open dm never badges');
});

test('own echoes never count as unread, even in a closed conversation', () => {
  const echo = { type: 'dm', peerId: 'me', to: 'ana', comment: { peerId: 'me', text: 'hi' } };
  const mineChat = { type: 'chat', peerId: 'me', comment: { peerId: 'me', text: 'hi all' } };
  assert.deepEqual(unreadAfter({}, echo, GENERAL, 'me'), {});
  assert.deepEqual(unreadAfter({}, mineChat, 'ana|me', 'me'), {});
});

test('separate dm conversations accumulate independently and the reducer is pure', () => {
  const from = (id) => ({ type: 'dm', peerId: id, to: 'me', comment: { peerId: id } });
  let s = {};
  s = unreadAfter(s, from('ana'), GENERAL, 'me');
  s = unreadAfter(s, from('bob'), GENERAL, 'me');
  s = unreadAfter(s, from('ana'), GENERAL, 'me');
  assert.deepEqual(s, { 'ana|me': 2, 'bob|me': 1 });
  const before = s;
  const next = unreadAfter(s, from('bob'), GENERAL, 'me');
  assert.equal(s, before, 'input state must not be mutated');
  assert.deepEqual(next, { 'ana|me': 2, 'bob|me': 2 });
  assert.notEqual(next, before);
});

test('a dm echo and its delivery share one conversation key', () => {
  const delivery = { type: 'dm', peerId: 'ana', to: 'me', comment: { peerId: 'ana' } };
  const echo = { type: 'dm', peerId: 'me', to: 'ana', comment: { peerId: 'me' } };
  const key = (m) => conversationKey(m.peerId, m.to);
  assert.equal(key(delivery), key(echo));
  let s = unreadAfter({}, delivery, GENERAL, 'me');
  s = unreadAfter(s, echo, GENERAL, 'me');
  assert.deepEqual(s, { 'ana|me': 1 }, 'the echo adds nothing');
});

test('a hidden panel never marks the open conversation as read', () => {
  const chat = { type: 'chat', peerId: 'ana', comment: { peerId: 'ana', text: 'hola' } };
  const dm = { type: 'dm', peerId: 'ana', to: 'me', comment: { peerId: 'ana', text: 'psst' } };
  // Closed guest drawer / inactive studio tab: even the open conversation badges.
  let s = unreadAfter({}, chat, GENERAL, 'me', false);
  assert.deepEqual(s, { [GENERAL]: 1 }, 'general chat badges while the drawer is closed');
  s = unreadAfter(s, dm, 'ana|me', 'me', false);
  assert.deepEqual(s, { [GENERAL]: 1, 'ana|me': 1 }, 'the last selected dm badges too');
  // Visible again: the open conversation is skipped; own echoes still never count.
  s = unreadAfter(s, chat, GENERAL, 'me', true);
  assert.deepEqual(s, { [GENERAL]: 1, 'ana|me': 1 });
  assert.deepEqual(unreadAfter({}, chat, GENERAL, 'me'), {}, 'visibility defaults to true');
  const echo = { type: 'dm', peerId: 'me', to: 'ana', comment: { peerId: 'me', text: 'hi' } };
  assert.deepEqual(unreadAfter({}, echo, GENERAL, 'me', false), {}, 'own echo never badges, hidden or not');
});