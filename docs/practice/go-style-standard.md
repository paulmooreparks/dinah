# Go style standard

Dinah's Go source has to be readable by a contributor who did not write it and cannot run it in their head. Nearly all of it is produced by agents, and agent-written Go fails in a characteristic way. The code comes out correct and gofmt-clean while packing three decisions onto a line no reviewer can take apart.

This standard has a mechanical floor that a tool settles and a short list of judged rules that a reviewer applies by reading. Implement writes to it. Agent Code Review holds diffs against it and cites it by rule.

## The mechanical floor

The floor is the part no one has to judge.

```
gofmt -l .
go build ./...
go vet ./...
```

All three run in CI on every pull request and on every push to the trunk, and Implement runs them before handing a card off. Build, vet and test run on Linux and on Windows. The gofmt check runs once, in a job of its own, because formatting is a property of the source rather than of a platform.

`gofmt -l .` prints the path of every file whose formatting differs from gofmt's own output. A file appearing in that list does not by itself make the command fail, so a check built on it reads the output rather than the status. A file gofmt cannot parse is reported as an error instead of being listed, and the CI job fails on either.

gofmt formats and does not judge. It will not split a dense line, invent a name, break an expression apart, or notice that a function is doing four things. Everything below exists because a gofmt-clean file can still be unreadable.

## The judged rules

Each rule names what it is aimed at, and where the aim is easy to overshoot, what it is not aimed at.

### 1. One statement per line, with the intermediates named

Bind an intermediate result to a named local when an expression nests three calls deep, or when a single line both computes a value and decides something with it. The name carries the meaning of the inner expression, so a reader does not have to evaluate it to find out what it was for.

Wrong:

```go
if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(whitespace.ReplaceAllString(strings.TrimSpace(term), " ")) + `\b`).MatchString(flat) {
	hits = append(hits, term)
}
```

Right, and this is the form in `Excluded` in `internal/profile/extract.go`:

```go
want := whitespace.ReplaceAllString(strings.TrimSpace(term), " ")
re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(want) + `\b`)
if re.MatchString(flat) {
	hits = append(hits, term)
}
```

The two forms do the same thing. The second one says what the pieces are, and gofmt will not turn the first into the second.

Not aimed at a parallel assignment of two related values, which is one statement: `id, body := m[1], m[3]` in the same file stays as it is. Not aimed at the `if err := f(); err != nil` form, which Go's own conventions prefer. Not aimed at a comparison chain that checks several fields of one value in a single test assertion, which reads as one question however many clauses it has.

### 2. A long signature or call breaks one argument per line

Break when the line runs past roughly 100 columns, or when three or more of its arguments are themselves expressions rather than plain names. Put one argument on each line and leave the trailing comma, which keeps the next diff to one line. Composite literals follow the same rule, and gofmt will not do this for you: it preserves whatever line breaks the author wrote inside a literal.

Wrong:

```go
rows = append(rows, BoundaryRow{Item: strings.TrimSpace(m[1]), Ruling: m[2], Reason: strings.TrimSpace(m[3]), Reopen: strings.TrimSpace(m[4]), Line: i + 1})
```

Right, and this is the form already in `internal/profile/extract.go`:

```go
row := BoundaryRow{
	Item:   strings.TrimSpace(m[1]),
	Ruling: m[2],
	Reason: strings.TrimSpace(m[3]),
	Reopen: strings.TrimSpace(m[4]),
	Line:   i + 1,
}
rows = append(rows, row)
```

No hard column limit applies here, and no tool enforces one. Treat 100 as the point where a reviewer starts looking at a line.

Not aimed at a long line with no argument list worth breaking. A string or regular-expression literal stays whole, since breaking it either changes it or hides it, which is why the `identification` constant in `cmd/dinah/main.go` and the pattern literal in `TestIndexMatchesTheExtraction` both run well past 100 columns. A call whose arguments are each a name, a literal, a field or element of one value, or the length of one of those, is already as clear as it will get however wide it runs, and that covers the `t.Fatalf` and `t.Errorf` reporting lines in `TestIndexMatchesTheExtraction` and the `fmt.Sprintf` inside a `Defect` literal that is itself already broken per field. An `if` with an initialiser calling one function is one decision, as it is under rule 1. And a comparison chain over several fields of one value is one question, also as under rule 1.

### 3. Guard clauses, and the happy path at the left margin

Handle the refusal, the error and the empty case first, then return or continue. The path that does the work then runs at the function's minimum indentation for its whole length. Three levels of nesting inside one function is the signal to invert a condition or lift a block into a function of its own. The scanning loop in `Extract` is the shape: it recognises a fence and continues, fails to match a statement line and continues, and reaches the real work with nothing left holding it open.

### 4. Every exported identifier carries a doc comment

Write a doc comment on each exported type, function, constant, variable and method, beginning with the identifier's own name, and one on each package. Document a struct field whenever its name alone does not settle what goes in it. Say what the thing is for and what a caller has to know, rather than restating the signature in words.

Test functions are exported functions and carry a comment too, saying what the test asserts rather than how it works. `internal/profile/extract_test.go` does this for every test in the file.

