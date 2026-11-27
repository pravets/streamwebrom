import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';

// chat.js imports rtc.js, which reads location at module scope.
globalThis.location = { search: '', hash: '' };
const { GENERAL, conversationKey, dmPeer, unreadAfter, unreadOnShow, messageKey, unreadAfterSeed, seenAfterRemove, unreadAfterRemove } = await import(await moduleURL('chat.js'));

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

test('showing a conversation clears its badge only when the pane is visible', () => {
  const state = { [GENERAL]: 2, 'ana|me': 3 };
  // Visible: opening a conversation marks exactly that one as read.
  assert.deepEqual(unreadOnShow(state, GENERAL, true), { 'ana|me': 3 });
  assert.deepEqual(unreadOnShow(state, 'ana|me'), { [GENERAL]: 2 }, 'visibility defaults to true');
  // Hidden: the counter survives until the user actually looks at the pane.
  assert.deepEqual(unreadOnShow(state, GENERAL, false), state);
  // Pure and idempotent: no mutation, and an absent key changes nothing.
  assert.deepEqual(state, { [GENERAL]: 2, 'ana|me': 3 });
  assert.equal(unreadOnShow(state, 'bob|me', true), state);
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

test('messageKey maps an envelope to its conversation in one place', () => {
  assert.equal(messageKey({ type: 'chat', peerId: 'ana' }), GENERAL);
  assert.equal(messageKey({ type: 'dm', peerId: 'ana', to: 'me' }), conversationKey('ana', 'me'));
  assert.equal(messageKey({ type: 'dm', peerId: 'me', to: 'ana' }), conversationKey('ana', 'me'));
});

test('unread survives a reconnect seed without resurrecting old history', () => {
  const seen = new Map([[GENERAL, 'c1'], ['ana|me', 'd1']]);
  const histories = new Map([
    [GENERAL, [{ id: 'c1', peerId: 'ana' }, { id: 'c2', peerId: 'ana' }, { id: 'c3', peerId: 'me' }]],
    ['ana|me', [{ id: 'd1', peerId: 'ana' }]],
    ['bob|me', [{ id: 'e1', peerId: 'bob' }]],
  ]);
  // New messages after the last seen one badge; my own message never counts.
  assert.deepEqual(unreadAfterSeed(histories, seen, 'me'), { [GENERAL]: 1 });
  // The open, visible conversation is read at once; a hidden pane keeps it.
  assert.deepEqual(unreadAfterSeed(histories, seen, 'me', GENERAL), {});
  assert.deepEqual(unreadAfterSeed(histories, seen, 'me', GENERAL, false), { [GENERAL]: 1 });
  // No known read point (first welcome / dm never opened): nothing is "new".
  assert.deepEqual(unreadAfterSeed(histories, new Map(), 'me'), {});
  // A read marker trimmed out of the 300-message window counts from the start.
  assert.deepEqual(unreadAfterSeed(new Map([[GENERAL, [{ id: 'x2', peerId: 'ana' }]]]), seen, 'me'), { [GENERAL]: 1 });
  // A conversation this client never opened keeps the badge it already tracked
  // (a dm that arrived while the pane was hidden) instead of losing it, and the
  // whole restored history is still not counted as new.
  assert.deepEqual(unreadAfterSeed(histories, new Map(), 'me', GENERAL, false, { [GENERAL]: 2 }), { [GENERAL]: 2 });
  assert.deepEqual(unreadAfterSeed(histories, new Map(), 'me', GENERAL, true, { [GENERAL]: 2 }), {},
    'a visible open conversation is read at once');
  assert.deepEqual(unreadAfterSeed(histories, new Map(), 'me', GENERAL, false), {},
    'nothing tracked and nothing read: no badge');
  // Pure: the inputs are not mutated.
  assert.equal(seen.get(GENERAL), 'c1');
});

test('a deleted message also moves the read marker that pointed at it', () => {
  const list = [{ id: 'c1' }, { id: 'c2' }, { id: 'c3' }];
  // Removing the marked message at index 1 (c2) steps back to c1: the next
  // snapshot must not treat already-read history as new.
  assert.equal(seenAfterRemove('c2', 'c2', list, 1), 'c1');
  // Removing the first message drops the marker, so the tracked counter is
  // kept instead of the whole history turning "new".
  assert.equal(seenAfterRemove('c1', 'c1', list, 0), null);
  // A marker on another message is untouched.
  assert.equal(seenAfterRemove('c3', 'c1', list, 0), 'c3');
  assert.equal(seenAfterRemove(undefined, 'c1', list, 0), undefined);
});

test('deleting an unread message also drops its badge', () => {
  const mine = 'me';
  // The only dm of a hidden conversation is deleted: nothing is unread now.
  assert.equal(unreadAfterRemove([], undefined, mine, 1), 0);
  // Several unread messages: the counter loses just the removed one.
  assert.equal(unreadAfterRemove([{ id: 'a' }], undefined, mine, 3), 2);
  // With a read marker the count is rebuilt from the remaining messages.
  assert.equal(unreadAfterRemove([{ id: 'a', peerId: 'ana' }, { id: 'b', peerId: 'ana' }], 'a', mine, 5), 1);
  // Own echoes never count.
  assert.equal(unreadAfterRemove([{ id: 'a' }, { id: 'b', peerId: mine }], 'a', mine, 2), 0);
  // A marker that is gone counts the whole remaining list, as in seed().
  assert.equal(unreadAfterRemove([{ id: 'a', peerId: 'ana' }], 'gone', mine, 1), 1);
});

test('removing my own echo keeps the peer message unread', () => {
  const mine = 'me';
  // Hidden conversation without a marker: incoming `a` plus my echo `b`.
  // Deleting `b` must not clear the badge that belongs to `a`.
  assert.equal(unreadAfterRemove([{ id: 'a', peerId: 'ana' }], undefined, mine, 1, { id: 'b', peerId: mine }), 1);
  // Deleting the peer's own unread message still drops the counter.
  assert.equal(unreadAfterRemove([], undefined, mine, 1, { id: 'a', peerId: 'ana' }), 0);
  // The author is optional: callers that do not pass it keep the old behaviour.
  assert.equal(unreadAfterRemove([], undefined, mine, 2), 1);
});