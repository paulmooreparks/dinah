# Readings of the core profile

This file records every place where `core-profile.md` (`dinah-core 0.17`) admitted more than one reading, or said nothing about something the reader had to decide, together with the reading `reader.py` implements. The reader's code cites these entries as R-1 to R-24.

The brief allows the five letters of the word that follows "work" in "workbench" to appear together only inside that word. Identifiers of that family are therefore written here with the numeric character reference `&#66;` in place of their first letter after the hyphen, so that CORE-&#66;ENCH-1 is the identifier the profile publishes on the line that begins `[CORE-` and ends `-1] A workbench definition MUST carry a title.` A Markdown renderer shows each one exactly as the profile spells it.

## R-1: The version identity the reader reports

Statements: CORE-VER-2.

Profile text: Opening lines: "Version identity: `dinah-core 0.17`, maturity channel `dev`." Section 2.1: "A conformance claim names `dinah-core 0.17` and says nothing about the channel, because the channel belongs to the document's history and the number belongs to the contract."

Readings considered: The version identity could be the whole of the opening line, including the maturity channel. It could be the text of the code span that follows "Version identity:", which names the profile and its version without the channel.

Chosen: The reader reports the text of the first code span that follows "Version identity:" in the first ten lines of the profile it is given, read at run time, which for this revision is dinah-core 0.17. It takes the identity string and the major and minor numbers it compares against from that same text.

Why: The opening line names the version identity and the maturity channel as two separate things, and section 2.1 says the channel belongs to the document's history rather than to the version.

## R-2: How the extraction reads lines

Statements: Every identifier the extraction returns is affected, and no single statement governs the choice.

Profile text: Section 3.2: "A normative statement occupies exactly one line." Section 3.2: "An extractor reads the document line by line and needs no Markdown parser:" Section 3.2: "so an extraction over a valid revision returns no identifier twice."

Readings considered: A line could end at a line feed alone, or also at a carriage return. Lines inside fenced blocks could be skipped or read like any other line. An identifier found twice could be reported twice or once.

Chosen: The reader decodes the profile as UTF-8, drops one leading byte order mark, splits the text at each line feed, removes one carriage return that ends a line, and applies the expression of section 3.2 to every line, including lines inside fenced blocks. It keeps the first occurrence of a repeated identifier and reports the repeat on standard error. A profile that is not UTF-8, or that yields no statement, ends the run with exit code 3.

Why: Section 3.2 says the extractor needs no Markdown parser, so the reader does not track fences, and it says a valid revision returns no identifier twice, so a repeat is a defect of the document rather than a second statement.

## R-3: Presence of a member and its value are judged by different statements

Statements: CORE-JSON-3, CORE-JSON-4, CORE-JSON-5, CORE-&#66;ENCH-1, CORE-&#66;ENCH-2, CORE-&#66;ENCH-3, CORE-STATE-1, CORE-STATE-2, CORE-STATE-11.

Profile text: Section 5.1: "[CORE-&#66;ENCH-1] A workbench definition MUST carry a title." Section 5.7: "[CORE-JSON-3] The interchange object MUST carry the members `profile`, `title` and `columns`." Section 11, the row for CORE-JSON-3: "The written object carries `profile`, `title` and `columns`, and an object missing one of them is refused with `malformed`."

Readings considered: An object with no title member could fail both CORE-&#66;ENCH-1 and CORE-JSON-3, since each requires a title. Alternatively the statements of section 5.7 could judge whether a member is present, while the statements of sections 5.1 and 5.2 judge what a present member carries.

Chosen: The second reading. An absent member fails CORE-JSON-3 or CORE-JSON-5 and leaves the statement about its value not applicable. A present member whose value cannot serve fails the statement of section 5.1 or 5.2 that governs that value. A columns member that is not an array fails CORE-JSON-4, and an empty array fails CORE-&#66;ENCH-2.

Why: Section 5.7 states the requirements of the form in terms of members, sections 5.1 and 5.2 state requirements of the model, and the index row for CORE-JSON-3 speaks of an object missing a member. Both halves report the same refusal name, so the split changes no verdict, and it lets a fixture break one statement and nothing else.

## R-4: What counts as a title

Statements: CORE-&#66;ENCH-1, CORE-STATE-2.

Profile text: Section 4: "**Title.** The short prose name of a workbench, a column or a card, which a person reads and which may change without changing identity."

Readings considered: Any value of a title member could count as a title. Only a JSON string could count. Only a JSON string carrying at least one character that is not white space could count.

