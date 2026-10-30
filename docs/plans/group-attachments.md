# Attachments From the Person in Group Chats

Paul asked on 28 October 2026 for a plan to let him attach files and pictures to his messages in a group chat, as he can in a single chat. He approved it the same day, and the Progress section at the end says what is built.

## How Attachments Work Today

A message on the bus can have attachments, but only an agent can attach anything. An attachment is a live link to a file inside the sending chat's workspace (`hashAttachments` in `internal/bus/bus.go`). The bus records the file's path and its SHA-256, so a recipient can tell whether the file has changed since it was sent. A recipient reads it with the `read_attachment` tool, and the bus allows that only to the sender and to the chats and workspaces the message was delivered to.

The person has no workspace on the bus, so there is nowhere for a file of theirs to live, and the bus refuses attachments from the person.

A single chat handles pictures differently. A pasted or dropped picture is saved in the chat's workspace, under the attachments folder of the profile's workspace folder, and goes to the agent as an image content block in the prompt itself (`attach.go`), so a model that reads images sees it at once.

## Proposal

### Where the person's files live

Each file the person attaches is copied into Dinah's home folder, under `attachments/` and the date. Dinah keeps its own copy instead of linking to the file where it was, because the person may move or change the original after sending, and a group's agents should all read what was sent. The bus records the copy as an attachment of the person's message, with its hash, as it does an agent's.

### How agents receive them

When a group message with attachments is delivered to a member chat, Dinah adds each picture to the prompt as an image content block, as a single chat does, provided the chat's provider and model accept images. Any other file, and a picture for a model that cannot read images, is listed with its name, type and size, and the agent reads it with `read_attachment`. The bus already limits that tool to the chats a message reached, and that rule extends to the person's attachments.

### What the person sees

The group window's message box accepts paste, drop and an Attach… command, with the same chips as a single chat's box, each with a Remove button. A sent message shows its attachments under the text, pictures as thumbnails that open in the media viewer and other files as links, which follow the document-links plan once it is built ([document-links.md](document-links.md)).

### Limits

The limits are a single chat's: the same largest file, and a number of files in one message that comes from the member chats' providers (see Decisions). A file over a limit is refused when it is added, with the reason, and is never sent.

## Decisions

Paul ruled on 28 October 2026 that Dinah keeps the person's attached files as long as the message that sent them, with a command in Settings that removes attachments older than an age the person chooses.

Paul ruled on 28 October 2026 that agents keep attaching files to group messages as they do to direct ones, as live links into their own workspaces.

He also asked for a limit on the number of files in one message, but not a hard-coded one, and asked whether providers and models have limits of their own. The Agent Client Protocol has no field by which an agent reports such a limit, so Dinah cannot learn it from the agent while it runs. Paul decided on 28 October 2026 that the limit is Files per Message, a setting on each provider's card in Settings › Agents, whose default comes from that provider's documentation and is shown beside the field with its source:

- Claude: 20. Anthropic's vision guide (https://platform.claude.com/docs/en/build-with-claude/vision) allows 600 images in one API request, and 100 for models with a 200k-token context window. When a request has more than 20 images, any image over 2000 px on a side is refused, and images sent in earlier turns count toward the 20. Dinah therefore also reduces any picture whose longer side is over 2000 px before a Claude chat is given it, keeping its proportions and its format.
- Codex: 20. OpenAI's images and vision guide (https://developers.openai.com/api/docs/guides/images-vision) documents a ceiling of 1,500 images and 512 MB in one request; 20 is Paul's choice, below it, and the card says both.
- Devin: no limit. Devin's documentation gives none, so the field starts empty, Dinah sends what the person attaches, and it shows Devin's error if Devin refuses a file.
- Ollama: no limit. Ollama's API documentation gives none, and the field starts empty on the same terms.

The limit applies to one message in a single chat's box, for that chat's provider. A group's message goes to several chats, possibly of different providers, so the group's box applies the smallest limit among the member chats that have one and says which member set it. A file over a member's own limit is still delivered to that member, listed for `read_attachment`, since the bus keeps it; only as many pictures as the member's limit go to it as image blocks. At the limit the box refuses a further pasted or dropped file with the reason and the setting's name, and Attach… is disabled with the reason as its tip, so the person learns of the limit before the message is sent rather than from an error afterwards.

