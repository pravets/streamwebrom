import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';

// ---- minimal browser mocks: node has no DOM, screen or matchMedia ----
const classes = () => {
  const set = new Set();
  return {
    toggle(c, on) { const want = on === undefined ? !set.has(c) : !!on; if (want) set.add(c); else set.delete(c); return want; },
    add(c) { set.add(c); }, remove(c) { set.delete(c); }, contains(c) { return set.has(c); },
  };
};
const node = () => ({ classList: classes(), setAttribute() {}, blur() {}, contains: () => false, addEventListener() {}, removeEventListener() {} });
const doc = new EventTarget();
doc.fullscreenElement = null; doc.activeElement = null;
doc.exitFullscreen = async () => { doc.fullscreenElement = null; };
globalThis.document = doc;
globalThis.window = new EventTarget();

const orientation = await import(await moduleURL('orientation.js'));
const { immersive } = await import(await moduleURL('immersive.js'));

test('orientationSupported detects the lock API (iOS has orientation but no lock)', () => {
  delete globalThis.screen;
  assert.equal(orientation.orientationSupported(), false);
  globalThis.screen = {};
  assert.equal(orientation.orientationSupported(), false);
  globalThis.screen = { orientation: {} };
  assert.equal(orientation.orientationSupported(), false);
  globalThis.screen = { orientation: { lock: async () => {} } };
  assert.equal(orientation.orientationSupported(), true);
});

test('lockLandscape locks landscape and swallows NotSupportedError/SecurityError', async () => {
  const calls = [];
  globalThis.screen = { orientation: { lock: async (o) => { calls.push(o); }, unlock: () => { calls.push('unlock'); } } };
  assert.equal(await orientation.lockLandscape(), true);
  assert.deepEqual(calls, ['landscape']);
  orientation.unlockLandscape();
  assert.deepEqual(calls, ['landscape', 'unlock']);
  for (const name of ['NotSupportedError', 'SecurityError']) {
    globalThis.screen = { orientation: { lock: async () => { const e = new Error('nope'); e.name = name; throw e; } } };
    assert.equal(await orientation.lockLandscape(), false);
  }
  globalThis.screen = {};
  assert.equal(await orientation.lockLandscape(), false);
  orientation.unlockLandscape(); // must not throw without the API
});

test('immersive locks orientation only after fullscreen succeeds and unlocks on exit', async () => {
  const order = [];
  const root = node(), button = node(), exit = node();
  root.requestFullscreen = async () => { order.push('fullscreen'); doc.fullscreenElement = root; };
  globalThis.screen = { orientation: { lock: async () => { order.push('lock'); }, unlock: () => { order.push('unlock'); } } };
  const cinema = immersive(root, button, exit, {
    onFullscreen: () => orientation.lockLandscape(),
    onExitFullscreen: () => orientation.unlockLandscape(),
  });
  await cinema.enter();
  assert.deepEqual(order, ['fullscreen', 'lock']);
  // Leaving fullscreen (Esc or the exit button) releases the lock.
  doc.fullscreenElement = null;
  doc.dispatchEvent(new Event('fullscreenchange'));
  assert.deepEqual(order, ['fullscreen', 'lock', 'unlock']);
  cinema.destroy();
  doc.fullscreenElement = null;
});

test('a stale fullscreenchange during entry cannot cancel the new cinema session', async () => {
  const order = [];
  const root = node(), button = node(), exit = node();
  let release;
  root.requestFullscreen = () => new Promise((resolve) => { release = () => { doc.fullscreenElement = root; resolve(); }; });
  globalThis.screen = { orientation: { lock: async () => { order.push('lock'); }, unlock: () => { order.push('unlock'); } } };
  const cinema = immersive(root, button, exit, {
    onFullscreen: () => orientation.lockLandscape(),
    onExitFullscreen: () => orientation.unlockLandscape(),
  });
  const entering = cinema.enter();
  // The exit of an earlier session lands while the new request is in flight.
  doc.fullscreenElement = null;
  doc.dispatchEvent(new Event('fullscreenchange'));
  assert.equal(root.classList.contains('cinema'), true, 'entry must survive a stale event');
  release();
  await entering;
  assert.deepEqual(order, ['lock'], 'the newer entry still locks orientation');
  assert.equal(root.classList.contains('cinema'), true);
  cinema.destroy();
  doc.fullscreenElement = null;
  order.length = 0;
});