Chosen: A title is a JSON string carrying at least one character that is not white space. A number, null, an empty string and a string of spaces all fail.

Why: A title is a name a person reads, which a number, null or an empty string cannot be, and the example of section 5.7 writes every title as a string. The profile states no rule about white space, so the reader asks only that something other than white space be there to read.

## R-5: What counts as a column identifier, and how two are compared

Statements: CORE-STATE-1, CORE-TEXT-2.

Profile text: Section 4: "**Identifier.** The name by which a workbench, a column or a card is referred to, unique in its context, which never changes." Section 5.6: "[CORE-TEXT-2] A tool MUST NOT apply locale-dependent case rules to an identifier or a token."

Readings considered: An id could be any JSON value or only a string. Two identifiers that differ only in letter case could be the same identifier or two different ones.

Chosen: An id must be a non-empty JSON string, and two ids are the same only when their strings are identical code point for code point, so s1 and S1 are two identifiers.

Why: An identifier is a name, which the example of section 5.7 writes as a string. The profile states no case folding at all and CORE-TEXT-2 forbids the locale-dependent kind, so the reader applies none.

## R-6: How the profile member spells a version

Statements: CORE-&#66;ENCH-3, CORE-&#66;ENCH-5.

Profile text: Section 5.7, the example: `"profile": "dinah-core/0.17",` Section 2: "The string rides inside a conformance claim and inside the `profile` member of section 5.7." Section 2.1: "A conformance claim names `dinah-core 0.17` and says nothing about the channel, because the channel belongs to the document's history and the number belongs to the contract."

Readings considered: The member could be required to follow the example exactly, as the identity string, a solidus, the major number, a full stop and the minor number. It could also admit the spelling a conformance claim uses, with a space in place of the solidus. It could admit any text from which two numbers can be recovered.

Chosen: The reader accepts the identity string from R-1, then a solidus or a single space, then the major number, a full stop and the minor number, each written in ASCII digits and compared as an integer. Any other value fails CORE-&#66;ENCH-3, including a number, another identity string, a third number and an appended channel.

Why: No statement fixes the grammar of the member. The example and section 2.1 show the only two spellings the profile itself uses for a version of this profile, and the reader refuses only what a conforming tool is required to refuse, so it does not refuse a spelling the profile uses. Section 2 says the identity string rides inside the member, so a value naming another identity string declares no version of this profile.

## R-7: The version a tool's claim names, and definitions that target an earlier version

Statements: CORE-&#66;ENCH-5.

Profile text: Section 5.1: "[CORE-&#66;ENCH-5] A tool MUST refuse to act on a workbench definition declaring a profile version that sorts after the profile version the tool's own conformance claim names, ordering two profile versions by major number first and, where the major numbers are equal, by minor number, reporting the refusal name `unsupported-version`." Section 12, the 0.7 entry: "A workbench declaring a revision from `dinah-core 0.1` through `dinah-core 0.6` is therefore refused by name, and the refusal says which migration to run."

Readings considered: The version to compare against could be fixed inside the reader, or taken from the profile the reader is given. A definition declaring an earlier version could be refused, following the 0.7 entry, or accepted, following CORE-&#66;ENCH-5 alone.

Chosen: The reader compares against the version stated in the opening lines of the profile it is given, on the reading that a tool conforming to that revision names that version in its claim. It refuses only a later version, so a definition declaring dinah-core 0.3 is accepted.

Why: CORE-&#66;ENCH-5 refuses only a version that sorts after the claim. The 0.7 entry describes how one tool treats its own stored definitions after a renaming of its storage, which section 1 puts outside the profile, and a changelog entry is not a statement line, so it adds no refusal a conforming tool is required to make.

## R-8: Slugs in the interchange form

Statements: CORE-STATE-10, CORE-JSON-10.

Profile text: Section 5.2: "[CORE-STATE-10] Each column MUST carry a slug unique within its workbench." Section 5.2: "It is unique within the workbench for the same reason an identifier is: a name resolving to two columns resolves to neither." Section 5.7: "[CORE-JSON-10] A column object MAY carry the members `instructions`, `operator_owned`, `capacity`, `slug`, and `gate_items`." Section 12, the 0.3 entry: "A tool reading or writing the interchange form may now carry a `slug` member on a column object, and a tool that does not recognise `slug` and has not read this revision sees it travel through as an unrecognized member under CORE-JSON-7, unaffected."

