// Reduction of §3a runBulk, §3b summaryFor cases 3/5/6/7, and §5 Move's ask.
const PLANT = process.env.PLANT ?? "none";
const mark = Symbol("dinah.bulk.report");

export async function runBulk(rows, refOf, resolve, deps, ask, act) {
  const resolved = [], slots = [];
  for (const row of rows) {
    const r = resolve(row);
    slots.push(r);
    if (r !== undefined) resolved.push(r);
  }
  const answer = await ask(resolved, deps.host);
  if (answer === undefined) {
    return { [mark]: true, selected: rows.length, cancelled: true,
      emptyRun: deps.emptyRun ?? "speak",
      entries: rows.map((r) => ({ ref: refOf(r), outcome: { kind: "skipped", why: deps.skipReason } })) };
  }
  const entries = [];
  for (let i = 0; i < rows.length; i++) {
    if (slots[i] === undefined) {
      entries.push({ ref: refOf(rows[i]), outcome: { kind: "skipped", why: deps.skipReason } });
      continue;
    }
    try { entries.push({ ref: refOf(rows[i]), outcome: await act(slots[i], answer, deps.host) }); }
    catch (e) { entries.push({ ref: refOf(rows[i]), outcome: { kind: "failed", failure: String(e?.message ?? e) } }); }
  }
  return { [mark]: true, selected: rows.length, entries, cancelled: false, emptyRun: deps.emptyRun ?? "speak" };
}

export function summaryFor(report) {
  if (report?.[mark] !== true) throw new Error("unmarked report");
  const n = (k) => report.entries.filter((e) => e.outcome.kind === k).length;
  const done = n("done"), failed = n("failed"), skipped = n("skipped");
  if (done + failed + skipped !== report.selected) throw new Error(`does not reconcile: ${done + failed + skipped} vs ${report.selected}`);
  if (report.cancelled) return { level: "none", message: "" };
  if (report.emptyRun === "silent" && done === 0 && failed === 0) return { level: "none", message: "" };
  if (report.selected <= 1) return { level: "none", message: "" };
  if (failed === 0 && skipped === 0) return { level: "info", message: `allSucceeded ${done}` };
  return { level: "warning", message: `partial ${done}/${report.selected} f${failed} s${skipped}` };
}

// §5 Move's ask. The four branches key on the RESOLVED list runBulk hands it.
export function moveAsk(legalMovesOf, spawnLog) {
  return async (resolved, host) => {
    // PLANT C: branch 2 keys on the targeted count, which is the round-4 wording.
    const count = PLANT === "C" ? host.targetedCount : resolved.length;
    if (resolved.length === 0) {
      if (PLANT === "C") { host.showError(`noSharedDestination ${count}`); return undefined; }
      // PLANT D: branch 4 cancels instead of running, so the summary goes silent.
      return PLANT === "D" ? undefined : true; // branch 4: ask nothing, run with every row skipped.
    }
    const per = resolved.map((c) => legalMovesOf(c, spawnLog));
    const shared = per[0].filter((m) => per.every((list) => list.includes(m)));
    if (shared.length > 0) return host.pick(shared);
    if (resolved.length > 1) { host.showError(`noSharedDestination ${count}`); return undefined; }
    host.showError(`noLegalMoves ${resolved[0]}`);
    return undefined;
  };
}
