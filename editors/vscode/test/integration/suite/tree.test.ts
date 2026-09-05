// The sidebar tree, drawn from a workbench the binary in this commit built.
//
// The unit layer drives every row shape against JSON literals, which is fast
// and covers the cases a real bench cannot easily be pushed into. This suite
// covers the one thing literals cannot: that the fields those literals spell
// are the fields `dinah --json status`, `tree` and `ls` actually emit. A
// rename on the Go side that the mirrors in wire.ts have not answered passes
// every unit test and fails here.
//
// CI-only. Never run the integration layer locally.

import * as assert from "node:assert/strict";

import { api, folder, until } from "./support";

/** One row, as the provider composes it, with the fields this suite reads. */
interface Row {
	readonly label: string;
	readonly description?: string;
	readonly tooltip?: string;
	readonly contextValue?: string;
}

suite("the sidebar tree against a real workbench", () => {
	test("the root is the workbench, and its children are the declared columns", async () => {
		const reported = await api();
		const roots = await reported.tree.getChildren();
		assert.equal(roots.length, 1, "one workspace folder should draw one root row");

		const root = reported.tree.getTreeItem(roots[0]) as Row;
		assert.equal(root.contextValue, "dinah.workbenchRoot");

		const columns = await reported.tree.getChildren(roots[0]);
		assert.ok(columns.length > 0, "the workbench drew no columns at all");
		for (const element of columns) {
			const row = reported.tree.getTreeItem(element) as Row;
			// Since dinah-331 a column row's contextValue says whether the
			// column will take another card. This fixture's workbench is the
			// one `dinah init` scaffolds, whose three columns declare no
			// wip_limit at all, so every column here is open and the exact
			// value is what gets asserted. A disjunction over open and full
			// would let the full half go unexercised while reading as though
			// it were covered. The bare "dinah.column" fails this assertion,
			// which is the point: that is what a row carries when the status
			// and tree answers disagreed about which columns exist, so
			// refusing it proves the join this suite exists to check actually
			// delivered a view.
			assert.equal(
				row.contextValue,
				"dinah.column.open",
				`a column of the uncapped fixture bench does not read as open: ${String(row.contextValue)}`,
			);
		}

		// Since dinah-373 a column row's description is the occupancy alone,
		// and its tooltip carries the facts the description used to spend a
		// word on. Both are pinned as whole strings rather than matched
		// against a pattern: a row that composed an empty description, which
		// is what a broken columnDescription produces, satisfies any check
		// that only asks for the old words to be gone.
		//
		// The fixture is the workbench `dinah init` scaffolds, whose three
		// columns declare no capacity and none of which awaits somebody
		// outside, with three cards filed into intake and one carried into
		// doing. That is why the counts below are what they are, and why the
		// tooltips differ only on the claim-or-pull-through line.
		const rows = columns.map((element) => reported.tree.getTreeItem(element) as Row);
		assert.deepEqual(
			rows.map((row) => row.label),
			["Intake", "Doing", "Done"],
		);
		assert.deepEqual(
			rows.map((row) => String(row.description)),
			["2", "1", "0"],
		);
		// The tooltip is where this suite earns its keep on this card. The
		// unit layer reads takes_work_up and operator_owned off JSON
		// literals, so only a run against the binary proves those are the
		// names `dinah --json status` emits.
		assert.deepEqual(
			rows.map((row) => String(row.tooltip)),
			[
				"Intake\nA card here waits to be pulled onward.\nAn agent moves a card out.",
				"Doing\nCards are claimed here.\nAn agent moves a card out.",
				"Done\nA card here waits to be pulled onward.\nAn agent moves a card out.",
			],
		);
	});

	test("a card added to the bench reaches a row with a real reference", async () => {
		const reported = await api();
		const roots = await reported.tree.getChildren();
		const columns = await reported.tree.getChildren(roots[0]);

		// The fixture put three cards in the first column. Whether they stand
		// under a state group or directly beneath the column depends on what
		// `dinah tree` returns for that column's kind, which is exactly the
		// answer this extension is not allowed to second-guess, so the walk
		// below descends through whatever it finds rather than assuming.
		const cards: Row[] = [];
		const walk = async (element: unknown): Promise<void> => {
			for (const child of await reported.tree.getChildren(element)) {
				const row = reported.tree.getTreeItem(child) as Row;
				if (String(row.contextValue).startsWith("dinah.card.")) {
					cards.push(row);
				}
				await walk(child);
			}
		};
		for (const element of columns) {
			await walk(element);
		}

		assert.ok(
			await until(() => cards.length > 0, 5_000),
			"the fixture's cards reached no row in the tree",
		);

		// The fixture puts two cards in intake and carries one into the work
		// column, so both take-up spellings are on the board at once. That is
		// the whole assertion: a provider that read the wrong field, or a
		// field that moved on the Go side, composes one spelling everywhere
		// and passes any test that only ever sees one.
		const menus = cards.map((row) => String(row.contextValue));
		assert.ok(
			menus.includes("dinah.card.ready.claim"),
			`no card offered Claim, though one stands in a column that takes work up: ${menus.join(", ")}`,
		);
		assert.ok(
			menus.includes("dinah.card.ready.none"),
			`no card withheld Claim, though two stand in intake, which takes none: ${menus.join(", ")}`,
		);
		// Nothing anywhere offers a Pull, because dinah's pull verb takes a
		// destination rather than a card.
		for (const row of cards) {
			assert.ok(
				!String(row.contextValue).includes("pull"),
				`${row.label} composed ${String(row.contextValue)}`,
			);
		}
	});

	test("the workbench row names the folder this window opened on", async () => {
		const reported = await api();
		const resolved = reported.workbenches.get(folder());
		assert.ok(resolved !== undefined && resolved.state === "ok");
	});
});
