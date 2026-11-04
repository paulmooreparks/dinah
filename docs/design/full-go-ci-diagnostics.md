# Full Go CI diagnostics

The full Go CI job must retain evidence when a test process exits without a named assertion in its ordinary output. This change improves diagnosis. It does not explain or correct the desktop exit or the terminal readiness failure observed on 5c84503.

## Directory handoff and capture

The prepare step creates `full-go-diagnostics` beneath `RUNNER_TEMP` and writes its absolute path to `GITHUB_ENV` as `DINAH_TEST_DIAGNOSTICS_DIR`. Later steps receive that variable through GitHub's documented environment-file mechanism. The directory is created before any tests run, and setup errors fail the step visibly. A job-level environment expression cannot use the `runner` context.

Directory creation uses `Directory.CreateDirectory`, whose documented exceptions include an existing file at the requested path. The setup regression checks that case before testing the successful handoff.

The full-suite step invokes the existing FIFO Go gate with `./... -count=1 -json`. It merges standard error into the recorded output and keeps `full-go-output.log` in the diagnostics directory while also displaying the stream in the job log. The shell returns the gate's original exit code. There are no test filters, deadline changes or exceptions.

Desktop crash capture uses the same directory, with a separate file for each process. Its setup must run before standard handles are released. The crash regression child uses its own directory. An external process termination may produce no runtime crash output, so the per-test JSON stream is separate evidence.

Post-test extraction and staging run under `always()`. Upload requires successful extraction/cleanup and staging and uses only the resulting isolated snapshot. A missing artifact is an error. Uploading diagnostics does not turn a failed test step into a successful job, and publication still depends on that job.

Cleanup failure, unsafe content or staging failure intentionally fails the diagnostic job and prevents upload, even if the Go tests passed. Missing CDB or symbols is a capture limitation rather than a diagnostic failure; safe WCT evidence is retained. The job summary reports the original Go test outcome separately from extraction/cleanup and staging outcomes. The test step still returns its original exit status. Staging remains fail-closed for every unapproved input; a skipped-file manifest is deferred.

## Ordinary-handle Node preflight

Before the full suite, a separate workflow step runs `tools/nodepreflight` with ordinary redirected handles. It resolves Node from the workflow process's PATH and probes `--version` with a five-second context deadline and a one-second `WaitDelay` for process/output cleanup. The JSON records the resolved executable, arguments, elapsed time, exit code, context result and cancellation outcome. This is evidence about the preflight process, not the image selected inside PowerShell or evidence of Ctrl+C delivery.

The report lists `NODE_*` variable names without their values. It retains at most 4 KiB of output in memory while draining the streams, and publishes only a canonical numeric version line. All other output is omitted because startup errors can include option values, private paths or credentials. Counts and omission/truncation flags distinguish empty output from redacted or excessive output. The preflight's Node exit or timeout is recorded without changing the subsequent Go test command or its decision. Failure to retain the report fails visibly.

The guard uses disposable child processes to check success, nonzero exit, mixed/private output, excessive output, deadline expiry, cancellation and executable lookup failure. The full-Go artifact contains `node-preflight.json` alongside the test stream and any crash files. Neither a missing journal entry nor a version-query exit code alone proves where Node stopped.

## Native scratch and public artifact boundary

`DINAH_TEST_NATIVE_SCRATCH_DIR` names a separate absolute directory outside the diagnostic text directory. Runtime dumps stay there. The extractor checks the directory boundary through the existing documented final-path resolver, including Windows aliases, before opening scratch. It attempts to delete every discovered dump on success, missing debugger, timeout and extraction failure. Cleanup failure is explicit and fails the extraction step.

The fixture has synthetic JavaScript arguments, but it inherits the CI environment unchanged. The workflow permits repository reads and checkout credentials exist on the runner; the failed run did not establish the child's complete environment or prove it secret-free. This change does not make that claim. Raw dumps can contain stack memory, environment fragments or inputs and are never approved for public artifacts.

