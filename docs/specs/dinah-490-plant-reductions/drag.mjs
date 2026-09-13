// Reduction of the §8 shapes this round mints. Pure; no vscode, no bulk layer.
const PLANT = process.env.PLANT ?? "none";

export function dragRowsFor(source) {
  const rows = [];
  for (const el of source) {
    if (el.kind === "card" && el.ref && el.root && el.columnId) {
      rows.push({ kind: "card", ref: el.ref,
        payload: { ref: el.ref, root: el.root, folder: el.folder, columnId: el.columnId } });
    } else {
      rows.push({ kind: "other", ref: el.label });
    }
  }
  // PLANT A: dragRowsFor drops the non-card rows.
  return PLANT === "A" ? rows.filter((r) => r.kind === "card") : rows;
}

export function offerDrag(source, mime, sink, wrap) {
  const rows = dragRowsFor(source);
  if (!rows.some((r) => r.kind === "card")) return;
  // PLANT B: offerDrag wraps only the card rows, which is the shape finding 1
  // says the mime entry carries today. Still a DragRow[], so it typechecks.
  sink.set(mime, wrap(PLANT === "B" ? rows.filter((r) => r.kind === "card") : rows));
}

export function dragRowsFrom(value) {
  if (!Array.isArray(value) || value.length === 0) return undefined;
  let cards = 0;
  for (const row of value) {
    if (typeof row !== "object" || row === null) return undefined;
    if (typeof row.ref !== "string") return undefined;
    if (row.kind === "other") continue;
    if (row.kind !== "card") return undefined;
    const p = row.payload;
    if (typeof p !== "object" || p === null) return undefined;
    for (const k of ["ref", "root", "folder", "columnId"]) {
      if (typeof p[k] !== "string") return undefined;
    }
    cards += 1;
  }
  return cards === 0 ? undefined : value;
}
