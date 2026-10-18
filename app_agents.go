package main

import "github.com/devalexandre/llamafile-ui/internal/chat"

// ------------------------------------------------------- agents & projects

func (a *App) ListAgents() ([]chat.Agent, error) { return a.chat.Store().ListAgents() }

func (a *App) SaveAgent(ag chat.Agent) (chat.Agent, error) {
	saved, err := a.chat.Store().SaveAgent(ag)
	if err == nil {
		a.emit("agents:changed")
	}
	return saved, err
}

func (a *App) DeleteAgent(id string) error {
	if err := a.chat.Store().DeleteAgent(id); err != nil {
		return err
	}
	a.emit("agents:changed")
	a.emit("projects:changed")
	a.emit("chat:conversations")
	return nil
}

func (a *App) ListProjects() ([]chat.Project, error) { return a.chat.Store().ListProjects() }

func (a *App) SaveProject(p chat.Project) (chat.Project, error) {
	saved, err := a.chat.Store().SaveProject(p)
	if err == nil {
		a.emit("projects:changed")
	}
	return saved, err
}

// DeleteProject removes a project; withConversations also deletes its chats,
// otherwise they move to the main list.
func (a *App) DeleteProject(id string, withConversations bool) error {
	if err := a.chat.Store().DeleteProject(id, withConversations); err != nil {
		return err
	}
	a.emit("projects:changed")
	a.emit("chat:conversations")
	return nil
}

func (a *App) ListProjectConversations(projectID string) ([]chat.Conversation, error) {
	return a.chat.Store().ProjectConversations(projectID)
}

// SetConversationAgent switches who answers in a conversation ("" = default assistant).
func (a *App) SetConversationAgent(conversationID, agentID string) error {
	if err := a.chat.Store().SetAgent(conversationID, agentID); err != nil {
		return err
	}
	a.emit("chat:conversations")
	return nil
}

// MoveConversation puts a conversation inside a project ("" = main list).
func (a *App) MoveConversation(conversationID, projectID string) error {
	if err := a.chat.Store().SetProject(conversationID, projectID); err != nil {
		return err
	}
	a.emit("chat:conversations")
	a.emit("projects:changed")
	return nil
}

// SendMessageWith sends a message; new conversations may start with an agent
// and inside a project.
func (a *App) SendMessageWith(req chat.SendRequest) (chat.Conversation, error) {
	conv, err := a.chat.SendWith(req)
	if err == nil && req.ProjectID != "" {
		a.emit("projects:changed")
	}
	return conv, err
}
