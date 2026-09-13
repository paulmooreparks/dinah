// Drives the round-6 perimeter criteria (AC-31 through AC-36) against
// perimeter.mjs. Every assertion prints green or RED, the script prints how
// many it ran, and it exits non-zero when any of them reddened.
import assert from "node:assert/strict";
import {
  REPORT_CHANNELS, PROMPT_CHANNELS, realHost, collectingHost, runBulk, summaryFor, moveAsk,
} from "./perimeter.mjs";

const PLANT = process.env.PLANT ?? "none";
let ran = 0, red = 0;
async function check(name, fn) {
  ran++;
  try { await fn(); console.log("  green", name); }
  catch (e) { red++; console.log("  RED  ", name, "::", String(e.message).split("\n")[0]); }
}

const col = (label) => ({ kind: "column", label });
const card = (ref) => ({ kind: "card", ref, label: ref });
const wb = (label) => ({ kind: "workbench", label });
const refOf = (r) => r.label;
const resolveKind = (kind) => (r) => (r.kind === kind ? r : undefined);

console.log("AC-33 clause 1: a per-row showInfo under a multi-row run");
await check("five columns, four empty: one message, four channel notes", async () => {
  const host = realHost({ checkpoint: async () => {} });
  const rows = [col("A"), col("B"), col("C"), col("D"), col("E")];
  const empty = new Set(["B", "C", "D", "E"]);
  const report = await runBulk(rows, refOf, resolveKind("column"),
    { host, skipReason: "not a column" },
    async () => true,
    async (row, _answer, actHost) => {
      if (empty.has(row.label)) actHost.showInfo(actHost.t("dialog.pull.empty", { from: row.label }));
      return { kind: "done" };
    });
  assert.equal(report.selected, 5);
  assert.equal(host.shown.length, 1, `shown ${JSON.stringify(host.shown)}`);
  assert.equal(host.shown[0].level, "info");
  assert.match(host.shown[0].text, /dialog\.bulk\.allSucceededNotes/);
  assert.equal(report.notes.length, 4);
  assert.equal(host.lines.length, 4);
});

console.log("AC-33 clause 2: a per-row showWarning under a multi-row run");
await check("three workbenches with findings: one warning, no per-row reveal", async () => {
  const host = realHost();
  const rows = [wb("Home"), wb("Work"), wb("Side")];
  const report = await runBulk(rows, refOf, resolveKind("workbench"),
    { host, skipReason: "not a workbench" },
    async () => true,
    async (row, _answer, actHost) => {
      actHost.appendLines([`${row.label}: 2 defects`]);
      await actHost.showWarning(actHost.t("dialog.workbench.checkFindings.toast", { workbench: row.label }), ["open"]);
      return { kind: "done" };
    });
  const reports = host.shown.filter((s) => s.level !== "reveal");
  assert.equal(reports.length, 1, `shown ${JSON.stringify(host.shown)}`);
  assert.equal(reports[0].level, "warning");
  assert.match(reports[0].text, /dialog\.bulk\.allSucceededNotes/);
  assert.equal(host.shown.filter((s) => s.level === "reveal").length, 0);
  assert.equal(report.notes.filter((n) => n.level === "warning").length, 3);
});

console.log("AC-35: the oneCall copy family");
await check("two card rows: one clipboard write, one plural message", async () => {
  const clipboard = [];
  const host = realHost({ copyToClipboard: async (text) => { clipboard.push(text); } });
  const rows = [card("wb-1"), card("wb-2")];
  // PLANT H: the copy command reports per row instead of declaring finish and
  // successMessage, which is one toast per row plus the summary.
  const perRow = PLANT === "H";
  await runBulk(rows, refOf, resolveKind("card"),
    {
      host, skipReason: "not a card",
      finish: perRow ? undefined : async (finished, h) => h.copyToClipboard(finished.map((r) => r.ref).join("\n")),
      successMessage: perRow ? undefined : (finished, t) =>
        (finished.length === 1
          ? t("dialog.card.copiedRef", { ref: finished[0].ref })
          : t("dialog.card.copiedRef.many", { count: finished.length })),
    },
    async () => true,
    async (row, _answer, actHost) => {
      if (perRow) {
        await actHost.copyToClipboard(row.ref);
        actHost.showInfo(actHost.t("dialog.card.copiedRef", { ref: row.ref }));
      }
      return { kind: "done" };
    });
  assert.deepEqual(clipboard, ["wb-1\nwb-2"]);
  assert.equal(host.shown.length, 1, `shown ${JSON.stringify(host.shown)}`);
  assert.match(host.shown[0].text, /dialog\.card\.copiedRef\.many/);
});

