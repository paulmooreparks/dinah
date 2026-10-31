package main

import (
	"fmt"
	"strings"

	"studio/internal/bridge"
)

// pageMethodClass says what a call of an App method from the page means for
// the exit guard, and whether the page is meant to call it at all.
type pageMethodClass int

const (
	// pageQuery is a query, a presentation change or the close handshake
	// itself, which exiting may interrupt.
	pageQuery pageMethodClass = iota
	// pageWork changes something, so a call still running counts as
	// unfinished work when the window is asked to close.
	pageWork
	// notForPage records a callable method that the page must not access.
	notForPage
)

// pageMethods is every App method the bridge offers the page, with its
// class. TestEveryPageMethodIsListed fails when an App method is added
// without a row here or a row names a method the App no longer has, so
// that nothing reaches the page, or is counted as unfinished work, by
// accident. A method that only reads or reports, or that is the way out of
// the program (RestartToUpdate closes the window from inside the call), is a
// query: counted while it ran, it would make a restart or a quit warn about
// the call that asked for it, or a chat whose history is still being read look
// like unfinished work. ExtensionCall is a query because the extension manager counts
// the calls that change something itself (Executing) and leaves out reads.
var pageMethods = func() map[string]pageMethodClass {
	result := map[string]pageMethodClass{}
	list := func(class pageMethodClass, names string) {
		for _, name := range strings.Fields(names) {
			result[name] = class
		}
	}
	list(pageQuery, `
 AckUIStateFlush AnswerExitConfirmation BeginExitWork EndExitWork ExitStatus
 NativeWindowState NativeWindowAction State Sessions SessionsAt History SavedConversation
 AgentProviders ProviderAccounts ProviderModelPreferences ProviderUsage CanSteer
 AppletArtifactAt AppletAttention AppletCatalog AppletGrants AppletInvocationAt AppletInvocationsAt
 AppletRequests AppletResults BusOverview BusGroup BusResolveGroup BusGroupMessages BusThread
 ChatLogPath CheckSkillDraft ClipboardRead CompareDraft DocumentRefAt DraftStamp
 ExtensionWorkspace Extensions GetDraft HomeMountPath ListDrafts ListWorkspaceMounts
 ListWorkspaceMountsExpected ListWorkspaceResources PageBlocksOf PageDocumentMarkdown
 PageDocumentOf PageSourceOf PreviewDraft PreviewExtension PreviewSessionContinuation PreviewSessionWorkspaceMove
 PreviewSessionRecovery PreviewWorkspaceResource ReadAppearance ReadFile ReadSkill ReadUIState
 ReadWorkspaceMount ReadWorkspaceMountExpected ReadWorkspaceResource RecentDocuments
 ReadLinkedImage RenderD2 ReportWindowFocus ResolveDocumentLink ResolveDocumentLinks
 ResolveSessionContinuation ResolveWorkspaceResourceMount SessionContinuation SessionOptions
 SessionTimes SkillCatalog SkillFields Skills SkillsAt SkillsForProviderAt StatWorkspaceMount
 SubagentUpdates StatWorkspaceMountExpected Templates TerminalProfiles TerminalResize
 TerminalShells UpdateState UploadConflicts UploadNameConflicts WatchFolders
 WorkspaceLocalPath WorkspaceSkills Log SaveUIState ExtensionCall
 SuggestDocumentName JudgementCalibration JudgementState JudgementSummary JudgementRecords JudgementKinds JudgementSettings JudgementModels JudgementProviders Team ChatLead SetTeamLead ClearTeamLead MakeGroupATeam SetTeamBudget
 PendingPermissions PendingExitConfirmation PendingPageAsks PendingExtensionReviews ProviderStates
 ChatEvents ChatStates
 ChatHistory ChatImageInput ChatWindows HostLocale ModelPrices Notifications
 InstalledPackages OldAttachments PlanPackages PlanPersonalPackages PlanRemovePackages PlanWorkspaceImport RestartToUpdate`)
	list(pageWork, `
 LogQueueDefault DismissJudgementWarning DismissJudgementWarnings SetJudgementSettings RunJudgementCalibration CorrectJudgement ResolveHold ForgetContextChoice AcceptSuggestion AddDraftFile AddTerminalProfile AgentCheck AgentConnect AgentRestart
 AgentSignIn AgentStop Answer AnswerAppletRequest AnswerExtensionCommand AnswerPage
 AttachLocalWorkspaceSource BusAddToGroup BusDeleteGroup BusMarkGroupRead BusRemoveFromGroup
 BusGroupBudget BusRenameGroup BusReplyInThread BusSend BusSendToGroup BusStartGroup BusTrust BusUntrust Cancel
 CancelAppletInvocationAt CancelExtensionCall CancelRestart CancelSubagent 
 ChooseAgentExecutable ChooseComputerSavePath ChooseDocumentFile
 ChooseExtensionSource ChooseImportFolder ChoosePackageBundle ChooseUploadFiles
 ChooseWorkspace ChooseWorkspaceImport ChooseWorkspaceSource ClearHistory ClearNotifications
 Connect ContinueSessionWithProvider ContinueSessionInWorkspace DismissSwitchFailure CopySkillToMine CopyWorkspaceMount CreateWorkspace
 CreateNamedChat CreateWorkspaceFolder CreateWorkspaceFolders DeleteSession DeleteTemplate
 DetachWorkspaceSource DiscardDraft DismissAppletResultAt DismissNotification DismissSteering
 EnableExtension ExportThemeFile ExportWorkspace FindPackageUpdates ForgetAppletGrant
 ForgetWorkspace HideSession ImportThemeFile ImportWorkspace InstallClaudeAdapter
 InstallExtension InstallPackages InstallPersonalPackages InvokeAppletAt
 LoadFullHistory MakeWorkspaceDirectory MarkBusInboxRead MergeIntoDraft MoveWorkspaceMount
 MoveWorkspacePin NewEmptyDraft NewSession NewSessionAt NewSessionWithProviderAt 
 OpenComputerDocument OpenComputerFile OpenDraft OpenPath OpenProviderUsage
 OpenSession OpenSessionAt OpenSessionWith OpenSkillDraft OpenUsage OpenWorkspaceMount
 OpenWorkspaceResource PickAttachments PickDraftFiles PinWorkspaceItem 
 PrepareSkill Queue RecoverSession
 RecoverSteering RejectSuggestion RelocateWorkspaceSource RemoveExtension RemoveOldAttachments
 RemoveOptionSubstitution RemovePackages RemoveTerminalProfile RemoveWorkspace
 RemoveWorkspaceMount RenameDocumentFile RenameSession RenameSkill RenameWorkspace
 RenameWorkspaceSource ResetProviderFileLimit Restart ReviseSuggestion
 SaveAppearance SaveAppletArtifactAt SaveDraft SaveDraftAs SaveDraftToComputer SaveHistory
 SaveNotification SaveRun SaveTemplate SelectWorkspace Send SetAgentExecutable SetApprovals
 SetClaudeAPIKey SetCodexAppletApprovals SetCodexApprovals SetDefault SetDefaultForProvider
 SetDefaultProvider SetDocumentView SetExtensionCommandReview SetLogLevel SetNewWindowMode
 SetNotifyWhenNeeded SetOllamaApprovals SetOllamaFileScope SetOllamaRoundLimit SetPersonName SetPinned SetPinnedForProvider
 SetPromptSendKey SetProviderFileLimit SetSessionOption SetShowThinking SetSkillField
 SetTerminalShell SetUsagePage SetWindowKindMode SetWorkspace ShowChatLog ShowLogFolder SignIn
 StartPreparedExtensionTerminal StartSkill Steer Stop StopAndSteer StopBackgroundTask
 TerminalClose TerminalStart TerminalWrite UnhideSessions UnpinWorkspaceItem Unqueue
 UpdateDraft UploadToWorkspaceFolder WriteWorkspaceMount`)
	return result
}()

// newPageBridge validates the inventory before creating any page bindings.
func newPageBridge(target any, table map[string]pageMethodClass) (*bridge.Bridge, error) {
	policy := make(map[string]bool, len(table))
	for name, class := range table {
		switch class {
		case pageQuery, pageWork:
			policy[name] = true
		case notForPage:
			policy[name] = false
		default:
			return nil, fmt.Errorf("page method %q has invalid class %d", name, class)
		}
	}
	return bridge.NewRestricted(target, policy)
}
