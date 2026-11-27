"""Helpers shared by the Playwright smoke scripts.

Run the scripts from the repository root (``python scripts/studio-smoke.py``);
Python puts the script's own folder on sys.path, so ``from smoke_util import
click_centered`` works without installing anything.
"""


def click_centered(locator):
    """Scroll a control clear of the sticky control bar, then click it.

    The studio's control bar is sticky at the bottom, so the quick actions of
    the last tiles can sit underneath it and swallow the click. Scrolling the
    element to the middle of the viewport is what a person does before tapping.
    """
    locator.evaluate("el => el.scrollIntoView({block:'center'})")
    locator.click()
