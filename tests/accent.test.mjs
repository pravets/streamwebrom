import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';

const { accentApplies, ACCENT_LAYOUTS } = await import(await moduleURL('compositor.js'));

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
