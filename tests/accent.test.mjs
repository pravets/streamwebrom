import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';

const { accentApplies, ACCENT_LAYOUTS, quickAccent } = await import(await moduleURL('compositor.js'));

test('the accent only takes effect for a source that is in the scene', () => {
  assert.equal(accentApplies('spotlight', ['a', 'b'], 'a'), true);
  assert.equal(accentApplies('spotlight', ['b'], 'a'), false, 'a source outside the scene must not steal the spotlight');
  assert.equal(accentApplies('solo', ['b'], 'a'), false);
});

test('the accent only exists in layouts with a spotlight slot', () => {
  for (const layout of ['pip', 'sidebar', 'spotlight', 'solo']) {
    assert.equal(accentApplies(layout, ['a'], 'a'), true, layout);
  }
  for (const layout of ['grid', 'split', 'screen', 'news']) {
    assert.equal(accentApplies(layout, ['a'], 'a'), false, layout);
  }
  assert.deepEqual([...ACCENT_LAYOUTS].sort(), ['pip', 'sidebar', 'solo', 'spotlight']);
});

test('no featured source means the order is untouched', () => {
  assert.equal(accentApplies('spotlight', ['a'], ''), false);
  assert.equal(accentApplies('spotlight', null, 'a'), false);
});

test('quickAccent adds a missing source to the stage and features it', () => {
  const scene = { stage: ['b'], layout: 'spotlight', featured: '' };
  const next = quickAccent(scene, 'a');
  assert.deepEqual(next.stage, ['b', 'a']);
  assert.equal(next.featured, 'a');
  assert.deepEqual(scene.stage, ['b'], 'the input scene must not be mutated');
  assert.equal(scene.featured, '');
});

test('quickAccent fills in defaults for an empty scene', () => {
  const next = quickAccent({}, 'a');
  assert.deepEqual(next.stage, ['a']);
  assert.equal(next.featured, 'a');
  assert.equal(next.layout, 'spotlight');
});

test('quickAccent switches to spotlight only when the layout has no accent slot', () => {
  for (const layout of ['grid', 'split', 'screen', 'news']) {
    const next = quickAccent({ stage: ['a'], layout, featured: '' }, 'a');
    assert.equal(next.layout, 'spotlight', layout);
    assert.equal(next.featured, 'a');
  }
  for (const layout of ACCENT_LAYOUTS) {
    const next = quickAccent({ stage: ['a'], layout, featured: '' }, 'a');
    assert.equal(next.layout, layout, `accent layout ${layout} is kept`);
  }
});

test('a second click removes the accent without touching the stage', () => {
  const scene = { stage: ['a', 'b'], layout: 'grid', featured: '' };
  const on = quickAccent(scene, 'a');
  const off = quickAccent(on, 'a');
  assert.equal(off.featured, '');
  assert.deepEqual(off.stage, ['a', 'b'], 'removing the accent keeps the line-up');
  assert.equal(off.layout, 'spotlight', 'removing the accent keeps the layout');
});

test('quickAccent never duplicates a stage entry and toggles idempotently', () => {
  const scene = { stage: ['a'], layout: 'pip', featured: 'b' };
  const once = quickAccent(scene, 'a');
  assert.deepEqual(once.stage, ['a'], 'already-present source is not added twice');
  assert.equal(once.featured, 'a');
  const twice = quickAccent(once, 'a');
  assert.equal(twice.featured, '');
  const thrice = quickAccent(twice, 'a');
  assert.deepEqual(thrice, once, 'click on/off/on returns to the same state');
});

test('a featured the layout ignores is not active: one click applies the accent', () => {
  const scene = { stage: ['a', 'b'], layout: 'grid', featured: 'a' };
  const next = quickAccent(scene, 'a');
  assert.equal(next.featured, 'a');
  assert.equal(next.layout, 'spotlight', 'the first click must apply, not silently clear the hidden featured');
  assert.deepEqual(next.stage, ['a', 'b'], 'the line-up is kept');
  assert.equal(accentApplies(next.layout, next.stage, next.featured), true);
  const off = quickAccent(next, 'a');
  assert.equal(off.featured, '', 'only an applied accent toggles off');
  assert.equal(off.layout, 'spotlight');
});

test('a featured source missing from the stage is re-added in one click', () => {
  const scene = { stage: ['b'], layout: 'spotlight', featured: 'a' };
  assert.equal(accentApplies(scene.layout, scene.stage, scene.featured), false);
  const next = quickAccent(scene, 'a');
  assert.deepEqual(next.stage, ['b', 'a']);
  assert.equal(next.featured, 'a');
  assert.equal(next.layout, 'spotlight', 'an accent layout is kept');
});
