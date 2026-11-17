import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { moduleURL } from './module-url.mjs';
globalThis.location={search:'',hash:''};
// The route policy is what this file tests: swap the signalling module for a
// stub so no browser media stack is needed.
const fakeRTC='data:text/javascript;base64,'+Buffer.from(`
export const api=(path,body)=>globalThis.__routeCheck(path,body);
export class Signal{constructor(){}async connect(){throw new Error('probe media failed')}close(){}}
export const publish=async()=>{throw new Error('probe media failed')};
export const subscribe=async()=>{throw new Error('probe media failed')};
export const iceConfiguration=(servers,mode)=>{if(mode!=='tls'||!servers||!servers.length)throw new Error('sin TLS');return {};};
`).toString('base64');
const i18nURL=await moduleURL('i18n.js');
const source=(await readFile(new URL('../web/js/network.js',import.meta.url),'utf8')).replace('"./rtc.js"',JSON.stringify(fakeRTC)).replace('"./i18n.js"',JSON.stringify(i18nURL));
const {chooseRoute,routeCandidates,automaticRoute}=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
const cfg={ice:[{urls:'turns:cloud.test:443?transport=tcp'}],localIce:[{urls:'turns:local.test:443?transport=tcp'}],relay:{provider:'cloudflare'}};

test('automatic route checks Cloudflare and selects local before room admission if media fails',async()=>{
  const checks=[],progress=[];
  const mode=await chooseRoute(cfg,async mode=>{checks.push(mode);if(mode!=='local')throw Error('allocation worked but video failed');},mode=>progress.push(mode));
  assert.equal(mode,'local');assert.deepEqual(checks,['auto','tls','local']);assert.deepEqual(progress,checks);
});
test('healthy principal route does not need a second relay or a user choice',async()=>{
  const checks=[];assert.equal(await chooseRoute(cfg,async mode=>checks.push(mode)),'auto');assert.deepEqual(checks,['auto']);
});
test('quota fallback is respected before the automatic connection check',()=>{
  assert.deepEqual(routeCandidates({...cfg,relay:{provider:'local'}}),['local']);
});
test('unavailable relays fail before joining the backstage instead of waiting indefinitely',async()=>{
  await assert.rejects(chooseRoute(cfg,async()=>{throw Error('blocked');}),/comprobar una ruta de video/);
});
test('LAN installations without a TLS relay retain direct automatic connection',async()=>{
  assert.equal(await chooseRoute({ice:[]},()=>{throw Error('must not probe');}),'auto');
});

const autoCfg={...cfg,capabilities:{automaticRouting:true}};

test('the automatic check uses one probe per candidate and releases every one',async()=>{
  const checks=[],deletes=[];
  globalThis.__routeCheck=async(path)=>{checks.push(path);return {room:'check-1',hostKey:'key'};};
  globalThis.fetch=async(url)=>{deletes.push(url);return {};};
  try {
    await assert.rejects(automaticRoute(autoCfg,()=>{}),/comprobar una ruta de video/);
    assert.equal(checks.length,3,'one fresh probe per candidate');
    assert.equal(deletes.length,3,'every probe is released');
    assert.ok(deletes.every(url=>url.includes('/api/route-check/check-1')));
  } finally { delete globalThis.__routeCheck; delete globalThis.fetch; }
});

test('a busy diagnostic pool falls back to the ordinary join',async()=>{
  globalThis.__routeCheck=async()=>{const e=Error('busy');e.code='route_probe_busy';throw e;};
  globalThis.fetch=async()=>{throw Error('must not release a probe that was never created');};
  try {
    assert.equal(await automaticRoute(autoCfg,()=>{}),'auto');
  } finally { delete globalThis.__routeCheck; delete globalThis.fetch; }
});

test('an invalid room or password is reported, not turned into a route or a fallback',async()=>{
  globalThis.__routeCheck=async()=>{const e=Error('Sala o contraseña inválida');e.code='route_auth_invalid';throw e;};
  try {
    await assert.rejects(automaticRoute(autoCfg,()=>{}),/contraseña/);
  } finally { delete globalThis.__routeCheck; }
});
