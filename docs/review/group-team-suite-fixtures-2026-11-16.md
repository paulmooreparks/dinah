# Group and team browser fixture repair

The three reported failures reproduce on master c35d83fc. They are fixture faults.

The bus and group-chats-background fixtures supplied foreign-workspace chats through SessionsAt, but their authoritative SavedChats fallback contained only the current workspace. Each fixture now supplies its complete invented inventory through SavedChats. The original cross-workspace, picker, state and background-opening assertions remain unchanged.

The teams fixture activated the lead window through pudlWindows.raise while the Team Report popover remained open after following an assignment link. The popover intercepted the next click. Its helper now performs Escape and observes the report closing before programmatic window activation. It uses normal keyboard dismissal, with no forced click, timeout extension or retry. The assignment-link and subsequent live-state assertions remain unchanged.

The repaired bus, group-chats-background and teams suites pass with chat-inventory, team-navigation, team-toolbar, teams-relay-details, teams-tail and teams-traffic. All nine pass in 19 seconds through the canonical shared queue using headless invented fixtures, with no page errors. Baseline and receiving logs remain private under the temporary dinah-bus-suite-baseline and dinah-bus-suite-repair folders. No raw compiler output is committed.

This branch changes only the three suites and this note. It claims no installed-window or separate fork acceptance. The exact commit and final profile/private-term/message/diff guard receipt accompany the handoff. Lead receives the standalone correction; Front End retains fixture corrections. Nothing was pushed or installed.
