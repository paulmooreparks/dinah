// ESLint for Dinah's own page scripts: the desktop page's and the web
// pages' that Go serves (FE-20, docs/review/ledger.md), the trees the
// frontend policy tests read. ESLint is a development tool in the browser
// suites' locked package graph; npm run lint there runs it from here:
// eslint cmd/dinah-desktop/frontend internal/web/static
//
// Rules on now: a name declared twice in one scope, and a name declared and
// never used. no-shadow, a name that hides one of an enclosing scope, is
// turned on once its remaining findings are fixed with the split of
// chat-applet.js (the lead's ruling of 2 November 2026).
const OWN = ['cmd/dinah-desktop/frontend/**/*.js', 'internal/web/static/**/*.js'];

export default [
  {
    ignores: [
      'cmd/dinah-desktop/frontend/e2e/**',
      'cmd/dinah-desktop/frontend/wailsjs/**',
      '**/pudl/**',
      '**/vendor/**',
      '**/node_modules/**',
    ],
  },
  {
    files: OWN,
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
