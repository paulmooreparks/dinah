# Full Go CI diagnostics

The full Go CI job must retain evidence when a test process exits without a named assertion in its ordinary output. This change improves diagnosis. It does not explain or correct the desktop exit or the terminal readiness failure observed on 5c84503.

## Directory handoff and capture

The prepare step creates `full-go-diagnostics` beneath `RUNNER_TEMP` and writes its absolute path to `GITHUB_ENV` as `DINAH_TEST_DIAGNOSTICS_DIR`. Later steps receive that variable through GitHub's documented environment-file mechanism. The directory is created before any tests run, and setup errors fail the step visibly. A job-level environment expression cannot use the `runner` context.

Directory creation uses `Directory.CreateDirectory`, whose documented exceptions include an existing file at the requested path. The setup regression checks that case before testing the successful handoff.

The full-suite step invokes the existing FIFO Go gate with `./... -count=1 -json`. It merges standard error into the recorded output and keeps `full-go-output.log` in the diagnostics directory while also displaying the stream in the job log. The shell returns the gate's original exit code. There are no test filters, deadline changes or exceptions.

Desktop crash capture uses the same directory, with a separate file for each process. Its setup must run before standard handles are released. The crash regression child uses its own directory. An external process termination may produce no runtime crash output, so the per-test JSON stream is separate evidence.

The upload step uses `always()` and uploads the whole diagnostics directory, including any crash files left by subprocesses. A missing artifact is an error. Uploading diagnostics does not turn a failed test step into a successful job, and publication still depends on that job.

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
- [Pinned actionlint release](https://github.com/rhysd/actionlint/releases/tag/v1.7.12)