Unexported identifiers need a comment only where a reader would otherwise have to reverse-engineer the intent. In practice that means regular expressions, magic constants, and any function whose name understates what it does.

`internal/profile/extract.go` is where to see the level being asked for, and the `Statement` type there is the shape to copy, with a comment on the type and a comment on each field.

### 5. Errors read like errors, and refusals are named from the contract

Error strings are lowercase and carry no trailing punctuation, because Go composes them into longer sentences. Wrap with `%w` whenever a caller might reasonably test for the cause, and use `%v` where the cause is being reported rather than passed on.

A refusal the coordination contract already names is spelled the way the contract spells it, so `at-capacity` and `not-operator` and their siblings travel unchanged into the code. A name invented at the call site is a second vocabulary for the same refusal, and the conformance suite reads the contract's.

### 6. The standard library first

The module has no external dependencies and no `go.sum`. A card may add one, and the card records it: name the dependency in WHAT SHIPPED, say which standard-library route you considered, and say what made it insufficient. Tests use `testing` and nothing else, so no assertion or mocking library enters the tree without that same justification.

### 7. A path comparison in a test reproduces the mechanism, not a platform correction

When a test asserts that a path the head produced equals some expected value, and the head derived that value through `os.Getwd`, `filepath.Abs`, or any other call that consults the working directory, build the expected side by calling the same sequence in the test rather than joining a string onto a raw fixture value. `t.TempDir()`'s own return value and a path resolved through the process's working directory name the same directory on some platforms and different spellings of it on others: a macOS temporary directory sits behind a symlink, and a Windows CI runner can hand out an already-short 8.3 form. A comparison built by joining onto the raw fixture string is really asserting how a platform spells paths, not what the code under test does.

Wrong:

```go
target := filepath.Join(dir, "elsewhere")
os.MkdirAll(target, 0o755)
...
if got != target { t.Errorf(...) }
```

Right:

```go
target := filepath.Join(dir, "elsewhere")
os.MkdirAll(target, 0o755)
wantTarget := resolvedDir(t, target) // chdir into target, then os.Getwd: the same sequence the head runs
...
if got != wantTarget { t.Errorf(...) }
```

Not aimed at a path a test only ever hands the head as an opaque argument and never compares as a string; nothing there derives a second spelling of it. Not aimed at reaching for a general-purpose normalization instead, such as `filepath.EvalSymlinks` or a case fold, to paper over a mismatch discovered on one platform: a correction that resolves a macOS symlink can just as easily expand a Windows short name back to its long form, breaking the platform that passed a moment ago. Reproducing the head's own mechanism agrees with the head on every platform without asserting anything about which platform's quirk is in play; a general-purpose normalization is a guess about which quirk explains the failure in front of you.

## Reuse before you write

The rule has two sides because one side alone has never held. An implementer asked to check for prior art will report that they checked, and a reviewer given no record has nothing to check against.

**At Implement.** Before writing a function, search for one that already does the job. On a tree this size the search is exhaustive rather than a sample, since `grep -rn '^func ' cmd internal` lists every function in the codebase in about one screen. Then say in WHAT SHIPPED which existing helper you reused, or state that you searched and none existed. A WHAT SHIPPED silent on reuse is an incomplete handoff.

**At Agent Code Review.** A new function duplicating one the codebase already has is a [major]. So is a second solution to a problem the codebase already solves a particular way, even where no function is literally repeated: this codebase reads documents line by line rather than parsing Markdown, and a diff that introduces a Markdown parser either follows the house approach or says in WHAT SHIPPED why the house approach fails for this case.

**The condition that retires the grep.** That instruction holds only while one grep still shows the whole surface, so the condition is written down here rather than left to be noticed later. When the tree outgrows it, meaning a contributor can no longer take in the exported surface from one screen of output, a codebase-map document supersedes this paragraph and becomes the thing an implementer searches.

## What this standard does not govern

The standard leaves the surfaces below alone. Prose in README and docs answers to the workbench document "Prose standard" instead. Doc comments are code, so they answer to this standard rather than to that one.

- Generated files, vendored code, and the Python hooks under `scripts/hooks`.
- README and docs prose.
- Naming beyond Go's own conventions, including short receiver names, short loop variables and `err`.
- Package layout beyond the existing `cmd/` and `internal/` split.
- Line length as a hard limit.

## How heavily this is enforced

A codebase maintained by agents absorbs a rule a tool can check and pays badly for a rule that invites argument, so the weights are stated here rather than left to the reviewer. A file gofmt would reformat is a [major], and so is a reuse duplication. Judged rules 1 to 5 and 7 are [minor] findings: the reviewer records them in the findings comment, and the next diff on the card fixes them. They do not on their own send a card back.

A card can finish on a diff that reads clean, so a [minor] can outlive the card that raised it. One of those is dropped when the card closes. A reviewer who thinks the class deserves better raises a follow-on card for it, and a class caught twice earns an entry in "Convention counterexamples" under that document's own promotion rule.

A reviewer may cite this document only for a rule it states. "This is not idiomatic Go", with no rule behind it, is not a finding under this standard, and the way to raise it is to propose the rule here.