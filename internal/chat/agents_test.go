package chat

import (
	"path/filepath"
	"strings"
	"testing"
)

type agentBackend struct{ Backend }

func (agentBackend) DefaultSystemPrompt() string       { return "padrão" }
func (agentBackend) SupportsVision(string) bool        { return false }
func (agentBackend) ContextSize() int                  { return 8192 }
func (agentBackend) AttachmentPath(name string) string { return name }

func TestAgentsAndProjects(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveAgent(Agent{Name: "  "}); err == nil {
		t.Fatal("agent without name accepted")
	}
	rev, err := st.SaveAgent(Agent{Name: "Revisor", Icon: "📝", SystemPrompt: "Revise textos.", Temperature: 0.2, CollectionID: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	proj, err := st.SaveProject(Project{Name: "Livro", AgentID: rev.ID})
	if err != nil {
		t.Fatal(err)
	}

	// A conversation inside the project inherits its agent and the agent's documents.
	c, err := st.CreateWith(Conversation{ModelID: "m", ProjectID: proj.ID})
	if err != nil {
		t.Fatal(err)
	}
	if c.AgentID != rev.ID || c.CollectionID != "manual" {
		t.Fatalf("project defaults not applied: %+v", c)
	}
	plain, _ := st.Create("m", "")

	// Project conversations stay out of the main list.
	main, _ := st.List("")
	if len(main) != 1 || main[0].ID != plain.ID {
		t.Fatalf("main list: %+v", main)
	}
	in, _ := st.ProjectConversations(proj.ID)
	if len(in) != 1 || in[0].ID != c.ID {
		t.Fatalf("project list: %+v", in)
	}

	// The agent's instructions replace the default system prompt.
	svc := NewService(st, agentBackend{}, Events{})
	msgs, err := svc.buildHistory(c, nil)
	if err != nil || len(msgs) == 0 || !strings.Contains(msgs[0].Content.(string), "Revise textos.") {
		t.Fatalf("system prompt: %+v %v", msgs, err)
	}
	if msgs, _ := svc.buildHistory(plain, nil); msgs[0].Content != "padrão" {
		t.Fatalf("default prompt: %+v", msgs)
	}

	// Deleting the agent keeps the conversation with the default assistant.
	if err := st.DeleteAgent(rev.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Get(c.ID); got.AgentID != "" {
		t.Fatal("conversation still points to a deleted agent")
	}
	// Deleting the project moves its conversations to the main list.
	if err := st.DeleteProject(proj.ID, false); err != nil {
		t.Fatal(err)
	}
	if main, _ := st.List(""); len(main) != 2 {
		t.Fatalf("conversations lost: %d", len(main))
	}
}
