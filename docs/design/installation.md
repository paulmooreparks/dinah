# Installation

Paul decided on 19 November 2026 that a product's programs are not installed in its home folder's bin. That folder is `/bin` in the product's virtual file system, which holds the standard scripts its own shell runs (`cmd/dinah-desktop/system_vfs.go`), so the window program, the command line and every copy of them that an update set aside were listed in `/bin` and offered as commands of the shell. By then `~/.dinah/bin` held several hundred such files.

## The program folder

A product's programs are installed in its program folder, which `programs.Folder` in `internal/programs` gives. On Windows it is the product's name in the person's own program folder, the known folder `FOLDERID_UserProgramFiles`, which is `%LOCALAPPDATA%\Programs` unless it has been moved, so Dinah is in `%LOCALAPPDATA%\Programs\Dinah`. The folder needs no administrator, so the product installs on a computer whose person cannot install for every user; Paul checked on 19 November that his locked-down laptop lets him create it. Elsewhere it is the product's name under the XDG data folder's `programs`. The variable `DINAH_PROGRAMS`, or the product's own prefix with `PROGRAMS`, sets another folder, which a check that runs an installation of its own uses.

Everything that looks for an installed program asks `programs.Folder`: Restart to Update, which watches the window program there; the Updates page's set-aside copies, which are beside the programs; the tool servers registered with agents; the drawing program; and the move of programs to their current names. The command line, the stand-alone document applications and the Devin hook command are found beside the window program, so they follow it.

## Finishing an installation

The window program, started with `--install` from the program folder, finishes an installation (`cmd/dinah-desktop/install_step.go`). The command line's `install` runs it after placing the programs, and so do a product's installer and its updater. It does five things, each of which changes nothing when it is run again.

1. It installs the stand-alone document applications beside itself, as `--document-install` does, when the product's profile offers them (`DocumentApplications`). A product whose profile does not offer them installs none, and this step removes any that an earlier installation of it added: the program beside the window program, its place in Open With, and its Start menu shortcut when that shortcut starts this product's program. A shortcut of the same name that starts another product's program is left alone (Paul, 24 November 2026).
2. It makes the window's shortcut in the person's Start menu programs, under the product's name.
3. It points every other shortcut at the top of the Start menu programs that starts the window program in the home folder's bin at the program folder's instead, so a shortcut the person made keeps working. The taskbar's pinned programs are kept in a folder Windows does not document, so they are left alone, and a pin to the earlier program is the person's to make again from the Start menu.
4. It moves the product's programs, and the copies of them that updates set aside, from the home folder's bin to the program folder (`programs.MoveOut`). A program found there under its own name becomes a set-aside copy, so the Updates page lists it and can remove it once nothing runs it. A file that cannot be moved, which happens to a running program only when the two folders are on different drives, stays and is moved by the next installation. The standard scripts and anything the person put in the bin are left where they are.
5. It puts the program folder on the person's own PATH, `Path` under `HKEY_CURRENT_USER\Environment`, in place of the home folder's bin where that was on it, and at the end otherwise (`programs.EditPath`, `programs.SetUserPath`). The value keeps its type and the other entries keep their order and spelling, and running programs are told with `WM_SETTINGCHANGE`.

The agent programs' settings may run a program from the home folder's bin by its full path: Claude Code's hooks and MCP servers in `~/.claude/settings.json` and `~/.claude.json`, and Codex's MCP servers in its `config.toml`. Those files are the person's, and an installer meant for anyone who downloads a product must not edit them unreviewed, so the installation does not change them (Paul, 19 November 2026). It lists the ones that still have a path into the bin, with how many, and says to change those paths to the program folder (`settingsUsing`). A product's own repair of its agents' entries, which shows the person what it would change first, is the other way to correct them.

A copy started with the home variable or the program folder variable set leaves the document applications, the Start menu and the PATH alone, since such a copy is a check and must change nothing of the person's.

## Moving an existing installation

A window of an earlier build, running while the programs are moved, still watches the home folder's bin, so Restart to Update never sees the first installation into the program folder. The person closes that window and starts it from the Start menu once, which Paul accepted on 19 November 2026.

A window of this build that finds itself running from the home folder's bin moves the installation itself (`moveInstallation` in `cmd/dinah-desktop/update.go`). That happens to a person whose earlier release's updater put this release in the bin, and to one who starts it from an old shortcut or pin. When the program folder has no window program, the window places the programs beside it there, the window program last, and runs `--install` on the new one, which moves what is left in the bin. Whether or not it placed anything, it then offers Restart to Update into the program folder's window, and every later update is seen. A copy started with a home or program folder of its own does nothing, as with the other parts of an installation.

## A product's own installer

A product may publish its programs as a release that a person installs by hand, with a script beside the release that installs it, or updates an installation, without an administrator. Such an installer places the programs in the program folder and runs `--install` on the window program, as the command line's `install` does, so a product's installer, its updater and Dinah's own installation finish the same way.

## Tests

`internal/programs/folder_test.go` covers the folder and its variable, the move, which takes the programs and their copies and nothing else, keeps a name that is taken under another, makes a program found under its own name a copy the Updates page lists, and moves nothing the second time, and the PATH edit, including entries spelled with variables, in another case, or twice.
