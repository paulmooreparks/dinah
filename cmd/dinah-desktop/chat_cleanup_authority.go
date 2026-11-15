package main

func (a *App) cancelChatAppletWork(workspace, id string) {
	a.appletMu.Lock()
	service := a.appletHost
	a.appletMu.Unlock()
	if service != nil {
		service.CancelOwner(workspace, "chat", id)
	}
}