Readings considered: Every column object could be required to carry a slug, which would refuse the example of section 5.7, whose columns carry none. Alternatively the slug could be a property of a column in the model that the interchange form may omit, with the reader judging only the slugs an object carries. Two equal slugs could be refused, or only reported, since the index row for CORE-STATE-10 does not say "refused".

Chosen: A column object without a slug is not refused. Two column objects carrying equal slugs fail CORE-STATE-10 and the object is refused as malformed. When no column carries a slug the statement is reported not applicable.

Why: CORE-JSON-10 makes slug a member a column object may carry, and the example of section 5.7, the profile's one exhibit of the form, carries none, so a reading that refuses the example cannot be the intended one. The profile does not say how a tool obtains a slug the form omits, and the reader does not guess. A repeated slug leaves a definition in which a name resolves to neither column, which section 5.2 gives as the reason the rule exists, and CORE-OUT-5 names malformed for such a definition.

## R-9: Kinds that carry a layer's prefix, and the spelling of kinds

Statements: CORE-STATE-11, CORE-STATE-12, CORE-OUT-3.

Profile text: Section 5.2: "Those are the three kinds this profile declares, and a tool may mint another under a layer's prefix, as section 9 describes." Section 11, the row for CORE-OUT-3: "Every refusal name reported is one section 6.1 declares or one containing a full stop." Section 3.4: "A token is machine vocabulary with a fixed spelling, stored and transmitted as written here, and never translated."

Readings considered: A kind could count as carrying a layer's prefix only when a layer of that name is declared in the same definition, or whenever it contains a full stop. A kind spelled Work could be the kind work or a fourth word.

Chosen: A kind is accepted when it is a JSON string equal to intake, work or done exactly, or a JSON string containing a full stop. The reader does not require the layer to be declared in the same object, and Work, WORK and any other spelling fail CORE-STATE-11.

Why: Section 9 says a layer's name contains a full stop and the profile's own names never do, the index states the parallel rule for refusal names as containing a full stop, and no statement requires a layer to be declared before a name is minted under it. Kinds are tokens, which have a fixed spelling.

## R-10: Capacity values

Statements: CORE-STATE-5, CORE-MOVE-4, CORE-OUT-5.

Profile text: Section 5.2: "[CORE-STATE-5] A column MAY declare a capacity limit, which is a whole number greater than zero."

Readings considered: As a permission, the statement could leave any value of capacity acceptable. As a permission that carries a definition, it could make a value that is not a whole number greater than zero something a tool must refuse. A whole number could be only a JSON number written without a fraction or an exponent, or any JSON number whose value is whole.

Chosen: A capacity member whose value is not a JSON number with a whole value greater than zero fails CORE-STATE-5, and the object is refused as malformed. The value 3.0 is accepted, and true, "3", 0, -1 and 2.5 fail.

Why: The statement defines what a capacity limit is, and a value outside the definition is not a limit CORE-MOVE-4 could compare a count against, so the definition carries something the profile requires of it and CORE-OUT-5 names malformed. RFC 8259 describes a number by its value, and 3.0 names the same whole number as 3.

## R-11: Members whose shape the profile does not fix

Statements: CORE-STATE-4, CORE-INSTR-1, CORE-JSON-10, CORE-JSON-11, CORE-JSON-12, CORE-JSON-13, CORE-FIELD-1, CORE-FIELD-2, CORE-FIELD-3, CORE-FIELD-4, CORE-FIELD-5, CORE-FIELD-7, CORE-FIELD-9, CORE-FIELD-10, CORE-CAP-1, CORE-CAP-2, CORE-GATE-1.

Profile text: Section 5.7: "[CORE-JSON-11] The interchange object MAY carry the members `fields` and `field_values`." Section 5.10: "A declaration carries three things: the key a reader types, the type its value takes, and one line of prose saying what the field means."

Readings considered: The reader could assume shapes, such as an array of declaration objects under fields and an object from key to value under field_values, and then judge keys, types and values against CORE-FIELD-2, CORE-FIELD-3 and CORE-FIELD-5. It could instead judge only whether these members are carried.

Chosen: The reader judges only whether operator_owned, instructions, gate_items, fields, field_values, require_fields and tiers are carried, and it compares slugs only for equality. The field and capability statements that could be judged only through a shape, CORE-FIELD-2, CORE-FIELD-3, CORE-FIELD-4, CORE-FIELD-5, CORE-FIELD-7, CORE-FIELD-9 and CORE-CAP-2, are classified none. The fixtures that carry these members use shapes of their own, and their sources say so.

