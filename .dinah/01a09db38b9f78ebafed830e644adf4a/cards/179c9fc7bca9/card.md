---
title: The manual verification path stops recommending a command that can go missing
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The README offers a manual route for somebody who would rather not pipe a script into a shell: download the binary, then check its checksum by hand. On Windows it tells them to use the same command the installer was just changed to stop depending on, because that command is not built in and disappears on a machine carrying another PowerShell edition's modules. The person following the careful path therefore meets the failure the automated path no longer has. It fails loudly and installs nothing, so nobody is endangered, but the instruction is wrong on the page a stranger reads first.
