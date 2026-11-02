# ESLint development dependency review

Release reviewed the ESLint 9.39.5 lock on 2 November 2026. The 86 added packages are development dependencies from registry.npmjs.org, each with SHA-512 integrity and no declared install hook. No existing locked package changed. CI installs this tree with npm ci, and the required Windows guard command runs ESLint and the npm audit gate. These tools are not included in the product.

Quality classified GHSA-g3hv-75wg-m249 in message 3682, and the lead approved the narrowly scoped exception in 3683. The [advisory](https://github.com/advisories/GHSA-g3hv-75wg-m249) describes attacker-controlled glob amplification in minimatch 10.2.6 at optimizationLevel >= 2, while its affected range includes all versions below 11.0.1. The locked minimatch 3.1.5 has none of the vulnerable preprocessing functions or the optimizationLevel option. Independently, ESLint does not request that optimization level, and our glob patterns come from repository-owned configuration and scripts.

The exception permits only that advisory on node_modules/minimatch at version 3.1.5, reachable solely from the exact ESLint 9.39.5 development dependency. The audit gate checks the lock graph even when npm reports no findings. A different advisory, another package path, another top-level dependency reaching that package, any other minimatch or ESLint version, a malformed report, or an audit execution error fails the guard. Inherited npm findings are accepted only when every advisory they lead to satisfies this exception.

The exception expires at 00:00 UTC on 2 December 2026. On that date the guard fails until the dependency and exception are reviewed. The allow-list is scripts/npm-audit-exceptions.json; no audit severity threshold is lowered.