Why: Section 5.7 names these members and gives no shape for any of them, and section 5.10 names the parts of a declaration without naming members for them. Judging a shape the profile never states would fill a silence the brief forbids filling. CORE-STATE-5 is the one permission whose statement fixes a value, and R-10 covers it.

## R-12: How permissions are reported

Statements: CORE-STATE-4, CORE-STATE-5, CORE-JSON-10, CORE-JSON-11, CORE-JSON-12, CORE-JSON-13, CORE-FIELD-1, CORE-FIELD-10, CORE-CAP-1, CORE-GATE-1, CORE-INSTR-1.

Profile text: Section 3.1: "The keyword vocabulary is the one RFC 2119 defines as amended by RFC 8174, and only the uppercase forms carry meaning:" Section 11, the row for CORE-STATE-4: "A definition marking a column operator-owned is accepted."

Readings considered: A permission could be reported as pass for every object, since nothing breaks it. It could be reported as pass where the object uses it and not applicable where it does not.

Chosen: A permission is reported as pass where the object uses it and as not applicable where it does not. Only CORE-STATE-5 can fail, under R-10. Across every statement, the verdict is refuse exactly when some statement fails.

Why: Under RFC 2119 a MAY is an option an object is free to take or leave, so an object that leaves it gives the reader nothing to judge, and the index states the outcome of each permission as the object that uses it being accepted.

## R-13: Designating an operator

Statements: CORE-OWNER-2, CORE-OWNER-3.

Profile text: Section 5.4: "How a workbench designates one is outside this profile, but that it does so is not, because a workbench with no operator has blocks nobody can lift and operator-owned columns nobody can leave." Section 5.4: "[CORE-OWNER-3] A tool MUST refuse to act on a workbench that designates no operator, reporting the refusal name `no-operator`." Section 11, the row for CORE-OWNER-3: "A verb asked on a workbench that designates no operator is refused with `no-operator`."

Readings considered: Because section 5.7 defines no member naming an operator, every interchange object could be read as a workbench designating no operator, which would refuse the example of section 5.7 with no-operator. Alternatively designation could be something outside the form, which one object cannot show.

Chosen: The reader never refuses an object on the ground of its operator, and CORE-OWNER-3 is classified none.

Why: Section 5.4 puts the means of designation outside the profile, and the index states the outcome of CORE-OWNER-3 for a verb, not for reading a definition. In section 10.1 one tool hands the Wedding definition to another in the interchange form and the second reads it.

## R-14: Layer declarations and unrecognized members

Statements: CORE-LAYER-1, CORE-LAYER-2, CORE-LAYER-3, CORE-JSON-7.

Profile text: Section 9: "A layer declares itself in the workbench definition under a dotted name, and the profile's own names never contain a dot, so a layer's name can never collide with a name a later revision of this profile introduces." Section 10.1: "The new tool reads it, keeps the two fields it does not recognize, and shows the same four columns in the same order."

Readings considered: A top-level member without a full stop that the profile does not define could be a layer declared under an undotted name, which CORE-LAYER-1 would refuse, or an unrecognized member, which CORE-JSON-7 requires a tool to keep. A top-level member with a full stop could be a layer declaration or merely an unrecognized member.

Chosen: A top-level member whose name contains a full stop is a layer declaration, and every other member the profile does not define is an unrecognized member and is accepted. Because the reader cannot tell an undotted layer declaration from an unrecognized member, and a dotted name can never reuse a name the profile defines, CORE-LAYER-1 and CORE-LAYER-3 are classified none. In pair, dotted top-level members are judged under CORE-LAYER-2 and every other member the profile does not define is judged under CORE-JSON-7.

Why: Section 10.1 and the 0.3 and 0.13 entries treat members a tool does not know as travelling through unaffected, section 5.10 says the top level of a definition is the namespace a layer declares itself in, and DOC-LAYER-1 keeps every name the profile defines free of a full stop.

## R-15: A byte order mark

Statements: CORE-JSON-2.

Profile text: Section 5.7: "[CORE-JSON-2] The interchange form MUST be one JSON object encoded in UTF-8."

Readings considered: A file beginning with the bytes EF BB BF could be refused, because its text then begins with U+FEFF, which the grammar of RFC 8259 does not admit. It could be accepted, because those bytes are valid UTF-8 and section 8.1 of RFC 8259 lets a parser ignore a byte order mark.

Chosen: The reader ignores one leading UTF-8 byte order mark, reports CORE-JSON-2 as pass, and says in the detail that the mark was there. A second mark is not ignored.

