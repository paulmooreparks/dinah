// ESLint for the desktop page's own scripts (FE-20, docs/review/ledger.md).
// It lives with the browser suites because their locked package graph holds
// the development tools, and it checks the frontend folder above. ESLint
// checks only files under the folder it runs from, so npm run lint runs it
// from the frontend folder with this file given: eslint -c e2e/eslint.config.mjs .
//
// Rules on now: a name declared twice in one scope, and a name declared and
// never used. no-shadow, a name that hides one of an enclosing scope, is
// turned on once its remaining findings are fixed with the split of
// chat-applet.js (the lead's ruling of 2 November 2026).
export default [
  {
    ignores: ['e2e/**', 'pudl/**', 'vendor/**', 'wailsjs/**', '**/node_modules/**'],
  },
  {
    files: ['**/*.js'],
    languageOptions: { ecmaVersion: 2022, sourceType: 'script' },
    linterOptions: { reportUnusedDisableDirectives: 'error' },
    rules: {
      'no-redeclare': 'error',
      // Arguments are part of a callback's shape, and a caught error may go
      // unnamed in use; neither is dead code.
      'no-unused-vars': ['error', { vars: 'all', args: 'none', caughtErrors: 'none' }],
    },
  },
];
