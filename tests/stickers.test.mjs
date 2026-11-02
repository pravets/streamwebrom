import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';

const { STICKERS } = await import(await moduleURL('stickers.js'));
const core = await import(await moduleURL('i18n.js'));

test('sticker ids stay in sync with the server allow-list', async () => {
  const source = await readFile(new URL('../internal/studio/chat.go', import.meta.url), 'utf8');
  const block = source.slice(source.indexOf('stickerIDs'));
  const serverIds = [...block.matchAll(/"([a-z]+)"/g)].map((match) => match[1]).slice(0, 32);
  for (const sticker of STICKERS) assert.ok(serverIds.includes(sticker.id), `${sticker.id} is not accepted by the server`);
  assert.equal(STICKERS.length, 16);
});

test('every sticker reads in all three languages', () => {
  for (const sticker of STICKERS) {
    for (const lang of core.LANGS) {
      const text = core.translator(lang)('stickers.' + sticker.id);
      assert.notEqual(text, 'stickers.' + sticker.id, `${sticker.id} has no ${lang} text`);
      assert.ok(text.trim().length > 0);
    }
  }
  assert.equal(new Set(STICKERS.map((s) => s.id)).size, STICKERS.length, 'duplicate sticker id');
});
