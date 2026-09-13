import { dragRowsFor, offerDrag, dragRowsFrom } from "./drag.mjs";
import assert from "node:assert/strict";

const source = [
  { kind: "card", ref: "wb-1", root: "/wb", folder: "/f", columnId: "c1" },
  { kind: "card", ref: "wb-2", root: "/wb", folder: "/f", columnId: "c1" },
  { kind: "card", ref: "wb-3", root: "/wb", folder: "/f", columnId: "c1" },
  { kind: "column", label: "Doing" },
];
let ran = 0;
function check(name, fn) { ran++; try { fn(); console.log("  green", name); }
  catch (e) { console.log("  RED  ", name, "::", e.message.split("\n")[0]); } }

console.log("clause 1 (dragRowsFor answers one row per dragged row)");
check("four rows, three card one other, other names the label", () => {
  const rows = dragRowsFor(source);
  assert.equal(rows.length, 4);
  assert.equal(rows.filter((r) => r.kind === "card").length, 3);
  assert.equal(rows.find((r) => r.kind === "other").ref, "Doing");
});

console.log("clause 2 (the drop reads back what offerDrag put there)");
check("round trip answers the same four rows", () => {
  let item;
  offerDrag(source, "mime", { set: (_m, v) => { item = v; } }, (rows) => rows);
  const back = dragRowsFrom(item);
  assert.notEqual(back, undefined, "dragRowsFrom answered undefined");
  assert.equal(back.length, 4);
  assert.equal(back.filter((r) => r.kind === "other").length, 1);
});

console.log("clause 3 (a drag holding no card row sets nothing)");
check("no entry set", () => {
  let item = "unset";
  offerDrag([{ kind: "column", label: "Doing" }], "mime", { set: (_m, v) => { item = v; } }, (r) => r);
  assert.equal(item, "unset");
});

console.log("clause 4 (a foreign value is not read as ours)");
check("dragRowsFrom refuses a bare payload and a foreign object", () => {
  assert.equal(dragRowsFrom({ ref: "wb-1", root: "/wb", folder: "/f", columnId: "c1" }), undefined);
  assert.equal(dragRowsFrom([{ ref: "wb-1", root: "/wb", folder: "/f", columnId: "c1" }]), undefined);
  assert.equal(dragRowsFrom([{ kind: "other", ref: "Doing" }]), undefined);
});
console.log("assertions run:", ran);
