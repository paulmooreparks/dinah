// The older suites exercise controls that now belong to separate windows.
// Reveal their owner through the same public API as the Window menu.
module.exports = function windows(page) {
  page.setDefaultTimeout(10000);
  page.on('pageerror', error => console.error(error.stack));
  async function reveal(selector) {
    let key;
    if (/^#(?:sessions|chat-filter|chat-scope)\b|data-cmd=["']new-session["'].*\.btn/.test(selector)) key = 'chat-browser';
    else if (/^#(?:ws-select|ws-actions-button|tree)\b/.test(selector)) key = 'workspace';
    else if (/^#questions\b/.test(selector)) key = 'needs-you';
    else if (selector.includes('[data-chat-current]')) {
      key = await page.locator('.win[data-chat-current]').getAttribute('data-win');
    }
    if (key) await page.evaluate(key => window.dinahWorkbench.open(key.startsWith('item-') ? window.dinahWorkbench.item(key) : key), key);
    return key;
  }
  for (const action of ['click', 'dblclick', 'fill', 'type', 'focus', 'press', 'selectOption', 'setInputFiles']) {
    const original = page[action].bind(page);
    page[action] = async (selector, ...args) => {
      const key = await reveal(selector);
      if (key && selector.startsWith('[data-cmd=')) selector = '[data-win="' + key + '"] ' + selector;
      return original(selector, ...args);
    };
  }
  return page;
};
