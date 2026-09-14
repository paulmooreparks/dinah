---
title: A person cannot see which attachment is missing its file
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
An attachment whose payload is gone still reads back. It keeps its name, its description and its place in the count, and only its path is absent. That is deliberate, because an attachment that vanished from a listing would be worse than one that cannot be opened.

A machine reading the answer can tell the two apart, since the path key is simply not there. A person cannot. The human table draws the row with the missing file exactly as it draws every other row, so the operator running a check over an older workbench sees nothing wrong and learns only when something tries to open it.

That is the wrong way round. The machine form is read by clients that can branch on a missing key, and the human form is read by the person who would actually go and find the file.

What this card asks for is that the human form say which rows have no reachable file. What that looks like is the work: a marker on the row, a line beneath it, or something the check reports separately, and each reads differently on a workbench with one bad attachment versus one with thirty.

Whatever is chosen must not require the reader to compare two outputs to notice. The current answer already technically contains the fact, in the sense that the path column is empty, and that is exactly the kind of difference nobody sees.

Filed out of the test pass on dinah-334, where the tester reproduced a missing payload against a live workbench and found the human tables identical either way. It is distinct from dinah-340, which is about a record built at creation time carrying no path; that one is a value the code never filled in, and this one is a real absence the reader is not shown.
