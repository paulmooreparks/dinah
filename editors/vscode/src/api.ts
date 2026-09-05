// The object activate() returns, and the types it is built from.
//
// A status bar item cannot be read back through the VS Code API, and neither
// can a welcome view, so an integration test that wanted to assert on either
// would have to assert on a screenshot. Returning this object instead is what
// lets those tests assert on state: they reach it as
// `vscode.extensions.getExtension(EXTENSION_ID).exports`.

/** What `dinah --json version` reports about a binary. */
export interface VersionReport {
	/** The binary's own release number. Displayed, never compared. */
	readonly tool: string;
	/** The conformance claim, `<name>/<major>.<minor>`. */
	readonly profile: string;
	/** The storage format version the binary implements. */
	readonly format: number;
}

/** Which rung of the ladder produced the binary this window is using. */
export type BinarySource = "setting" | "path";

/** One workbench a `dinah.ambiguous-workbench` refusal found. */
export interface Candidate {
	readonly title: string;
	/** Absent on a workbench that declares none. */
	readonly slug?: string;
	/** The workbench directory, which is what `--workbench` takes. */
	readonly path: string;
	/**
	 * The refusal a reader could not get past on its way to this workbench,
	 * which says the workbench itself would not read.
	 *
	 * Absent on every candidate an ambiguous-workbench refusal produces, since
	 * a tie is between workbenches that all read. It arrives on a candidate a
	 * downward walk describes and cannot open. It never carries a refusal
	 * raised by a question asked of a workbench that opened; that is a
	 * different fact and rides its own field on the root-scoped answers.
	 */
	readonly refused?: string;
}

/**
 * Every way the binary question can come out. The three skew arms are kept
 * apart from `unusable` because they mean different things to a reader: a
 * skewed binary is a real dinah that this extension will not drive, and an
 * unusable one may not be dinah at all.
 */
export type BinaryState =
	| {
			readonly state: "ok";
			readonly path: string;
			readonly source: BinarySource;
			readonly version: VersionReport;
	  }
	| { readonly state: "no-binary" }
	| {
			readonly state:
				| "unusable"
				| "format-skew"
				| "profile-skew"
				| "binary-too-old";
			readonly path?: string;
			readonly detail: string;
			readonly version?: VersionReport;
	  };

/** Every way the workbench question can come out for one workspace folder. */
export type WorkbenchResolution =
	| {
			readonly state: "ok";
			/** The resolved workbench directory, absolute. */
			readonly root: string;
			readonly title: string;
			/** The rung that resolved it: flag, environment, search or config. */
			readonly source: string;
			readonly profile: string;
			/**
			 * Whether `root` lies inside the workspace folder this resolution
			 * belongs to. False is the dinah-241 case: the walk climbs to the
			 * drive root, so a folder several levels below an unrelated
			 * workbench resolves to that workbench.
			 */
			readonly insideWorkspace: boolean;
	  }
	| {
			readonly state: "refused";
			readonly refusal: string;
			readonly detail?: string;
			readonly candidates?: readonly Candidate[];
	  };

/**
 * The sidebar's data source, as much of it as a test needs to reach.
 *
 * A TreeView's contents cannot be read back through the VS Code API any more
 * than a status bar item's can, so the provider itself rides the returned
 * object and an integration test walks it directly.
 */
export interface TreeProviderHandle {
	readonly getChildren: (element?: unknown) => Promise<unknown[]>;
	readonly getTreeItem: (element: unknown) => unknown;
}

/** What `activate()` returns. */
export interface DinahApi {
	readonly binary: BinaryState;
	/** One entry per workspace folder, keyed by the folder's fsPath. */
	readonly workbenches: ReadonlyMap<string, WorkbenchResolution>;
	/** The sidebar's provider, exposed for the same reason as the two above. */
	readonly tree: TreeProviderHandle;
	/** The status bar item's tooltip, exposed because the item cannot be read back. */
	readonly statusTooltip: string;
	/** The status bar item's text, exposed for the same reason. */
	readonly statusText: string;
	/** The dinah release this build of the extension was cut alongside. */
	readonly pairedRelease: string;
}
