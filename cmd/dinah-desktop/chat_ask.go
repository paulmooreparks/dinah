package main

import _ "embed"

// Asking another chat travels over the bus, so there is one way for chats
// to reach each other (docs/plans/group-chats.md, One way to reach a chat).
// An invocation of dinah.chat/ask@2 sends its question as a bus message of
// the question kind from the asking chat's session on the bus to the asked
// chat's (bus.Ask). Being sent to the chat itself, the question wakes it as
// any such message does: a closed chat is opened in the background, and a
// window restored at the prompt to reconnect is reconnected (bus_wake.go).
//
// The bus never hands a question over as an ordinary message (bus.Deliver).
// The invocation waits until the asked chat is connected, takes the
// question from the bus (bus.TakeQuestion) and runs it as a turn of that
// chat whose words it keeps (askWait), steered into a busy chat's turn when
// its provider and model allow it. The asked chat's log shows the question
// as the card of the bus message, and the turn's words are the answer,
// which becomes the invocation's result; the saved result then reaches the
// asking chat as a completion report on the bus, as it did before
// (chat_delivery.go), and so do an error, a cancellation and an expiry.
//
// A question whose invocation ends without taking it is withdrawn, so it no
// longer wakes the chat. Questions left by a window that stopped while they
// waited are withdrawn at the watcher's next round, since no invocation of
// this run waits for them (withdrawOrphanQuestions); the broker reports
// their invocations to the asking chats as failed.

//go:embed chat_ask.go
var chatAskSource []byte

//go:embed bus_wake.go
var chatWakeSource []byte
