import { runBulk, summaryFor, moveAsk } from "./move.mjs";
import assert from "node:assert/strict";

function fakeHost(targetedCount) {
  const errors = [], picks = [];
  return { targetedCount, errors, picks,
    showError: (m) => errors.push(m),
    pick: (items) => { picks.push(items); return items[0]; } };
}
const spawns = [];
const legal = { "wb-1": ["Doing", "Done"], "wb-2": ["Doing"], "wb-3": ["Doing"] };
const run = (rows) => {
  spawns.length = 0;
  const host = fakeHost(rows.length);
  return runBulk(rows, (r) => r.label,
    (r) => (r.kind === "card" ? r.ref : undefined),
    { host, skipReason: "not a card" },
    moveAsk((ref) => { spawns.push(["instructions", ref]); return legal[ref]; }, spawns),
    async (ref, dest) => { spawns.push(["move", ref, dest]); return { kind: "done" }; })
    .then((rep) => ({ rep, host }));
};
let ran = 0;
const check = (n, f) => { ran++; return f().then(() => console.log("  green", n),
  (e) => console.log("  RED  ", n, "::", String(e.message).split("\n")[0])); };

const cards = (...r) => r.map((x) => ({ kind: "card", ref: x, label: x }));
const cols = (...l) => l.map((x) => ({ kind: "column", label: x }));

console.log("AC-13 clause 4a: a mixed selection whose cards share no destination");
await check("noSharedDestination names the resolved count (2), not the targeted one (4)", async () => {
  legal["wb-8"] = ["Backlog"];
  legal["wb-9"] = ["Done"];
  const { rep, host } = await run([{ kind: "card", ref: "wb-8", label: "wb-8" },
    { kind: "card", ref: "wb-9", label: "wb-9" }, ...cols("Doing", "Backlog")]);
  assert.deepEqual(host.errors, ["noSharedDestination 2"]);
  assert.deepEqual(host.picks, []);
  assert.equal(rep.selected, 4);
  assert.equal(summaryFor(rep).level, "none");
});

console.log("AC-13 clause 4b: a selection resolving to zero cards");
await check("asks nothing, spawns nothing, warns like Claim does", async () => {
  const { rep, host } = await run(cols("Doing", "Done", "Backlog"));
  assert.deepEqual(spawns, []);
  assert.deepEqual(host.picks, []);
  assert.deepEqual(host.errors, []);
  assert.equal(rep.selected, 3);
  assert.equal(rep.entries.filter((e) => e.outcome.kind === "skipped").length, 3);
  assert.deepEqual(summaryFor(rep), { level: "warning", message: "partial 0/3 f0 s3" });
});
console.log("assertions run:", ran);
