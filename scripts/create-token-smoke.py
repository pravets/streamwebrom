"""Protected room creation in a real browser.

Run against a disposable server started with -create-token:
  python scripts/create-token-smoke.py --url http://localhost:18447 --token test-create-token-123456
"""
import argparse
import time
from playwright.sync_api import sync_playwright, expect

parser = argparse.ArgumentParser()
parser.add_argument('--url', default='http://localhost:18447')
parser.add_argument('--token', required=True)
parser.add_argument('--browser', help='Optional Chromium executable')
args = parser.parse_args()

with sync_playwright() as pw:
    browser = pw.chromium.launch(headless=True, executable_path=args.browser,
                                 args=['--use-fake-device-for-media-stream', '--use-fake-ui-for-media-stream'])
    context = browser.new_context(locale='es-ES', service_workers='block')
    errors = []
    page = context.new_page()
    page.on('pageerror', lambda error: errors.append(str(error)))
    page.goto(args.url)

    # A protected server announces itself, and the page waits for that answer
    # before offering local creation.
    expect(page.locator('#createCodeRow')).to_be_visible(timeout=10000)
    expect(page.locator('#create')).to_be_enabled()
    expect(page.locator('#createConfigError')).to_be_hidden()

    # The API also works without the UI, for scripts and CI.
    anonymous = context.request.post(args.url + '/api/rooms', data={'room': f'anon-{int(time.time())}'})
    assert anonymous.status == 401, anonymous.status
    assert anonymous.json()['code'] == 'room_create_forbidden'
    authorized = context.request.post(args.url + '/api/rooms', data={'room': f'script-{int(time.time())}'},
                                      headers={'X-Create-Token': args.token})
    assert authorized.status == 201, authorized.status

    # A wrong code keeps the visitor on the page with the field still available.
    page.locator('#createCode').fill('x' * 16)
    page.locator('#create').click()
    expect(page.locator('.toast.err')).to_be_visible(timeout=10000)
    assert '/studio' not in page.url
    expect(page.locator('#createCodeRow')).to_be_visible()

    # VDO.Ninja rooms are created elsewhere and never need the code.
    page.locator('#roomEngine').select_option('vdo')
    expect(page.locator('#createCodeRow')).to_be_hidden()
    page.locator('#roomEngine').select_option('local')
    expect(page.locator('#createCodeRow')).to_be_visible()

    page.locator('#createCode').fill(args.token)
    page.locator('#create').click()
    page.wait_for_url('**/studio?room=*', timeout=20000)
    assert '#key=' in page.url
    assert args.token not in page.url

    # The code lives in the field: it must not reach storage or the URL.
    stored = page.evaluate("JSON.stringify(localStorage)")
    assert args.token not in stored, 'access code leaked into localStorage'
    assert args.token not in page.content()
    assert not errors, errors
    browser.close()
