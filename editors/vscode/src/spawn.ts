// The real spawner, and the filesystem the resolver reaches through.
//
// This module and this module alone imports node:child_process. A repository
// check asserts that, which is how "the extension never parses dinah's human
// output" stays enforceable instead of merely reviewed: a second spawn site
// anywhere under src/ turns the check red, and every route through this one
// goes past cli.ts, which composes --json itself.

import { execFile } from "node:child_process";

import type { SpawnOptions, SpawnOutcome, Spawner } from "./cli";

/** How long one dinah invocation is given before it is abandoned. */
export const SPAWN_TIMEOUT_MS = 15_000;

/** Runs one process and resolves with what it produced, never rejecting. */
export const nodeSpawner: Spawner = (
	exe: string,
	argv: readonly string[],
	options: SpawnOptions,
): Promise<SpawnOutcome> =>
	new Promise<SpawnOutcome>((resolve) => {
		execFile(
			exe,
			[...argv],
			{
				cwd: options.cwd,
				env: options.env,
				timeout: SPAWN_TIMEOUT_MS,
				maxBuffer: 8 * 1024 * 1024,
				windowsHide: true,
			},
			(error, stdout, stderr) => {
				if (error && typeof (error as { code?: unknown }).code === "string") {
					// execFile reports a failure to start with a string code
					// (ENOENT, EACCES) and a failure of the process itself with
					// a numeric one, so the type of the code is what separates
					// "there is no such binary" from "it ran and refused".
					resolve({
						code: null,
						stdout,
						stderr,
						spawnError: {
							code: (error as NodeJS.ErrnoException).code,
							message: error.message,
						},
					});
					return;
				}
				const code =
					error && typeof (error as { code?: unknown }).code === "number"
						? ((error as unknown as { code: number }).code)
						: 0;
				resolve({ code, stdout, stderr });
			},
		);
	});
