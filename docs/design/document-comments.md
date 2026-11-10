# Comments on documents

A document can be discussed where it is edited. A comment thread is about a passage of the document, holds what the person and the agents said about it, and is open until someone resolves it. Threads are kept beside the document and never in its text, so that no reader of the document sees them, and a document whose source keeps comments of its own, such as a page on a site, exchanges its threads with the site's comments. Decided on 10 November 2026; the model and its rules come from the comment threads of the product Dinah was first derived from, which this generalises.

## Where threads are kept

`internal/threads` keeps each document's threads in a record of its own in the state folder, under the document's identity: its kind, workspace, source and path, or the identity its source's kind gives it (`workspaces.SourceKind.DocumentKey`), such as a page's ID on its site, which stays when the page is renamed or moved. A document that exists only as a draft keeps its threads under the draft, and they move to the document when it is first saved; Save As gives a copy the threads too. Every change is made under the record's lock, so the command line may write records as well.

A thread is anchored by its words, the quoted passage with the text around it, never by a marker in the text. `threads.Place` finds each thread in the current text, nearest where it was last seen, and a thread whose words are gone is shown as lost where they were, never dropped unannounced. A thread's messages are the opener, replies, a resolve and a reopen; a person's reply to a resolved thread reopens it. A thread records whether its last word is a person's (due, for an agent to answer) or an agent's (waiting on the person).

## The window

`document-comments.js` draws a document's threads in a margin beside the window's applet, hidden until there is a comment. Each card has the thread's kind and state, its quoted words (which go to the words in the text), its messages, and Reply and Resolve, or Reply to Reopen. New Comment… (Ctrl+Alt+M, in the Document menu) opens a thread on the words selected in the applet; Next and Previous Comment (F8, Shift+F8) walk the cards. Which comments are shown is one preference for every window: the open ones, all of them, or none; the count is shown whatever the choice, so hiding comments never hides that there are some.

The applets are yavc.app's and do not know the threads, so the margin stands beside them and finds a thread's words with the page's own find. Marking the commented words inside the applet waits for a decoration service from yavc.app.

## The site's comments

A source whose documents have comments of their own implements `resource.CommentReader` and `resource.CommentWriter` on its provider. `internal/documents` brings the site's comments in (`BringInComments`): each top-level comment becomes a thread, placed on the words the site says it was made on when the text still has them, or on the document's first line as a comment on the whole; its replies become the thread's messages; and a comment resolved or reopened on the site after the thread was last resolved or reopened here resolves or reopens the thread, with a message saying so. Each comment keeps the site's ID in `Native`, so bringing them in again adds only what is new. The window brings them in when it opens and once a minute. A message brought in from the site is marked as brought (`Message.Brought`), since a reader of the site wrote it and no agent was asked about it. The comments pane shows a thread whose last word was brought in as Open, where a thread whose last word the person wrote here is shown as For the agent. `Book.MarkBrought` marks the messages that were brought in before the mark existed, by the time the site gives for each comment.

Threads made here go to the site when the document is saved or published (`sendComments`, after the write): a new thread becomes a comment on its words, by the quote and which occurrence of it, or, when the writer cannot place it, a comment on the whole document quoting the words, with a note to the person; replies, resolves and reopens follow. What was sent is recorded with each message, so nothing is sent twice, and a reply to a comment the site no longer has is kept here alone with a note. A document whose source publishes offers Publish when it has unsent threads as well as when its text changed.

## Agents

The applet `dinah.document-comments` gives agents List Drafts, List Comment Threads, Comment on a Draft (open a thread on quoted words, reply, resolve) and Bring In the Site's Comments. A thread an agent opens reaches the site only when the person saves or publishes, so the actions declare reading alone.

## Tests

`internal/threads/threads_test.go` covers the record. `internal/documents/comments_test.go` brings a fake site's comments in and follows a resolution, sends a thread, its reply and its resolution with saves and keeps a reply to a deleted comment here, and keeps threads on a file without a site. The product's provider tests cover its reader and writer.
