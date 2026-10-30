// The presenter consumes engine execution and aggregate state without inference.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const window = {};
const context = vm.createContext({ window, document: { dispatchEvent() {} }, CustomEvent: class { constructor(name, value) { this.detail = value.detail; } } });
vm.runInContext(fs.readFileSync(path.join(__dirname, '../state-order.js'), 'utf8'), context);
vm.runInContext(fs.readFileSync(path.join(__dirname, '../chat-state.js'), 'utf8'), context);
const describe = window.dinahChatState.describe;
for (const [state, word, colour, glyph] of [
  ['working', 'Working', 'accent', 'chat-working'], ['waiting', 'Waiting', 'warn', 'flag'],
  ['stuck', 'Stuck', 'danger', 'warning'], ['idle', 'Idle', '', 'ring']
]) {
  const result = describe({ life: 'open', exec: { state, why: state === 'waiting' ? 'Context full' : '' } });
  assert.equal(result.state, state); assert.equal(result.kind, colour); assert.equal(result.glyph, glyph);
  assert.ok(result.text.startsWith(word));
}
assert.equal(describe({ life: 'open', exec: { state: 'waiting', why: 'Context full' } }).text, 'Waiting (Context full)');
for (const life of ['closed', 'retired']) {
  const result = describe({ life, exec: { state: 'working' } });
  assert.equal(result.glyph, ''); assert.ok(!result.text.includes('Working'));
}
assert.equal(describe({ working: true, state: 'open' }).text, 'State unavailable');
const rollup = describe({ counts: { top: 'waiting', stuck: 2, waiting: 1, working: 4, idle: 0 } });
assert.equal(rollup.state, 'waiting'); // The engine chooses priority, even if counts disagree.
assert.equal(rollup.text, '2 stuck, 1 waiting, 4 working');
assert.ok(rollup.label.includes('2 stuck'));
assert.equal(describe({ chatState: { life: 'open', exec: { state: 'idle', why: '' } } }).text, 'Idle');
assert.equal(describe({ life: 'open', elsewhere: 'Running in another Dinah window' }).glyph, '');
const elsewhere = describe({ counts: { unknown: 1, closed: 2 } });
assert.equal(elsewhere.text, '1 running elsewhere, 2 closed'); assert.equal(elsewhere.glyph, '');
assert.equal(describe({ top: 'idle', idle: 0, unknown: 0, closed: 2 }).text, 'Idle (2 closed)');
const live = { stateGen: 'one', stateVer: 2, chats: [{ sessionId: 'chat', bus: 'bus-chat', life: 'open', window: false, exec: { state: 'waiting', why: 'Context full' } }], groups: [{ name: 'group', counts: { top: 'waiting', waiting: 1 } }] };
assert.equal(window.dinahChatState.apply(live), true);
assert.equal(describe({ sessionId: 'chat' }).text, 'Waiting (Context full)');
assert.equal(describe({ id: 'bus-chat' }).state, 'waiting');
assert.equal(describe({ name: 'group', counts: { top: 'idle', idle: 1 } }).state, 'waiting');
assert.equal(window.dinahChatState.apply({ ...live, stateVer: 1, chats: [] }), false);
assert.equal(describe({ sessionId: 'chat' }).state, 'waiting');
assert.equal(window.dinahChatState.apply({ ...live, stateGen: 'two', stateVer: 1, chats: [{ sessionId: 'chat', life: 'closed', window: false }] }), true);
assert.equal(describe({ sessionId: 'chat' }).text, 'Closed');
console.log('PASS shared execution-state presentation contract');
