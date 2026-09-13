// Reduction of §3a's perimeter, §3b's collectingHost and summaryFor, and the
// three host shapes src/ declares. Round 6. Nothing in the extension imports
// this file; it exists so that the plants the perimeter criteria prescribe can
// be performed rather than reasoned about.
const PLANT = process.env.PLANT ?? "none";
const mark = Symbol("dinah.bulk.report");

// §3b. The members through which a command puts words in front of the reader,
// declared once and read by collectingHost rather than spelled at each host.
export const REPORT_CHANNELS = ["showError", "showInfo", "showWarning"];
// The members through which a command asks the reader something. These always
// reach the real host, because the question is the reader's own.
export const PROMPT_CHANNELS = ["pick", "input", "confirmDestructive"];

/** A host bound to a window that records what it was asked to show. */
export function realHost(extras = {}) {
  const shown = [];
  return {
    shown,
    lines: [],
    t: (key, args) => (args === undefined ? key : `${key} ${JSON.stringify(args)}`),
    showError: (m) => shown.push({ level: "error", text: m }),
    showInfo: (m) => shown.push({ level: "info", text: m }),
    showWarning: async (m) => {
      shown.push({ level: "warning", text: m });
      return undefined;
    },
    appendLines(l) {
      this.lines.push(...l);
    },
    revealOutput: () => shown.push({ level: "reveal", text: "" }),
    ...extras,
  };
}

/**
 * §3b. Captures every REPORT_CHANNEL the host carries, and checkpoint when it
 * carries one, so a run of several rows produces one message rather than one
 * message per row. Every other member, the prompts among them, is forwarded.
 */
export function collectingHost(host) {
  const errors = [], notes = [], folders = [];
  // PLANT E: the round-5 shape, which intercepts showError alone and lets every
  // per-row showInfo and showWarning reach the reader.
  // PLANT F: a hand-written list that omits showWarning, which is the same
  // defect one channel over.
  const intercepted =
    PLANT === "E" ? ["showError"]
    : PLANT === "F" ? ["showError", "showInfo"]
    : REPORT_CHANNELS;
  const wrapped = { ...host };
  for (const name of intercepted) {
    if (host[name] === undefined) continue;
    if (name === "showError") wrapped.showError = (m) => { errors.push(m); };
    if (name === "showInfo") wrapped.showInfo = (m) => { notes.push({ level: "info", text: m }); };
    if (name === "showWarning") wrapped.showWarning = async (m) => {
      notes.push({ level: "warning", text: m });
      return undefined; // Nobody chose an action, so revealOutput is not reached.
    };
  }
  if (host.checkpoint !== undefined) {
    wrapped.checkpoint = async (folder) => { folders.push(folder); };
  }
  return { host: wrapped, drain: () => ({ errors, notes, folders }) };
}

/** §3a. One entry point owns the run from the prompt to the message. */
export async function runBulk(rows, refOf, resolve, deps, ask, act) {
  const resolved = [], slots = [];
  for (const row of rows) {
    const r = resolve(row);
    slots.push(r);
    if (r !== undefined) resolved.push(r);
  }
  // The prompt runs before the host switch, so ask always holds the real host.
  // PLANT G: ask is handed the collecting host, so a refusal composed inside it
  // is captured and the reader is told nothing at all.
  const askHost = PLANT === "G" ? collectingHost(deps.host).host : deps.host;
  const answer = await ask(resolved, askHost);
  const entry = (row, outcome) => ({ ref: refOf(row), outcome });
  if (answer === undefined) {
    return {
      [mark]: true, selected: rows.length, cancelled: true,
      emptyRun: deps.emptyRun ?? "speak", notes: [],
      entries: rows.map((r) => entry(r, { kind: "skipped", why: deps.skipReason })),
    };
  }
  const many = rows.length > 1;
  const collector = many ? collectingHost(deps.host) : undefined;
  const actHost = collector === undefined ? deps.host : collector.host;
  const entries = [];
  const finished = [];
  for (let i = 0; i < rows.length; i++) {
    if (slots[i] === undefined) {
      entries.push(entry(rows[i], { kind: "skipped", why: deps.skipReason }));
      continue;
    }
    let outcome;
    try {
      outcome = await act(slots[i], answer, actHost);
    } catch (e) {
      outcome = { kind: "failed", failure: String(e?.message ?? e) };
    }
    entries.push(entry(rows[i], outcome));
    if (outcome.kind === "done") finished.push(slots[i]);
  }
  const drained = collector === undefined
    ? { errors: [], notes: [], folders: [] }
    : collector.drain();
  if (deps.finish !== undefined) await deps.finish(finished, deps.host);
  for (const folder of drained.folders) await deps.host.checkpoint?.(folder);
  const report = {
    [mark]: true, selected: rows.length, entries, cancelled: false,
    emptyRun: deps.emptyRun ?? "speak", notes: drained.notes,
    doneMessage: deps.successMessage?.(finished, deps.host.t),
  };
  // §4 step 7: the channel first, then the one message.
  const lines = [];
  for (const e of entries) {
    if (e.outcome.kind === "failed") lines.push(`${e.ref}: ${e.outcome.failure}`);
    if (e.outcome.kind === "skipped") lines.push(`${e.ref}: ${e.outcome.why}`);
  }
  for (const note of drained.notes) lines.push(note.text);
  if (lines.length > 0) deps.host.appendLines(lines);
  const summary = summaryFor(report, deps.host.t);
  if (summary.level === "info") deps.host.showInfo(summary.message);
  if (summary.level === "warning") {
    const picked = await deps.host.showWarning(summary.message, ["dialog.openOutput.label"]);
    if (picked !== undefined) deps.host.revealOutput();
  }
  return report;
}

/** §3b. The single message a run produces, or none. */
export function summaryFor(report, t = (k, a) => `${k} ${JSON.stringify(a ?? {})}`) {
  if (report?.[mark] !== true) throw new Error("unmarked report");
  const n = (k) => report.entries.filter((e) => e.outcome.kind === k).length;
  const done = n("done"), failed = n("failed"), skipped = n("skipped");
  if (done + failed + skipped !== report.selected)
    throw new Error(`does not reconcile: ${done + failed + skipped} vs ${report.selected}`);
  const notes = report.notes ?? [];
  if (report.cancelled) return { level: "none", message: "" };
  if (report.emptyRun === "silent" && done === 0 && failed === 0) return { level: "none", message: "" };
  const clean = failed === 0 && skipped === 0;
  if (clean && report.doneMessage !== undefined)
    return { level: "info", message: report.doneMessage };
  if (report.selected <= 1) return { level: "none", message: "" };
  if (clean && notes.length === 0)
    return { level: "info", message: t("dialog.bulk.allSucceeded", { count: done }) };
  if (clean)
    return {
      level: notes.some((note) => note.level === "warning") ? "warning" : "info",
      message: t("dialog.bulk.allSucceededNotes", { count: done, notes: notes.length }),
    };
  return {
    level: "warning",
    message: t("dialog.bulk.partial", {
      succeeded: done, selected: report.selected, refused: failed, skipped,
    }),
  };
}

/** §5 Move's ask, kept here so the perimeter plants can drive a refusal. */
export function moveAsk(sharedOf) {
  return async (resolved, host) => {
    if (resolved.length === 0) return true;
    const shared = sharedOf(resolved);
    if (shared.length > 0) return shared[0];
    host.showError(host.t("dialog.move.noSharedDestination", { count: resolved.length }));
    return undefined;
  };
}
