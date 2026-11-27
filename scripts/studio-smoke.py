"""Live network/battery stats, touch ordering, clean stage and OBS quality/recovery."""
import argparse
import json
import struct
import time
import zlib
from pathlib import Path
from playwright.sync_api import sync_playwright, expect
from smoke_util import click_centered


def solid_png(width, height, rgb=(48, 48, 64)):
    """A plain PNG built in memory: the media source needs a real picture size.

    A 1x1 canvas is degenerate for the encoder, so the smoke uses a small but
    normal 16:9 frame instead of shipping a fixture file in the repository.
    """
    row = b"\x00" + bytes(rgb) * width
    raw = row * height

    def chunk(tag, data):
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)

    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")


COVER_PNG = solid_png(320, 180)

parser=argparse.ArgumentParser()
parser.add_argument('--url',default='http://localhost:18443')
parser.add_argument('--browser')
parser.add_argument('--screenshots')
args=parser.parse_args()
with sync_playwright() as pw:
    browser=pw.chromium.launch(headless=True,executable_path=args.browser,args=[
        '--use-fake-device-for-media-stream','--use-fake-ui-for-media-stream','--autoplay-policy=no-user-gesture-required'])
    context=browser.new_context(locale='es-ES',permissions=['camera','microphone'],service_workers='block')
    keys=context.request.post(args.url+'/api/rooms',data={'room':'studio-'+str(time.time_ns()),'lobby':False,'password':'smoke-password'}).json()
    errors=[]
    host=context.new_page();host.on('pageerror',lambda e:errors.append(str(e)))
    host.goto(args.url+'/studio?room='+keys['room']+'#key='+keys['hostKey'])
    expect(host.locator('#status')).to_have_text('conectado',timeout=20000)
    host.locator('#addCam').click()
    expect(host.locator('#backstage .tile')).to_have_count(1,timeout=20000)
    mobile=browser.new_context(locale='es-ES',permissions=['camera','microphone'],service_workers='block',has_touch=True,is_mobile=True,viewport={'width':851,'height':393})
    guest=mobile.new_page();guest.on('pageerror',lambda e:errors.append(str(e)))
    guest.add_init_script("window.__battery={level:.63,charging:true};navigator.getBattery=async()=>__battery")
    guest.goto(args.url+'/guest?room='+keys['room']+'&label=Panelista&password=smoke-password&autostart=1#invite='+keys['inviteKey'])
    expect(guest.locator('#call')).to_be_visible(timeout=20000)
    expect(guest.locator('#connection')).to_be_hidden(timeout=20000)
    expect(host.locator('#backstage .tile')).to_have_count(2,timeout=20000)
    expect(guest.locator('#roomPing')).to_contain_text('ms',timeout=15000)
    expect(host.locator('#roomPing')).to_contain_text('ms',timeout=15000)
    host_guest=host.locator('#backstage .tile').filter(has=host.locator('.name').filter(has_text='Panelista'))
    expect(host_guest.locator('.tile-latency')).to_contain_text('63%',timeout=20000)
    expect(guest.locator('[data-id="me"] .tile-latency')).to_contain_text('63%',timeout=20000)
    guest.evaluate('__battery.level=.41;__battery.charging=false')
    expect(host_guest.locator('.tile-latency')).to_contain_text('41%',timeout=12000)
    assert '⚡' not in host_guest.locator('.tile-latency').inner_text()

    # Hiding a camera must also hide its backstage picture without stopping audio.
    host_guest.locator('.source-options').click()
    host.locator('#sourceSettings button[title="Ocultar video en escenario y backstage (mantiene el audio)"]').click()
    expect(host_guest).to_have_class(__import__('re').compile('video-hidden'))
    expect(host_guest.locator('video')).to_have_css('opacity','0')
    expect(guest.locator('[data-id="me"]')).to_have_class(__import__('re').compile('video-hidden'))
    host.locator('#sourceSettings button[title="Mostrar video"]').click()
    expect(host_guest.locator('video')).to_have_css('opacity','1')
    host.locator('#closeSourceSettings').click()
    # Persist actual UI options and confirm stored roster links survive a reload.
    host.locator('#openRoomDialog').click()
    host.locator('#persistDays').select_option('90')
    host.locator('#persistSave').click()
    expect(host.locator('#persistStatus')).to_contain_text('Guardada hasta',timeout=10000)
    expect(host.locator('#persistentParticipants')).to_contain_text('Panelista')
    expect(host.locator('#persistentParticipants button').filter(has_text='Link OBS guardado').first).to_be_visible()
    guest.wait_for_function('room=>!!localStorage.getItem("swr.rejoin."+room)',arg=keys['room'])
    host.locator('#closeRoomDialog').click()
    # Reordering moves existing media nodes; rotation must retain that order.
    before=guest.locator('#grid .tile').evaluate_all('(els)=>els.map(e=>e.dataset.id)')
    guest.locator('#reorderGrid').tap()
    guest.locator('#grid .tile').last.locator('[data-move="prev"]').tap()
    guest.wait_for_function('first=>document.querySelector("#grid .tile").dataset.id!==first',arg=before[0])
    order=guest.locator('#grid .tile').evaluate_all('(els)=>els.map(e=>e.dataset.id)')
    guest.set_viewport_size({'width':393,'height':851});guest.set_viewport_size({'width':851,'height':393})
    assert guest.locator('#grid .tile').evaluate_all('(els)=>els.map(e=>e.dataset.id)')==order
    guest.locator('#reorderGrid').tap()
    guest.locator('#callFullscreen').tap()
    expect(guest.locator('#call')).to_have_class('call cinema')
    guest.wait_for_function("()=>{const r=document.querySelector('#grid').getBoundingClientRect();return r.height>=innerHeight-2&&r.width>=innerWidth-2}")
    assert guest.locator('#grid .tile').evaluate_all('els=>els.every(e=>{const r=e.getBoundingClientRect();return Math.abs(r.width/r.height-16/9)<.025})')
    if args.screenshots:
        folder=Path(args.screenshots);folder.mkdir(parents=True,exist_ok=True)
        guest.screenshot(path=str(folder/'mobile-landscape-fullscreen.png'))
    guest.locator('#exitFullscreen').tap();expect(guest.locator('#call')).not_to_have_class('call cinema')

    # Changing camera send quality modifies the publisher, not only CSS size.
    host_guest.locator('.source-options').click();host.locator('#sourceSettingsQuality select').select_option('360')
    guest.wait_for_function("()=>document.querySelector('[data-id=me] video').srcObject.getVideoTracks()[0].getSettings().height===360",timeout=15000)
    host.locator('#closeSourceSettings').click()
    host_guest=host.locator('#backstage .tile').filter(has=host.locator('.name').filter(has_text='Panelista'))
    stream_id=host_guest.get_attribute('data-id')
    _btn = host_guest.locator('.stagebtn')
    click_centered(_btn)
    host.locator('#studioViewMenu summary').click()
    host.locator('#stageView').click()
    expect(host.locator('#studioRoot')).to_have_class('studio cinema')
    expect(host.locator('.topbar')).to_be_hidden();expect(host.locator('.side')).to_be_hidden()
    expect(host.locator('#backstage')).to_be_hidden();expect(host.locator('.controlbar')).to_be_hidden()
    assert host.locator('#program').evaluate('e=>{const r=e.getBoundingClientRect();return Math.abs(r.width/r.height-16/9)<.025&&r.width>=innerWidth-2}')
    if args.screenshots:host.screenshot(path=str(Path(args.screenshots)/'host-stage-only.png'))
    host.locator('#exitFullscreen').click();expect(host.locator('#backstage')).to_be_visible()

    obs=context.new_page();obs.on('pageerror',lambda e:errors.append(str(e)))
    obs.add_init_script("window.__pcs=[];const P=RTCPeerConnection;window.RTCPeerConnection=class extends P{constructor(...a){super(...a);__pcs.push(this)}}")
    start=time.monotonic()
    obs.goto(args.url+'/view?room='+keys['room']+'&stream='+stream_id+'&quality=360&fps=15&mute=1#key='+keys['viewKey'])
    obs.wait_for_function("()=>{const v=document.querySelector('#single');return v.videoWidth>0&&v.readyState>=2}",timeout=15000)
    first_frame_ms=round((time.monotonic()-start)*1000)
    assert obs.locator('#singleOut').evaluate('e=>e.width===640&&e.height===360')
    # An ICE failure used to freeze the OBS source until its page was reloaded.
    obs.evaluate("const p=__pcs[0];p.close();Object.defineProperty(p,'connectionState',{value:'failed'});p.dispatchEvent(new Event('connectionstatechange'))")
    obs.wait_for_function("()=>__pcs.length>=2&&__pcs.at(-1).connectionState==='connected'&&document.querySelector('#single').readyState>=2",timeout=15000)
    obs.goto(args.url+'/view?room='+keys['room']+'&scene=1&quality=720&fps=25&mute=1#key='+keys['viewKey'])
    expect(obs.locator('#out')).to_be_visible(timeout=15000)
    assert obs.locator('#out').evaluate('e=>e.width===1280&&e.height===720')
    if args.screenshots:obs.screenshot(path=str(Path(args.screenshots)/'obs-720p.png'))
    # Closing a tab and returning keeps the participant's OBS stream ID.
    guest_url=guest.url
    guest.close()
    expect(host_guest).to_have_count(0,timeout=10000)
    guest=mobile.new_page();guest.on('pageerror',lambda e:errors.append(str(e)))
    guest.goto(guest_url)
    expect(guest.locator('#connection')).to_be_hidden(timeout=20000)
    expect(host_guest).to_have_count(1,timeout=20000)
    assert host_guest.get_attribute('data-id')==stream_id
    # Host local output settings restore from the saved room, even after cache storage is cleared.
    host.locator('[data-tab="obs"]').click()
    host.locator('#obsQuality').select_option('540')
    host.wait_for_timeout(800)
    host.evaluate('sessionStorage.clear()')
    host.reload()
    expect(host.locator('#status')).to_have_text('conectado',timeout=15000)
    expect(host.locator('#obsQuality')).to_have_value('540')
    host.locator('#openRoomDialog').click()
    expect(host.locator('#persistStatus')).to_contain_text('Guardada hasta')
    host.locator('#closeRoomDialog').click()
    # Accent: it is only offered for a source that is already in the scene, and
    # only in layouts with a spotlight slot. Nothing is silently ignored.
    host.locator('#sceneBar .scenecard').nth(1).click()          # scene 2 starts empty
    host_guest.locator('.source-options').click()
    accent=host.locator('#sourceSettingsActions button').filter(has_text='Destacar en esta escena')
    expect(accent).to_be_disabled()
    expect(accent).to_have_attribute('title','Añade la fuente a la escena 2 para poder destacarla')
    host.locator('#sourceSettingsActions button').filter(has_text='Pasar al escenario').click()
    accent=host.locator('#sourceSettingsActions button').filter(has_text='Destacar en esta escena')
    expect(accent).to_be_disabled()                               # scene 2 still uses the grid layout
    expect(accent).to_have_attribute('title','Este diseño no tiene hueco para el destacado: elige Destacado, PiP, Lateral o Solo')
    host.locator('#closeSourceSettings').click()
    host.locator('#layouts button[data-l="spotlight"]').click()
    host_guest.locator('.source-options').click()
    accent=host.locator('#sourceSettingsActions button').filter(has_text='Destacar en esta escena')
    expect(accent).to_be_enabled()
    accent.click()
    expect(host.locator('#sourceSettingsActions button').filter(has_text='Quitar destacado')).to_be_visible()
    host.locator('#closeSourceSettings').click()
    # Media sources letterbox by default; the host can switch one to cover and
    # the guest preview must follow the program through the shared room state.
    host.locator('#mediaFile').set_input_files(files=[{'name': 'cover.png', 'mimeType': 'image/png', 'buffer': COVER_PNG}])
    media_tile = host.locator('#backstage .tile[data-kind="media"]')
    expect(media_tile).to_have_count(1, timeout=20000)
    media_tile.locator('.source-options').click()
    host.locator('#sourceSettingsQuality select').select_option('cover')
    host.locator('#closeSourceSettings').click()
    guest_media = guest.locator('#grid .tile[data-kind="media"]')
    try:
        expect(guest_media).to_have_count(1, timeout=35000)
    except AssertionError:
        # The media source is published by the host; if the guest never
        # subscribed, say why instead of only "count 0".
        print('GUEST GRID:', guest.locator('#grid').inner_html()[:2000], flush=True)
        print('PAGE ERRORS:', errors, flush=True)
        raise
    expect(guest_media).to_have_class(__import__('re').compile('fit-cover'))
    expect(guest_media.locator('video')).to_have_css('object-fit', 'cover')
    # Switching language relabels the labels this module builds itself: the
    # conversation switcher and the dm button on the tiles. Inside the call the
    # prejoin picker is hidden, so switch through the same module API it calls.
    guest.evaluate("async () => { const m = await import('/js/i18n.js'); m.setLang('ru'); }")
    expect(guest.locator('#chatBtn')).to_have_attribute('title', 'Чат')
    guest.locator('#chatBtn').click()
    expect(guest.locator('#chatConvos button').first).to_contain_text('Общий')
    expect(guest_media.locator('.tile-dm')).to_have_attribute('title', 'Личное сообщение')
    assert not errors,errors
    print(json.dumps({'liveRTTAndRoomPing':True,'batteryChargingUpdates':True,'touchOrderSurvivesRotation':True,'fullScreen16by9':True,'hostStageOnly':True,'cameraSend360p':True,'obsFirstFrameMs':first_frame_ms,'obsMediaRecovers':True,'output360p720p':True,'persistentRoomRosterAndOptions':True,'hiddenBackstage':True,'accentNeedsSceneAndSlot':True,'mediaCoverFollowsProgram':True,'chatLabelsFollowLanguage':True,'pageErrors':errors}),flush=True)
    mobile.close();context.close();browser.close()