await check("one card row: the singular message, unchanged from today", async () => {
  const clipboard = [];
  const host = realHost({ copyToClipboard: async (text) => { clipboard.push(text); } });
  await runBulk([card("wb-1")], refOf, resolveKind("card"),
    {
      host, skipReason: "not a card",
      finish: async (finished, h) => h.copyToClipboard(finished.map((r) => r.ref).join("\n")),
      successMessage: (finished, t) =>
        (finished.length === 1
          ? t("dialog.card.copiedRef", { ref: finished[0].ref })
          : t("dialog.card.copiedRef.many", { count: finished.length })),
    },
    async () => true, async () => ({ kind: "done" }));
  assert.deepEqual(clipboard, ["wb-1"]);
  assert.equal(host.shown.length, 1, `shown ${JSON.stringify(host.shown)}`);
  assert.match(host.shown[0].text, /dialog\.card\.copiedRef /);
});

console.log("AC-32 clause 1: ask holds the real host, so a refusal is heard");
await check("two cards sharing no destination: the reader is told once", async () => {
  const host = realHost();
  const report = await runBulk([card("wb-8"), card("wb-9")], refOf, resolveKind("card"),
    { host, skipReason: "not a card" },
    moveAsk(() => []),
    async () => ({ kind: "done" }));
  assert.equal(host.shown.length, 1, `shown ${JSON.stringify(host.shown)}`);
  assert.equal(host.shown[0].level, "error");
  assert.match(host.shown[0].text, /dialog\.move\.noSharedDestination/);
  assert.equal(summaryFor(report).level, "none");
});

console.log("AC-32 clause 2: collectingHost over each of the three host shapes");
for (const [shape, members] of [
  ["CommandHost", ["showError", "showInfo", "showWarning"]],
  ["WorkbenchCommandHost", ["showError", "showInfo", "showWarning"]],
  ["ColumnCommandHost", ["showError", "showInfo", "showWarning"]],
]) {
  await check(`${shape}: every report channel is captured and nothing is shown`, async () => {
    const host = realHost({ pick: async () => "picked", input: async () => "typed", confirmDestructive: async () => true });
    const { host: wrapped, drain } = collectingHost(host);
    wrapped.showError("e");
    wrapped.showInfo("i");
    const answered = await wrapped.showWarning("w", ["open"]);
    assert.deepEqual(host.shown, [], `shown ${JSON.stringify(host.shown)}`);
    assert.equal(answered, undefined);
    const drained = drain();
    assert.deepEqual(drained.errors, ["e"]);
    assert.deepEqual(drained.notes.map((n) => n.level), ["info", "warning"]);
    assert.deepEqual(members, REPORT_CHANNELS);
    for (const prompt of PROMPT_CHANNELS) assert.equal(wrapped[prompt], host[prompt]);
  });
}

console.log("AC-36: a run where every row fails writes one channel line per row");
await check("three failing rows, three lines, no duplicate from the drained error", async () => {
  const host = realHost();
  await runBulk([card("wb-1"), card("wb-2"), card("wb-3")], refOf, resolveKind("card"),
    { host, skipReason: "not a card" },
    async () => true,
    async (row, _answer, actHost) => {
      actHost.showError(`refused: ${row.ref}`);
      return { kind: "failed", failure: `refused: ${row.ref}` };
    });
  assert.equal(host.lines.length, 3, `lines ${JSON.stringify(host.lines)}`);
  assert.deepEqual(host.lines, ["wb-1: refused: wb-1", "wb-2: refused: wb-2", "wb-3: refused: wb-3"]);
});

console.log("assertions run:", ran, "red:", red);
if (red > 0) process.exitCode = 1;