The runner locates CDB through PATH or the documented Windows SDK debugger location and probes `-version` under a five-second bound. Actual runner availability has not yet been observed for this change. Each dump extraction has a ten-second limit within a shared thirty-second budget. CDB runs from a fresh private working directory using `-sins`, an explicit Microsoft public symbol path and `~*kcnL 40;q`. The command requests clean numbered frames, hides source lines and limits each thread to hexadecimal 40 frames. It does not request parameters or memory.

The extractor retains only validated thread indices, frame numbers, approved system/Node module names, function names and offsets. Banners, addresses, source paths, arguments, unknown modules, oversized lines and unfiltered debugger output are discarded. Microsoft symbols are configured explicitly. A reviewed Node PDB retrieval route is not established, so the report states that limitation rather than inventing a symbol server or claiming complete native symbols. Missing debugger, missing usable frames and deadline expiry are safe limitation results; WCT text remains independent evidence. No raw dump is uploaded to compensate.

The upload tool opens inputs through `os.OpenRoot`, rejects links, non-text extensions, dump signatures, NUL/binary content and oversized inputs, then copies approved text into a fresh directory whose name the test children did not inherit. It rechecks that actual snapshot. `.dmp` is explicitly excluded by the upload action as well; the guard also rejects case variants and dumps renamed as logs. A late write to the original diagnostics directory cannot add a file to the copied snapshot. Limits are 32 MiB per file, 128 MiB total and 4,096 files; exceeding them fails visibly.

Focused guards cover raw/renamed dumps, the final uploaded set, snapshot isolation, filtered frame fields, missing debugger, bounded extraction and raw-dump cleanup after failure. Local CDB availability was not found, so the extraction tests use disposable stand-in processes. They do not claim a real dump was symbolicated or that the runner has CDB. Those limitations must accompany any proposed diagnostic push.

## Workflow validation

The mandatory workflow syntax guard runs actionlint v1.7.12 through its versioned Go module path. That version's runner table includes `macos-15-intel`, so no runner-label override or expression-error suppression is needed. Optional external shell and Python linters are disabled; actionlint's workflow syntax and expression checks remain enabled.

The guard checks the repository workflow and an invalid fixture whose job-level environment uses `runner.temp`. The fixture must fail for that context error. The capture guard runs the real prepare-step handoff and the full-suite shell command with a stand-in gate returning either zero or 17. It checks both output streams, directory setup failure and the upload policy. These fixtures do not run the product tests or claim that either original failure is repaired.

## Documented contracts

- [GitHub context availability](https://docs.github.com/en/actions/reference/workflows-and-actions/contexts#context-availability)
- [GitHub environment files](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands#setting-an-environment-variable)
- [GitHub status checks](https://docs.github.com/en/actions/reference/workflows-and-actions/expressions#always)
- [Artifact upload behavior](https://github.com/actions/upload-artifact)
- [PowerShell exit status](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_automatic_variables#lastexitcode)
- [Directory creation](https://learn.microsoft.com/en-us/dotnet/api/system.io.directory.createdirectory)
- [Go crash output](https://pkg.go.dev/runtime/debug#SetCrashOutput)
- [Go subprocess cancellation and output bounds](https://pkg.go.dev/os/exec#Cmd)
- [Go traversal-resistant filesystem roots](https://pkg.go.dev/os#OpenRoot)
- [CDB command-line options](https://learn.microsoft.com/en-us/windows-hardware/drivers/debugger/cdb-command-line-options)
- [Clean numbered stack frames and source-line suppression](https://learn.microsoft.com/en-us/windows-hardware/drivers/debuggercmds/k--kb--kc--kd--kp--kp--kv--display-stack-backtrace-)
- [Pinned actionlint release](https://github.com/rhysd/actionlint/releases/tag/v1.7.12)
