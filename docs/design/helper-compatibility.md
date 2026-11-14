# Helper compatibility

The helper protocol is independent of the release number. Every channel starts at protocol 1, which is also the meaning of an absent number. A compatible addition does not change that number. An incompatible exchange changes only its channel's number. Each side lists the numbers it can serve; the highest common number is selected before operations. An empty intersection refuses without dispatch and names the newer side, the channel and the need to restart the window so matching helpers start. Malformed numbers refuse rather than become legacy 1.

## Existing exchanges

| Channel | First exchange before this change | Existing compatibility check |
| --- | --- | --- |
| Window bus MCP | `--bus-mcp --state-dir --session` opens SQLite, reads the session, then serves MCP stdio to the agent. The window watches that same database. | MCP negotiates its external protocol with the agent; no helper/window protocol exists. |
| Hosted bus MCP from the command program | The program opens SQLite and either stands down in a window or serves the registered session. | No helper/window number exists. |
| Steering and session hooks | Provider JSON arrives on stdin, then the helper reads or changes SQLite hook/steer records. | Provider hook formats exist; no helper/window number exists. |
| Applet helper | A capability identifies an endpoint-only descriptor; the helper POSTs a JSON operation to `/rpc` with its bearer token. | No number is exchanged. Descriptors and bodies reject unknown fields. |
| Document control | A descriptor supplies address, token, PID and release text. Find GETs `/control/ping`, then operations use the same token. | Release text is reported but not checked as a protocol. Ensure treats all Find failures as permission to start another service. |
| Extension package control | The CLI reads a host descriptor and POSTs pause/resume before changing a package. | No number is exchanged. |
| Extension backend and callback | MCP initialize and signed action metadata precede tool calls. Backend callbacks POST to `/host` with a scoped token. | External MCP version negotiation exists. Callback JSON has no host protocol number; the SDK is a separate repository. |
| Detached slide-show wrapper | The parent supplies an entry URL and display rectangle in environment variables; the wrapper loads the parent's page and generated bridge. | The browser script and bridge are served together, but the compiled wrapper has no parent compatibility check. |

Agent providers' ACP, Codex app-server, Ollama HTTP and MCP transports retain their own documented protocols. They are external systems rather than installed Dinah helpers. The shell page and generated bridge are served by the same process. An extension backend's SDK is not changed by this branch; its existing callback is legacy protocol 1, while the receiving host will advertise and guard that channel.

The separate SDK follow-up is to add the same offer, read-only probe and response check to its callback client and publish it for extension producers. It must retain legacy 1 while existing extensions remain supported. This branch neither edits that repository nor claims a new SDK producer was verified.

Document control names the document service and document client in its refusal. Restarting only the main window cannot replace a service that still has document windows attached. The message therefore also asks the person to save and close those windows and wait for the idle service to exit. Negotiation never stops a service or discards unsaved documents.

## Exchange and legacy operation

HTTP channels use `X-Dinah-Protocols` for offered positive integer numbers and `X-Dinah-Protocol` for the selected number. The server advertises its offer even on incompatibility. These are headers so strict legacy JSON readers and descriptors remain unchanged. A read-only OPTIONS probe precedes any potentially mutating request. A legacy refusal of OPTIONS with no protocol header means protocol 1; the helper continues only if it supports 1. A network error is not an absent number. Every actual request and response is checked too, and incompatibility returns HTTP 412 before decoding or dispatching operations.

SQLite bus and hook helpers use a read-only `/helper-compatibility` endpoint on the window's existing loopback server before opening the database or serving tools. The window supplies that address through its profile-prefixed HELPER_ENDPOINT environment variable, inherited by the agents and their helpers. It sends no secret and grants no operation. The endpoint refuses browser-origin requests and incorrect Host values. Missing environment means the legacy window and protocol 1. The detached slide-show wrapper checks the same endpoint on its own channel before creating a window. No new flag is passed to a legacy executable. No side file is added that could outlive a window and redirect a later launch.

The bus and steering helpers also run in terminal sessions with no window. They carry on when the endpoint is absent, unreachable or takes longer than the 500 ms check budget. A response without a number is legacy 1. Only an answered incompatible protocol offer stops those helpers; standalone SQLite operation keeps its previous behavior. HTTP operation channels still refuse a transport failure rather than treating it as a compatible peer.

The document client must preserve a compatibility refusal through Ensure; it must never erase the endpoint or start another service on that refusal. Extension control must likewise preserve the refusal and skip package mutation. Current producers offer only 1. Tests use injected policies to cover equal, newer, older, supported older, malformed and absent numbers on each owned channel, with dispatch counters and unchanged invented state. No live endpoint, real home or desktop input is used.
