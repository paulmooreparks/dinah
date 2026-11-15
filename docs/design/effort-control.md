# Effort Control

On 15 November 2026 Paul pointed out that the Thinking button in the composer's toolbar is much wider than its neighbours, with "Medium" centred in empty space. App.css fixes the button's words at 9rem for every value so that it never changes size. He suggested replacing it, and the team lead took his suggestion: the Thinking button becomes an Effort button whose label never changes, pressing it opens a PUDL segmented control with the levels the chat offers, and a chat that offers no effort setting shows no button, as today.

The clickable mockup is `docs/mockups/effort-control/index.html`, and opened as a plain file it starts on the composer. It loads Dinah's shipped PUDL, brand, dinah and app stylesheets, the palette the real window applies, and the shipped PUDL menu, command and toolbar scripts, which draw the toolbar from a command table shaped like the chat applet's. The proposed rules are in `effort-control.css`. Pictures of every state at 1440 and 390 pixels, in light and dark, are in `pictures/`, and `explore/walkthrough.js` uses every part headless and checks it.

## The control

The button shows the gear glyph, the word "Effort" and a small mark that is always 26 pixels wide. The mark has one cell for each level the chat offers, filled up to the level in effect, so it shows both the level and the range at a glance. A change of level only fills or empties cells, so the button and everything beside it stay where they are. The button is the same width for three, five and one levels. A screen reader hears "Effort" and then the level, such as "medium, level 2 of 3".

Pressing the button opens a panel with the levels as a PUDL segmented control, in the order and with the names the agent program reports. Dinah invents no level names. The level in effect is pressed in, and its description, when the agent gives one, shows under the control. One press on a level chooses it, closes the panel and returns focus to the button.

From the keyboard, Enter or Space on the button opens the panel with focus on the level in effect. Left and Right move along the levels and show each one's description, Home and End go to the ends, Enter or Space chooses, and Escape closes the panel without a change. Focus returns to the button either way.

The button is a PUDL command with a panel of its own, which pudl-commands.js already supports. When the toolbar folds on a narrow window, the same panel draws inside the Agent group's menu, under one Effort heading.

A chat with one level shows the button with a single full cell, and its panel says the model offers one effort level, so there is nothing to choose. A chat with no effort setting shows no button.

When a level is chosen while a turn is running, the panel says beforehand, and the working line says afterwards, what that agent program documents about the change, and nothing more. Engine checked each program's documentation (`docs/review/effort-change-protocol-2026-11-16.md` on `engine/effort-protocol`). Codex documents that the effort belongs to the start of a turn, so for Codex the turn already running keeps its level and the new one applies from the next turn. Claude and Devin, through ACP, do not say whether a change applies to the turn already running, and the panel says exactly that. Ollama uses the new value from its next request to the model, which may come before the turn ends, and the panel says that. The mockup also has a state showing today's Thinking button for comparison.

## What the mockup does not settle

- A chat whose agent offers more than one thinking setting at once is not mocked. The panel would show one segmented control for each, with the setting's name above it, and the mark would follow the first.

## Questions for Paul

1. Should a chat with only one level show the button, as mocked, or hide it as a chat with none does? I recommend showing it, because it tells you the model has a fixed effort without you having to look elsewhere.
2. Is the cell mark clear enough at a glance, or would you rather the panel were the only place the level shows? I recommend the mark, because it costs no width and shows the level without opening anything.