## Order of Work

1. The store in Dinah's home folder and the bus's acceptance of the person's attachments, with their hashes.
2. Delivery to member chats, as image content blocks where the model reads images and as listed files otherwise.
3. The group window's paste, drop, Attach… and the drawing of sent attachments.

## Progress

All three steps are built, on 28 October 2026, together with the command in Settings that the decision asks for.

The bus keeps the person's files itself (`internal/bus/attachments.go`). `SendFiles` copies each file into the `attachments` folder of Dinah's home folder, under the local date, with the name made safe for a folder and a number added when the name is taken that day, and records it as an attachment of the message with its name, its type, its size, its SHA-256 and a mark that the bus keeps the copy. No workspace is ever written to. A file over 25 MB is refused with the reason a single chat gives, before anything is written, and the limit is now one constant that the single chat uses too. A message of files alone needs no words. The bus accepts the person's files for any address, so a message straight to a chat can have them as well, but the window offers attaching only in a group's box; the New Message form in the Group Chats pane and the replies in a thread window do not have it yet.

Files per Message is built, on 28 October 2026 (`file_limits.go`). The limits are kept in the settings by provider, only where the person chose other than the documented default, and State gives the page every provider's limit with its default, its documented ceiling, its source and the sentence the box shows. The shared box (`composer.js`) counts the files attached from the computer, pasted or dropped, holding a place for a file while it is read so that files dropped together cannot pass the limit, and refuses the rest by name; files chosen in the system's dialog past the limit are refused afterwards, since the dialog cannot be told how many to allow. Go works out a group's limit and the member that set it (`groupFileLimit`) and gives it with the group, and it checks the count again when a chat's or a group's message is sent. A delivery to a member chat gives as image blocks at most as many pictures as that chat's limit, and the prompt says which pictures it left listed for `read_attachment`. The reduction for Claude is in `picture_fit.go`: PNG and JPEG are resampled with `golang.org/x/image/draw` and written with the standard library's encoders, and each frame of a GIF is scaled by nearest neighbour onto its own palette. WebP has a documented decoder in `golang.org/x/image/webp` and no encoder, so a WebP picture over the size goes unchanged, and the prompt tells the agent so.

`read_attachment` lets the chats a message reached read its files, which for a group's message is every member chat it went to, and for a message to a chat is that chat; the rule is the one agents' attachments already had, now written down for both. A picture comes back as MCP image content, a file that is not text is described with its type instead of being returned as unreadable characters, and a file the person has removed is said to have been removed.

When a group's message reaches a member chat, Dinah asks whether the chat's agent takes images by the `image` field of the prompt capabilities it gave in its answer to `initialize`, as a single chat's pasted pictures do. The protocol has no way for an agent to say this per model, so a model that cannot see pictures behind an agent that says it takes them still receives them, which is the single chat's behaviour too. Where the agent takes images, each PNG, JPEG, GIF or WebP picture goes into the prompt as an image block, through the same code that makes a single chat's pasted pictures image blocks, and a sentence after the messages says which attachments they are. Every file, pictures included, is listed with its name, type and size for `read_attachment`, so an agent that takes no images reads them that way. A delivery steered into a running turn carries the pictures only when that turn can take pictures as steering; otherwise they are left listed.

The group's box takes a picture pasted into it, files dropped anywhere on the group's window and Attach…, which opens the computer's file dialog, and also stands in the window's menu. The chips, with their pictures, sizes and Remove, the reading of pasted and dropped files and the limit are in the shared box (`composer.js`), and the single chat's box now uses them from there as well. The files are not part of a group's draft, as a single chat's are not, so a draft outlasts a restart with its words and without its files. A sent message shows its pictures under the text as thumbnails that open in the media viewer, served by the window from the bus's copy and never from anywhere else, and every other file as a link that opens it as a link to a file in a chat's message does; links that open files inside Dinah wait for the document-links plan.

Settings › Chat has Attachments in Group Chats, where the person chooses an age, from one week to one year or any age, and Remove Attachments…. It first says how many files are older than that and how much space they take, asks before deleting, and then says what was freed. Each removed file's message stays, its attachment is shown struck through with the word Removed, and an agent that reads it is told it was removed.

Question 1 is unchanged: agents attach files to group messages as before, as live links into their workspaces.
