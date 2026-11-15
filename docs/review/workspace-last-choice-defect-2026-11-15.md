# Last workspace choice receiving report

The shipped page silently lost a return choice while another workspace was opening. A deterministic headless check delayed SetWorkspace(B), selected A, and completed B. The host and selector ended at B, and the only selection call was SetWorkspace(B).

The correction starts from shipped revision d202c0ac. The selector and recent-workspace controls now share a serialized selection queue. It retains the latest requested destination, skips superseded queued choices and keeps the selector on the last request. A final failure reports the existing error and shows the last workspace that actually opened. The decision is recorded in ../design/workspace-selection.md.

The new workspace-last-choice browser suite fails on the unchanged shipped code when the expected return call never arrives. It passes with the correction for both legacy path selection and saved workspace IDs. It also covers replacement of queued destinations, selecting the in-flight destination again, a newer request during the next call, final and queued failures, recovery after a failure, recent-workspace actions and selector frame samples. Every case keeps at most one host selection call active.

All eight selected headless suites pass together: workspace-last-choice, workspace-identity, workspace-transfer, new-workspaces, workspace-toolbar, taskbar-stacks, app-ui3 and section-order. The existing missing-folder confirmation and workspace-creation checks pass. Tests drive the shipped page through controlled App bridge responses; they do not claim a new compiled-host run. This change modifies no host code.

The branch contains the page correction, its headless suite and Dinah design/review notes. Tests use invented workspace records and isolated browser storage. No visible browser or desktop, native provider, push, merge or installation was used. Front End retains receiving corrections until accepted.