Why: The profile says nothing about a byte order mark, and RFC 8259 permits a parser to ignore one, so a conforming tool is not required to refuse such a file, and the reader refuses only what such a tool is required to refuse.

## R-16: What counts as a JSON text, and repeated member names

Statements: CORE-JSON-2.

Profile text: Section 5.7: "The words object, array, element, member, string and number in this subsection are JSON's own, with the meanings RFC 8259 gives them."

Readings considered: The reader could accept what common parsers accept, including NaN and Infinity, or hold to the grammar of RFC 8259. An object that repeats a member name could be refused, or judged by its first value, or judged by its last value.

Chosen: The reader holds to the grammar of RFC 8259, so NaN, Infinity, raw control characters inside strings and text after the object all fail CORE-JSON-2. An object that repeats a member name is accepted as JSON, the reader judges the last value, and the detail of CORE-JSON-2 names the repeated member.

Why: RFC 8259 has no NaN or Infinity in its grammar. It says the names within an object SHOULD be unique rather than MUST, so an object repeating one is still a JSON object and nothing in the profile requires a tool to refuse it. The profile is silent on which value a tool reads, and the reader takes the last, which RFC 8259 reports that many implementations do.

## R-17: The refusal name of a refused object

Statements: CORE-OUT-5, CORE-OUT-6, CORE-&#66;ENCH-5, CORE-JSON-2.

Profile text: Section 6.1: "[CORE-OUT-5] A tool MUST report the refusal name `malformed` when it refuses a workbench definition, a column, a card or an interchange object for want of something this profile requires it to carry and no more particular refusal name this profile declares applies." Section 6.1: "[CORE-OUT-6] An outcome of `refused` MUST carry the refusal name of the first unsatisfied check in the order section 6 declares, which places the two checks preceding every verb ahead of the preconditions the verb's own subsection states."

Readings considered: A file that is not UTF-8, or not JSON, could carry no refusal name, since CORE-OUT-5 speaks of a want of something the object must carry, or it could carry malformed. An object that fails CORE-&#66;ENCH-5 and also lacks a member could be named by either failure.

Chosen: Every refused object is reported as malformed, except that an object failing CORE-&#66;ENCH-5 is reported as unsupported-version whatever else it fails.

Why: CORE-JSON-2 requires the form to be one JSON object in UTF-8, so a file that is not is refused for want of what the profile requires, and no more particular name fits. Section 6.1 places the version check first among the checks ahead of every list, and no other name that reading a definition can produce comes before it.

## R-18: Cards, links, items and acts are not in the interchange form

Statements: CORE-CARD-1 to CORE-CARD-10, CORE-LINK-1 to CORE-LINK-6, CORE-ITEM-1 to CORE-ITEM-4, CORE-ACTING-1 to CORE-ACTING-4, CORE-CAP-3.

Profile text: Section 12, the 0.14 entry: "The interchange form of section 5.7 carries columns and no cards, so no field of a card travels in it, and the parenthetical is wrong about the interchange form."

Readings considered: The statements about cards could be judged over a cards member, since the 0.2 entry once said the interchange form carries a card's number field. They could be classified none, since the form defines no member for cards.

Chosen: Every statement about cards, links, structured items and recorded acts is classified none, and a cards member in an object is treated as an unrecognized member like any other.

Why: Section 5.7 names no member for cards, and the 0.14 entry corrects the 0.2 entry and says the form carries columns and no cards.

## R-19: Standing instructions have no member

Statements: CORE-INSTR-2, CORE-INSTR-6.

Profile text: Section 10.1: "The workbench's standing instructions say that nothing is agreed until both of them have said so." Section 5.7: "[CORE-JSON-11] The interchange object MAY carry the members `fields` and `field_values`."

Readings considered: The standing instructions could be expected under some member of the interchange object, such as instructions at the top level. Alternatively the form could have no place for them.

Chosen: CORE-INSTR-2 and CORE-INSTR-6 are classified none. The fixture built from section 10.1 carries the standing instructions under a top-level member named instructions, which the reader treats as one of the two members the new tool does not recognize, and the Wedding operators as the other.

Why: No statement of section 5.7 names a top-level member for standing instructions, and the walkthrough says the receiving tool keeps two fields it does not recognize without saying which.

## R-20: What reading and writing back has to keep

Statements: CORE-JSON-1.

Profile text: Section 5.7: "[CORE-JSON-1] A tool MUST be able to read and to write the interchange form of a workbench definition." Section 11, the row for CORE-JSON-1: "A workbench definition written in the interchange form reads back with the same title, columns and order."