test('a real exit after the entry settled still leaves cinema mode', async () => {
  const order = [];
  const root = node(), button = node(), exit = node();
  root.requestFullscreen = async () => { doc.fullscreenElement = root; };
  globalThis.screen = { orientation: { lock: async () => { order.push('lock'); }, unlock: () => { order.push('unlock'); } } };
  const cinema = immersive(root, button, exit, {
    onFullscreen: () => orientation.lockLandscape(),
    onExitFullscreen: () => orientation.unlockLandscape(),
  });
  await cinema.enter();
  doc.fullscreenElement = null;
  doc.dispatchEvent(new Event('fullscreenchange'));
  assert.deepEqual(order, ['lock', 'unlock']);
  assert.equal(root.classList.contains('cinema'), false);
  cinema.destroy();
});

test('a rejected fullscreen keeps cinema mode and never locks orientation', async () => {
  const order = [];
  const root = node(), button = node(), exit = node();
  root.requestFullscreen = async () => { throw new Error('denied'); };
  globalThis.screen = { orientation: { lock: async () => { order.push('lock'); }, unlock: () => {} } };
  const cinema = immersive(root, button, exit, { onFullscreen: () => orientation.lockLandscape() });
  await cinema.enter();
  assert.equal(root.classList.contains('cinema'), true);
  assert.deepEqual(order, []);
  cinema.destroy();
});

test('cinema without orientation hooks (studio usage) keeps working', async () => {
  const calls = [];
  const root = node(), button = node(), exit = node();
  root.requestFullscreen = async () => { calls.push('fullscreen'); doc.fullscreenElement = root; };
  const cinema = immersive(root, button, exit, { native: false });
  await cinema.enter(); // CSS-only cinema: no fullscreen request
  assert.equal(root.classList.contains('cinema'), true);
  assert.deepEqual(calls, []);
  await cinema.enter(true); // studio's native fullscreen button
  assert.deepEqual(calls, ['fullscreen']);
  cinema.destroy();
  doc.fullscreenElement = null;
});

test('without the lock API a portrait hint appears only while landscape mode is on', () => {
  globalThis.screen = { orientation: {} }; // iOS-like: no lock
  const mq = new EventTarget(); mq.matches = true;
  window.matchMedia = (q) => { assert.equal(q, '(orientation: portrait)'); return mq; };
  const hint = node();
  const ctl = orientation.installOrientationHint(hint);
  assert.equal(hint.classList.contains('hidden'), true); // mode off
  ctl.setEnabled(true);
  assert.equal(hint.classList.contains('hidden'), false); // portrait + no API -> show
  mq.matches = false; mq.dispatchEvent(new Event('change'));
  assert.equal(hint.classList.contains('hidden'), true); // landscape -> hide
  mq.matches = true; mq.dispatchEvent(new Event('change'));
  assert.equal(hint.classList.contains('hidden'), false);
  ctl.setEnabled(false);
  assert.equal(hint.classList.contains('hidden'), true);
  ctl.destroy();
  mq.dispatchEvent(new Event('change')); // listener removed: stays hidden
  assert.equal(hint.classList.contains('hidden'), true);
});

test('browsers that can lock orientation never show the rotate hint', () => {
  globalThis.screen = { orientation: { lock: async () => {} } };
  const mq = new EventTarget(); mq.matches = true;
  window.matchMedia = () => mq;
  const hint = node();
  const ctl = orientation.installOrientationHint(hint);
  ctl.setEnabled(true);
  assert.equal(hint.classList.contains('hidden'), true);
  ctl.destroy();
});