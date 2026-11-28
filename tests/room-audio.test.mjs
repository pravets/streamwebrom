import {test} from 'node:test';
import assert from 'node:assert/strict';
import {moduleURL} from './module-url.mjs';

class Stream extends EventTarget {
  constructor(tracks=[]){super();this.tracks=tracks} getAudioTracks(){return this.tracks.filter(t=>t.kind==='audio')}
  addTrack(t){this.tracks.push(t);this.dispatchEvent(new Event('addtrack'))}
}
class Audio extends EventTarget {
  constructor(){super();this.dataset={};this.paused=true;this.muted=false;this.volume=1;this.block=false;this.calls=0}
  play(){this.calls++;if(this.block)return Promise.reject(new Error('gesture required'));this.paused=false;return Promise.resolve()}
  pause(){this.paused=true} remove(){this.removed=true}
}
const doc=new EventTarget();doc.visibilityState='visible';doc.body={append(){}};doc.createElement=()=>new Audio();
globalThis.document=doc;globalThis.window=new EventTarget();globalThis.MediaStream=Stream;
Object.defineProperty(globalThis,'navigator',{value:{audioSession:{type:'auto'}},configurable:true});
const {RoomAudio,effectiveDeafen}=await import(await moduleURL('room-audio.js'));
const settle=()=>new Promise(resolve=>setImmediate(resolve));

test('late audio tracks start on a separate media element and keep playing when video is hidden',async()=>{
 const playback=new RoomAudio();const stream=new Stream();const audio=playback.add('guest',stream);
 assert.equal(audio.calls,0);stream.addTrack({kind:'audio',readyState:'live'});await settle();
 assert.equal(audio.paused,false);assert.equal(audio.srcObject.getAudioTracks().length,1);
 doc.visibilityState='hidden';doc.dispatchEvent(new Event('visibilitychange'));
 assert.equal(audio.paused,false);assert.equal(playback.get('guest'),audio);
 playback.close();assert.equal(audio.srcObject,null);assert.equal(navigator.audioSession.type,'auto');doc.visibilityState='visible';
});
test('blocked playback is recovered by an actual user gesture without permanently muting participants',async()=>{
 let blocked=false;const playback=new RoomAudio(v=>blocked=v);const stream=new Stream([{kind:'audio'}]);
 const audio=playback.add('guest',stream);await settle();audio.pause();audio.block=true;playback.resume();await settle();
 assert.equal(blocked,true);assert.equal(audio.muted,false);
 audio.block=false;doc.dispatchEvent(new Event('pointerdown'));await settle();
 assert.equal(blocked,false);assert.equal(audio.paused,false);playback.close();
});
test('replacing a source detaches old listeners and does not retain old audio',async()=>{
 const playback=new RoomAudio();const oldStream=new Stream([{kind:'audio'}]);const old=playback.add('same-id',oldStream);await settle();
 const fresh=playback.add('same-id',new Stream([{kind:'audio'}]));await settle();
 assert.equal(old.srcObject,null);assert.equal(old.paused,true);assert.equal(old.removed,true);
 const calls=old.calls;oldStream.addTrack({kind:'audio'});assert.equal(old.calls,calls);
 fresh.volume=.35;fresh.muted=true;playback.resume();assert.equal(fresh.volume,.35);assert.equal(fresh.muted,true);playback.close();
});
test('room mute is host deafen OR self deafen: either flag alone mutes',()=>{
 assert.equal(effectiveDeafen(false,false),false);
 assert.equal(effectiveDeafen(true,false),true);
 assert.equal(effectiveDeafen(false,true),true);
 assert.equal(effectiveDeafen(true,true),true);
});
test('host undeafen keeps the guest self-deafen choice; only room audio elements are muted',async()=>{
 const playback=new RoomAudio();const stream=new Stream([{kind:'audio',enabled:true}]);
 const audio=playback.add('guest',stream);await settle();
 let host=false,self=true;
 const apply=()=>{audio.muted=effectiveDeafen(host,self)};
 apply();assert.equal(audio.muted,true);
 host=true;apply();assert.equal(audio.muted,true);
 host=false;apply();assert.equal(audio.muted,true); // the guest's own choice survives the host's undeafen
 self=false;apply();assert.equal(audio.muted,false);
 assert.equal(stream.tracks[0].enabled,true); // outgoing tracks are never touched
 playback.close();
});
test('forEach walks every managed audio element, even without a matching video tile',async()=>{
 const playback=new RoomAudio();
 const a=playback.add('cam-guest',new Stream([{kind:'audio'}]));
 const b=playback.add('screen-share',new Stream([{kind:'audio'}]));await settle();
 const seen=[];playback.forEach((audio,id)=>seen.push([id,audio]));
 assert.deepEqual(seen.map(([id])=>id).sort(),['cam-guest','screen-share']);
 playback.forEach((audio)=>{audio.muted=true});
 assert.equal(a.muted,true);assert.equal(b.muted,true,'screen share audio follows the room mute too');
 playback.close();
});