Readings considered: The comparison could cover only the title and each column's id, title and kind. It could cover every member the profile defines. It could include the profile member.

Chosen: The reader compares the title, fields, field_values and tiers where the sent object carries them, the number and order of columns, and every member the profile defines on each column at the same position, using JSON equality under which 3 and 3.0 are equal and true is not 1. It does not compare the profile member, and it allows the returned form to carry members the sent object did not.

Why: A definition written back without a column's capacity or instructions is not the definition that was read. The profile member declares the version the writing tool targets, which section 10.1 describes the writing tool as naming, and a tool may add a slug that CORE-STATE-10 requires of a column the sent object gave none.

## R-21: Which members count as unrecognized, and how columns are matched

Statements: CORE-JSON-7, CORE-LAYER-2.

Profile text: Section 5.7: "[CORE-JSON-7] A tool MUST preserve the members it does not recognize in an interchange object it has read and written back."

Readings considered: The members a tool does not recognize could be only those the particular tool does not know, which the reader cannot see, or every member the profile does not define. A column's unrecognized members could be looked for at the same position in the returned form or on the column with the same identifier.

Chosen: The reader judges every member the profile does not define at that level, the top level or a column object, as unrecognized, apart from dotted top-level names, which are layers under R-14. It finds a column's members on the returned column with the same id, and at the same position only when the sent column has no string id.

Why: The reader cannot know what a given tool recognizes, and a tool that knows only this profile recognizes exactly the members the profile defines. Matching by identifier keeps a reordering, which CORE-JSON-1 reports, from being charged a second time as a loss of members.

## R-22: Encoding and tokens judged on what a tool writes

Statements: CORE-TEXT-1, CORE-TEXT-3, CORE-JSON-2.

Profile text: Section 11, the row for CORE-TEXT-1: "Text the tool writes decodes as UTF-8." Section 11, the row for CORE-TEXT-3: "A machine-readable response carries the canonical token whatever language is asked for."

Readings considered: CORE-TEXT-1 could be judged on each offered object, where it would coincide with CORE-JSON-2, or on the form a tool writes. CORE-TEXT-3 could treat any missing member name as a translation, or only one that appears to have been replaced.

Chosen: Both are classified pair. CORE-TEXT-1 fails when the returned bytes are not UTF-8. CORE-TEXT-3 fails when a column's kind comes back spelled differently, or when a member name the profile defines is gone from an object while a name the sent object never carried has appeared in the same object. A member that is simply dropped is left to CORE-JSON-1.

Why: The index states both outcomes in terms of what a tool writes. A dropped member is a loss rather than a translation, while a defined name replaced by an unfamiliar one is what a translated token looks like on a machine-readable surface.

## R-23: A pair in which a file cannot be read

Statements: CORE-JSON-1, CORE-JSON-7, CORE-LAYER-2, CORE-TEXT-3.

Profile text: Section 5.7: "[CORE-JSON-7] A tool MUST preserve the members it does not recognize in an interchange object it has read and written back."

Readings considered: When the returned file is not one JSON object, the preservation statements could be reported not applicable or as failures. When the sent file is not one JSON object, the comparisons could be reported as failures or not applicable.

Chosen: When the returned file is not one JSON object, CORE-JSON-1 fails, CORE-JSON-7 and CORE-LAYER-2 fail if the sent object carried something to preserve and are not applicable otherwise, and CORE-TEXT-3 is not applicable. When the sent file is not one JSON object, every comparison is not applicable, and CORE-TEXT-1 still judges the returned bytes.

Why: The profile is silent here. A member that does not come back has not been preserved, whatever the reason, while a translation can only be seen in a form that can be read, and nothing can be owed back from a sent file that was never an object.

## R-24: A named file that cannot be opened

Statements: No statement governs this choice, which concerns the reader's command line.

Profile text: Section 1: "It says nothing about files, directories, databases, wire protocols, transports, user interfaces, or the words a screen shows a reader in any particular language."

Readings considered: A path that cannot be opened could yield a result with a refuse verdict, or it could be treated as a command line the reader did not understand.

Chosen: The reader writes a diagnostic to standard error, writes nothing to standard output, and exits with code 2.

Why: The profile is silent about files, and a path with no bytes behind it offers no interchange object to judge, so any verdict would be a judgement about the command line rather than about an object. Bytes that can be read but are not UTF-8 or JSON are still judged, under R-17.
