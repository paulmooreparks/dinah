#!/usr/bin/env bash
# make-fixture.sh writes the storage migration fixture of dinah-637 through
# verbs, with whatever dinah binary it is given, into before/ beside this
# script. The committed fixture was written by the build at the merge base of
# the dinah-637 branch, before any code of that card, and is never regenerated
# by a later build: a later build writes the new layout, and the fixture's
# whole purpose is the old one.
#
# Usage: make-fixture.sh <dinah binary> <scratch directory>
#
# The scratch directory must not exist. The script creates the workbench
# there, writes it, and copies the store into before/, replacing whatever
# before/ held.
set -euo pipefail

bin=$1
scratch=$2
here=$(cd "$(dirname "$0")" && pwd)

mkdir "$scratch"
export DINAH_HOME="$scratch/home"
export DINAH_ACTOR=sam
unset DINAH_WORKBENCH DINAH_HARNESS DINAH_PROVIDER DINAH_MODEL DINAH_SERVER

"$bin" init "$scratch/wb" --from "$here/definition.json" --slug fx --operator sam > /dev/null
store=$(ls -d "$scratch"/wb/.dinah/*)
d() { "$bin" --workbench "$store" "$@" > /dev/null; }

printf 'the first payload\n' > "$scratch/card-payload.txt"
printf 'a payload on a card comment\n' > "$scratch/comment-payload.txt"
printf 'a payload on an item comment\n' > "$scratch/item-comment-payload.txt"

d add "the main card"
d add "the card that will be archived"
d add "the card with hand-edited comments"
d add "the card that stays in intake"

# Card comments on the main card: plain, archived, deleted, empty, and one
# carrying an attachment.
d comment fx-1 "a first card comment, which carries an attachment"
d attach fx-1/comments/1 "$scratch/comment-payload.txt" --description "the card comment's payload"
d comment fx-1 "a second card comment, archived afterwards"
d comment fx-1 "a third card comment, deleted afterwards"
printf '' | "$bin" --workbench "$store" comment fx-1 - > /dev/null
d comment fx-1 "a card comment with <angle brackets> & an ampersand"
d attach fx-1 "$scratch/card-payload.txt" --description "the main card's own payload"

# Acceptance criteria in every state an acceptance criterion reaches.
d file fx-1 acceptance_criterion "the first criterion stays pending" --owner operator
d set fx-1/ac/1 evidence test
d file fx-1 acceptance_criterion "the second criterion is verified by text"
d cite fx-1/ac/2 test internal/verb/mutate_test.go#TestSecond --observed fail:pass
d cite fx-1/ac/2 record "the review note"
d verify fx-1/ac/2 --text "the second criterion was checked against the fixture"
d file fx-1 acceptance_criterion "the third criterion fails by reference"
d comment fx-1/ac/3 "the handler still answers 200"
d fail fx-1/ac/3 fx-1/ac/3/comments/1
d file fx-1 acceptance_criterion "the fourth criterion is waived"
d waive fx-1/ac/4 --text "the operator let the card go on without it"
d file fx-1 acceptance_criterion "the fifth criterion is withdrawn"
d withdraw fx-1/ac/5 --text "the card was narrowed and the check went with it"
d file fx-1 acceptance_criterion "the sixth criterion was verified and reopened"
d verify fx-1/ac/6 --text "it looked right the first time"
d reopen fx-1/ac/6 "the fixture predated the fix"

# Open questions and decisions.
d file fx-1 open_question "does the deadline move?" --owner operator --column review
d resolve fx-1/oq/1 --text "the deadline is the 15th"
d file fx-1 open_question "which vendor supplies the payload?"
d comment fx-1/oq/2 "an item comment carrying an attachment"
d attach fx-1/oq/2/comments/1 "$scratch/item-comment-payload.txt" --description "the item comment's payload"
d comment fx-1/oq/2 "an item comment archived afterwards"
d comment fx-1/oq/2 "an item comment deleted afterwards"
d comment fx-1/oq/2 "an item comment that stays"
d file fx-1 decision "the write path takes the card's own lock"
d comment fx-1/d/1 "it mirrors what comment and attach already take"
d resolve fx-1/d/1 fx-1/d/1/comments/1
d file fx-1 decision "a decision archived afterwards"
d comment fx-1/d/2 "a comment on the archived decision"
d file fx-1 decision "a decision deleted afterwards"
d set fx-1/oq/2 text "which vendor supplies the second payload?"

# The standing item a move into Doing mints.
d move fx-1 doing

# The card that will be archived, carrying members of both kinds.
d comment fx-2 "a comment on the card that will be archived"
d file fx-2 acceptance_criterion "a criterion on the card that will be archived"
d comment fx-2/ac/1 "an item comment on the card that will be archived"
d move fx-2 done

# The card whose comments are edited by hand: one loses its author, one has
# its body changed after its digest was recorded, and one is minted by an
# unblock.
d comment fx-3 "a comment whose author is removed by hand"
d comment fx-3 "a comment whose body is edited by hand"
d block fx-3 "a vendor has not answered" --kind external
d unblock fx-3 "the vendor answered"

# Column comments: one on a live column, one on a column that is archived.
d comment review "a comment on the review column"
d comment parked "a comment on the column that is archived"
d archive parked

# Archives and deletions, made after every member exists, so no ordinal is
# reissued inside the fixture. Each collection is taken highest position
# first, so every reference below names the member it meant.
d delete fx-1/comments/3 --yes
d archive fx-1/comments/2
d delete fx-1/oq/2/comments/3 --yes
d archive fx-1/oq/2/comments/2
d delete fx-1/d/3 --yes
d archive fx-1/d/2

# Hand edits, made last so no verb rewrites them.
card3=$(ls "$store/cards" | while read id; do
	if grep -q "the card with hand-edited comments" "$store/cards/$id/card.md"; then echo "$id"; fi
done)
for c in "$store/cards/$card3"/comments/*/comment.md; do
	if grep -q "author is removed by hand" "$c"; then
		sed -i -e '/^author: /d' -e 's/^ts: \(.*\)$/ts: \1\nauthor_unrecoverable: true/' "$c"
	fi
	if grep -q "body is edited by hand" "$c"; then
		sed -i 's/a comment whose body is edited by hand/a comment whose body was edited by hand, after its digest was recorded/' "$c"
	fi
done

rm -rf "$here/before"
mkdir "$here/before"
cp -r "$store"/. "$here/before/"
basename "$store" > "$here/workbench-id.txt"
echo "fixture written from $store"
