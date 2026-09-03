// What a registration guard can prove, and what it cannot.
//
// assertCommandsFullyRegistered compares the command ids identity.ts declares
// against the ids activate() collected while it ran. When those two sets
// disagree the function throws. activate() calls it before it returns, so a
// dropped registration fails the whole extension's activation and names the
// id rather than letting the extension load with a menu item nobody wired up.
//
// The guard proves that activate() attempted a vscode.commands.registerCommand
// call for every id identity.ts declares. It proves nothing about what the
// editor did with that attempt. Whether VS Code accepted the registration,
// whether the command reaches the menu a person right-clicks, and whether
// invoking it does the right thing are promises of the editor, and no test in
// this repository can settle any of them.
//
// This module imports nothing, which is what lets a unit test call it. Those
// unit tests live in test/unit/manifest.test.ts and they call it with lists
// they fabricate themselves, so they cover the comparison and nothing else.
// The one place the function ever runs against the real registration set is
// activate(), and no unit test can reach that. extension.ts imports vscode,
// and test/unit/layers.test.ts forbids the unit layer from importing either
// vscode or a module that does. The integration suite, which starts a real
// editor host and activates the extension inside it, is what reaches that
// call.
//
// That suite does not hold activate()'s call to this function in place, and a
// later reader should not credit it with doing so. Activation succeeds just as
// readily with that call deleted, so the suite goes red only when a
// registration is genuinely missing at the same time. The wiring was
// established once, by dropping a registration deliberately and watching the
// suite fail (dinah-369). No standing check reddens if the call is removed
// afterwards.

/**
 * Refuses a registration set that does not match the declared roster.
 *
 * The comparison reads both arrays as sets, so the order activate() happens to
 * register in does not matter and registering one id twice is not an error.
 */
export function assertCommandsFullyRegistered(
	declared: readonly string[],
	registered: readonly string[],
): void {
	const declaredSet = new Set(declared);
	const registeredSet = new Set(registered);
	const missing = declared.filter((id) => !registeredSet.has(id));
	const unexpected = [...registeredSet].filter((id) => !declaredSet.has(id));
	if (missing.length === 0 && unexpected.length === 0) {
		return;
	}
	const parts: string[] = [];
	if (missing.length > 0) {
		parts.push(`missing: ${missing.join(", ")}`);
	}
	if (unexpected.length > 0) {
		parts.push(`unexpected: ${unexpected.join(", ")}`);
	}
	throw new Error(
		`Dinah's command registration is incomplete (${parts.join("; ")}).`,
	);
}
