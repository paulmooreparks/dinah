# Launcher menu mnemonic

The Launcher command now declares A in the View menu. Its undeclared letter previously defaulted to L before Color Mode was read, causing Color Mode's existing L declaration to be refused. No other row declaration, command label or shortcut changes.

The keyboard suite now checks the rendered Launcher letter A and Color Mode letter L, alongside its existing assertion that no menu letter is refused. Keyboard, launcher-shortcut, title-launcher and host-menus-order pass all 203 assertions in 20 seconds on the branch from cbec7bf8. The hidden/headless receipts are under dinah-launcher-mnemonic in the temporary directory. The original integration failure remains in the retained full-run log.

Only these four browser suites were run. The full browser run, broad Go gate and installed receiving remain the integrator's work. No vendored files change.
