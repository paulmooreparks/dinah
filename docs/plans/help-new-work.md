# Help for the new chat controls

The help work is on `frontend/help-new-work`, based on `5db16def`. Help previously offered Keyboard Shortcuts and About. It did not open a guide under docs. Help now offers Working with Chats, a read-only window using the existing PUDL window host and its saved address.

The guide covers Decisions, broadcasts, Pause Chats and Restart, kept attention cards, the background-chat limit and the optional Updates page. Each section says where to find the controls and what happens when they are used. It distinguishes filling a recommended answer from sending, previewing recipients from broadcasting, and installing an update from restarting. The restart section includes the same-program restart assigned to Engine in parallel; this branch does not implement or exercise that change.

Keyboard Shortcuts now includes the new View commands without assigning new key combinations. Its Keys For list also describes the Decisions buttons, the application menu's restart choices, and the Settings controls for background chats, updates and agent-settings repair. The existing Notifications key remains Ctrl+Shift+I. The new rows describe existing menu or button access and change no command dispatch or feature operation.

The new guide uses static markup in index.html and a small help-content.js module. No backend, service, Decisions layout, update logic or restart logic changes on this branch. The Architectural Principles remain at `C:\Users\paul\OneDrive\Documents\Architectural Principles.md`.

## Verification

Verification is pending the shared FIFO. Only the affected keyboard suite and the new guide navigation suite are queued, both headless. The latter checks Help menu access, an address that survives reload, a single guide window and absence of feature-operation calls while reading help. ESLint has passed for help-content.js.

No visible window, desktop input, live recipient, real settings repair, restart, installation, push or merge has been used. The browser fixture cannot establish native installed-program behavior. Ownership stays with Front End until Lead accepts the branch.
