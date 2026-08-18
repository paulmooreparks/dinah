# dinah-101 UX sketch: one way to render a row

This sketch shows what a person sees change when every row goes through one
renderer. Each case carries the output Dinah produces today at commit
`1ec2428`, the output the card's contract produces instead, and the display
column each field starts in.

Read the column numbers rather than the transcripts where the two disagree. A
Markdown viewer draws Devanagari and Japanese in whatever font it has, and that
font need not agree with your terminal about how wide a glyph is. The column
numbers are what Dinah computes, and they are what this change fixes.

Every block below marked "after" was produced by applying the card's layout
rules to the real strings Dinah printed, so no transcript here is invented.

## Case 1: two rows of the command list glue their summary onto their usage

`dinah` with no arguments prints the whole surface. The usage column is 39
wide. Two entries outrun it, and the padder gives an overrunning field a single
space instead of a column, so those two summaries start wherever their usage
happened to end. No locale is involved. You see this in English on the first
screen the tool ever shows you.

### Today

```
  status                                 where this workbench stands, and what you hold
  init [--from <source>] [--slug <slug>] [--operator <actor>] create a workbench here, optionally from a template
  check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states] look for structural defects in this workbench
```

The summary of `status` starts at display column 41, the summary of `init` at 62, and the summary of `check` at 77.

### After

```
  status                                 where this workbench stands, and what you hold
  init [--from <source>] [--slug <slug>] [--operator <actor>]
                                         create a workbench here, optionally from a template
  check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states]
                                         look for structural defects in this workbench
```

Every summary in the block now starts at display column 41. A usage line that
reaches its column takes the rest of its own line, and its summary resumes
underneath at the column it belongs to. Dinah abbreviates nothing and truncates
nothing.

## Case 2: the refusal column of a Hindi command help drifts by a column

`dinah help add --lang hi` prints one row per precondition, with the refusal
name after a column 52 wide. Devanagari writes its vowels as combining marks,
and a combining mark occupies no column of its own. The padder counts
characters, so each row pays for its own marks and comes up short by however
many it happens to carry.

### Today

```
  1  निवेदन में शीर्षक है                                malformed
  2  नामित स्थिति वर्कबेंच घोषित करती है                 unknown-state
  3  नामित स्थिति अपनी सीमा से नीचे है                   at-capacity
```

The refusal names start at display columns 52, 52, 53, and they are meant to start at the same one.

### After

```
  1  निवेदन में शीर्षक है                                     malformed
  2  नामित स्थिति वर्कबेंच घोषित करती है                      unknown-state
  3  नामित स्थिति अपनी सीमा से नीचे है                       at-capacity
```

All three refusal names start at display column 57, which is the two-space
indent, the three-column number, and the 52-column check text added up. The
rows carry the same words they carried before.

## Case 3: a workbench named in Japanese pushes its own row to the right

`dinah workbenches` prints the title in a column 32 wide. A Japanese character
occupies two columns on screen and counts as one character, so a title of five
characters is padded as though it drew five columns when it draws ten.

### Today

```
  作業台管理                           inner           C:\dinah-scratch\dinah-101-spec\wb\作業台管理\.dinah\ac2ee28fb26d
```

The slug column starts at display column 39, and the declared layout puts it at 34.

### After

```
  作業台管理                      inner           C:\dinah-scratch\dinah-101-spec\wb\作業台管理\.dinah\ac2ee28fb26d
```

The slug column starts at display column 34 and the path column at 50, which
is where a row of Latin titles has always put them.

## Case 4: a narrow window no longer receives an indent wider than itself

When a field reaches its column, the fields after it resume on a continuation
line indented to that field's own column. That indent is computed from the
declared widths and knows nothing about the window. In a window 40 columns
wide, an indent of 34 leaves you a fragment floating in whitespace.

The contract clamps the indent so a continuation line always keeps at least 20
columns for its own text. The row below carries a workbench title that reaches
the 32-column title column, and it is drawn twice: once as Dinah draws it
today, and once under the contract at `COLUMNS=40`.

### Today

```
  A workbench title long enough to reach its column
                                  inner           C:\dinah-scratch\...\.dinah\ac2ee28fb26d
```

### After, at `COLUMNS=40`

```
  A workbench title long enough to reach its column
                    inner           C:\dinah-scratch\...\.dinah\ac2ee28fb26d
```

The continuation indent drops from 34 to 20, which is the window's 40 columns
less the 20 the contract reserves for text. The path carries no space to break
at, so Dinah emits it whole and lets the terminal wrap it where it likes. Dinah
breaks no word at any width, because a path broken across two lines is a path
you cannot copy.

## What you are being asked to accept

Four things, and each of them appears in a block above.

- A field that reaches its column ends its line, and the rest of the row
  resumes underneath at the column it belongs to. Cases 1 and 4 show it.
- Padding counts the columns a string occupies on screen rather than the
  characters it is made of. Cases 2 and 3 show it.
- A continuation line keeps at least 20 columns for its own text, whatever the
  declared layout asked for. Case 4 shows it.
- Dinah truncates nothing and breaks no word, at any window width.
