---
title: A guard greps for a variable name, so renaming the variable disarms it
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
One test in the extension's unit suite checks that a module does not read a field off a row before checking the row exists. It cannot call that module, because the module cannot be imported by the unit tests, so it reads the module's source and looks for a dereference of a variable spelled a particular way.

Rename the variable and the test stays green with the defect present. A reviewer planted exactly that and watched it pass.

Two other things caught the plant, so nothing is broken today: a partner test written beside it, and the compiler, which refused an earlier version of the plant outright. The gap is that the coupling is invisible. Nobody renaming that variable is told a test depends on its spelling, and the test that goes quiet says nothing when it does.

Reading source rather than calling code is an established idiom in this tree and is not by itself the problem. The problem is that this instance can be replaced with the real thing. The handler it guards closes over four values and touches nothing from the editor's own interface, so extracting it as a factory would let a test call the real handler with the real bad input and assert what it actually does. The logging hook such a test would need is already there and already injected; the handler bypasses it and writes to the output channel directly.

So the work is to make the calling module importable and to have the guard call it, rather than to strengthen the pattern match. Where a source-reading test remains, it should say in the test what it is coupled to, so the next person renaming that thing meets the coupling rather than silently disarming it.

Filed out of the first code review of dinah-342, recorded there as a minor rather than something holding that card. The shape it belongs to already appears twice in the workbench's convention counterexamples.
