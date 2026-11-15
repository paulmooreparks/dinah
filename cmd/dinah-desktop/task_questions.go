package main

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"sync"

	"studio/internal/services"
	"studio/internal/taskhost"
)

type taskQuestionState struct {
	once      sync.Once
	mu        sync.Mutex
	questions *taskhost.Questions
	err       error
	// These private ports allow isolated App fixtures to bind a consumer store.
	describe func(context.Context, taskhost.Scope) (taskhost.QuestionOwner, error)
	check    func(context.Context, taskhost.Scope) error
}

func (a *App) taskQuestionService() (*taskhost.Questions, error) {
	a.taskQuestions.once.Do(func() {
		describe, check := a.taskQuestions.describe, a.taskQuestions.check
		if describe == nil {
			describe = a.describeTaskQuestion
		}
		if check == nil {
			check = a.checkTaskIdentity
		}
		questions, err := taskhost.NewQuestions(taskhost.QuestionPorts{Describe: describe, Check: check, Emit: a.emit})
		a.taskQuestions.mu.Lock()
		a.taskQuestions.questions, a.taskQuestions.err = questions, err
		a.taskQuestions.mu.Unlock()
	})
	a.taskQuestions.mu.Lock()
	defer a.taskQuestions.mu.Unlock()
	return a.taskQuestions.questions, a.taskQuestions.err
}

// PendingTaskPermissions observes task-owned questions independently of chat
// permissions. Reading it constructs no service, task, chat or provider.
func (a *App) PendingTaskPermissions() []taskhost.Question {
	a.taskQuestions.mu.Lock()
	questions := a.taskQuestions.questions
	a.taskQuestions.mu.Unlock()
	return questions.Pending()
}

func (a *App) describeTaskQuestion(ctx context.Context, scope taskhost.Scope) (taskhost.QuestionOwner, error) {
	a.taskServices.mu.Lock()
	window := a.taskServices.window
	a.taskServices.mu.Unlock()
	metadata, registration, err := window.DescribeTask(ctx, scope.Service, scope.Workspace, scope.Task)
	if err != nil {
		return taskhost.QuestionOwner{}, err
	}
	if metadata.Workspace != scope.Workspace || metadata.Task != scope.Task {
		return taskhost.QuestionOwner{}, taskRefusal("not_authorized", "task presentation belongs to another task or workspace")
	}
	a.mu.Lock()
	workspaceName := ""
	for _, record := range a.settings.WorkspaceRecords {
		if record.ID == scope.Workspace {
			workspaceName = record.Name
			break
		}
	}
	a.mu.Unlock()
	if workspaceName == "" {
		workspaceName = "Untitled workspace"
	}
	target := taskQuestionTarget(metadata, registration)
	return taskhost.QuestionOwner{Service: scope.Service, ServiceName: registration.Name, Workspace: scope.Workspace, WorkspaceName: workspaceName, Task: scope.Task, Generation: scope.Generation, TaskTitle: metadata.Title, Target: target}, nil
}

func taskQuestionTarget(metadata services.TaskPresentation, registration services.WindowPresentation) taskhost.QuestionTarget {
	if metadata.Page == "" {
		reason := metadata.Unavailable
		if reason == "" {
			reason = "The task's page is unavailable."
		}
		return taskhost.QuestionTarget{Reason: reason}
	}
	if !windowToolID.MatchString(metadata.Page) || !slices.Contains(registration.Pages, metadata.Page) || len(metadata.State) > 8192 || strings.ContainsRune(metadata.State, 0) {
		return taskhost.QuestionTarget{Reason: "The task page is outside this service's registered pages."}
	}
	if _, err := url.ParseQuery(metadata.State); err != nil {
		return taskhost.QuestionTarget{Reason: "The task's page state is invalid."}
	}
	query := url.Values{"open": {metadata.Page}, "view." + metadata.Page: {metadata.State}}
	return taskhost.QuestionTarget{Href: "?" + query.Encode()}
}

func (a *App) askTaskQuestion(ctx context.Context, scope taskhost.Scope, text string, options []taskhost.QuestionOption) (string, error) {
	questions, err := a.taskQuestionService()
	if err != nil {
		return "", err
	}
	return questions.Ask(ctx, scope, text, options)
}

func (a *App) askTypedTaskQuestion(ctx context.Context, scope taskhost.Scope, text string, spec taskhost.QuestionSpec) (taskhost.QuestionAnswer, error) {
	questions, err := a.taskQuestionService()
	if err != nil {
		return taskhost.QuestionAnswer{}, err
	}
	return questions.AskTyped(ctx, scope, text, spec)
}

// AnswerTaskQuestion is the person's window entry point, never a service port.
func (a *App) AnswerTaskQuestion(key, answerJSON string) error {
	answer, err := decodeTaskQuestionAnswer(answerJSON)
	if err != nil {
		return err
	}
	a.taskQuestions.mu.Lock()
	questions := a.taskQuestions.questions
	a.taskQuestions.mu.Unlock()
	if questions == nil {
		return taskRefusal("stale", "the task question is no longer pending")
	}
	return questions.AnswerTyped(key, answer)
}

func (a *App) closeTaskQuestions() {
	a.taskQuestions.mu.Lock()
	questions := a.taskQuestions.questions
	a.taskQuestions.mu.Unlock()
	questions.Close()
}
