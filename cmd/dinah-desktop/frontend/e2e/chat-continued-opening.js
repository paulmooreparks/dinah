// chat-continued-pinned.js, with the continuation's window being opened at the
// moment the old session's window is told of its continuation.
process.env.HEAD_CLOSED = '1';
require('./chat-continued-pinned.js');
