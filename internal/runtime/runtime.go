// Package runtime manages llamafile server processes: starting one with a
// model, health-checking it and stopping it. The app runs two of them: one
// for chat and one (on the next port) for embeddings.
package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/settings"
	"github.com/devalexandre/llamafile-ui/internal/sysproc"
	"github.com/devalexandre/llamafile-ui/internal/tools"
)

const (
	StateMissing  = "missing"  // no llamafile binary found
	StateStopped  = "stopped"  // binary available, server not running
	StateStarting = "starting" // process launched, loading model
	StateRunning  = "running"  // /health answered 200
	StateError    = "error"
)

// Role configures what a server instance is for.
type Role struct {
	Name       string
	PortOffset int
	Args       func(cfg settings.Settings) []string
}

var (
	ChatRole = Role{Name: "chat", Args: func(cfg settings.Settings) []string {
		return []string{"-c", strconv.Itoa(cfg.ContextSize)}
	}}
	// EmbedRole serves /v1/embeddings. A large batch lets one request carry
	// several chunks; a single slot keeps the whole context per input.
	EmbedRole = Role{Name: "embed", PortOffset: 1, Args: func(settings.Settings) []string {
		return []string{"--embedding", "-c", "8192", "-b", "8192", "-ub", "8192", "-np", "1"}
	}}
)

type Status struct {
	Role       string `json:"role"`
	State      string `json:"state"`
	Message    string `json:"message"`
	Error      string `json:"error"`
	BinaryPath string `json:"binaryPath"`
	Version    string `json:"version"`
	Thin       bool   `json:"thin"`
	ModelID    string `json:"modelId"`
	ModelName  string `json:"modelName"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	BaseURL    string `json:"baseUrl"`
	// Port/host in settings differ from the running server.
	NeedsRestart bool `json:"needsRestart"`
}

// ModelResolver maps a model id to the files the server needs.
type ModelResolver interface {
	ModelPath(id string) string
	MMProjPath(id string) string
	DisplayName(id string) string
}

type Manager struct {
	role     Role
	mu       sync.Mutex
	settings *settings.Store
	models   ModelResolver
	status   Status
	cmd      *exec.Cmd
	exited   chan struct{}
	ready    chan struct{} // closed when the current start attempt finishes
	startErr error
	logs     []string
	runHost  string
	runPort  int

	OnChange func(Status)
	OnLog    func(line string)
}

func New(st *settings.Store, models ModelResolver, role Role) *Manager {
	m := &Manager{settings: st, models: models, role: role}
	m.status.Role = role.Name
	m.Detect()
	return m
}

func (m *Manager) port(cfg settings.Settings) int { return cfg.Port + m.role.PortOffset }

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *Manager) statusLocked() Status {
	s := m.status
	cfg := m.settings.Get()
	if s.State == StateRunning || s.State == StateStarting {
		s.Host, s.Port = m.runHost, m.runPort
		s.NeedsRestart = cfg.Host != m.runHost || m.port(cfg) != m.runPort
	} else {
		s.Host, s.Port = cfg.Host, m.port(cfg)
	}
	host := s.Host
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	s.BaseURL = fmt.Sprintf("http://%s:%d", host, s.Port)
	return s
}

func (m *Manager) emit() {
	s := m.Status()
	if m.OnChange != nil {
		m.OnChange(s)
	}
}

func (m *Manager) setState(state, message, errMsg string) {
	m.mu.Lock()
	m.status.State, m.status.Message, m.status.Error = state, message, errMsg
	m.mu.Unlock()
	m.emit()
}

func (m *Manager) Logs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.logs...)
}

func (m *Manager) appendLog(line string) {
	m.mu.Lock()
	m.logs = append(m.logs, line)
	if len(m.logs) > 400 {
		m.logs = m.logs[len(m.logs)-400:]
	}
	m.mu.Unlock()
	if m.OnLog != nil {
		m.OnLog(line)
	}
}

// Detect looks for the llamafile binary and updates the state.
func (m *Manager) Detect() Status {
	path, version := tools.Find(tools.Llamafile, m.settings.Get().LlamafilePath)
	m.mu.Lock()
	m.status.BinaryPath, m.status.Version, m.status.Thin = path, version, tools.IsThin(path)
	if m.status.State != StateRunning && m.status.State != StateStarting {
		if path == "" {
			m.status.State = StateMissing
			m.status.Message = "llamafile não encontrado"
		} else {
			m.status.State = StateStopped
			m.status.Message = "Parado"
		}
		m.status.Error = ""
	}
	m.mu.Unlock()
	m.emit()
	return m.Status()
}

// Installed marks a freshly downloaded binary as executable and re-detects.
func (m *Manager) Installed(path string) error {
	if err := os.Chmod(path, 0o755); err != nil {
		return err
	}
	m.Detect()
	return nil
}

func portFree(host string, port int) bool {
	l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// Start launches the server with modelID, stopping any running instance.
// It returns immediately; use WaitReady to block until the model is loaded.
func (m *Manager) Start(modelID string) error {
	m.Stop()

	cfg := m.settings.Get()
	port := m.port(cfg)
	m.mu.Lock()
	bin := m.status.BinaryPath
	m.mu.Unlock()
	if bin == "" {
		m.Detect()
		return errors.New("llamafile não encontrado — baixe-o em Configurações")
	}
	modelPath := m.models.ModelPath(modelID)
	if modelPath == "" {
		return fmt.Errorf("o modelo %q não está instalado", modelID)
	}
	if !portFree(cfg.Host, port) {
		err := fmt.Errorf("a porta %d já está em uso — escolha outra em Configurações", port)
		m.setState(StateError, "Porta em uso", err.Error())
		return err
	}

	args := []string{"--server", "--nologo", "-m", modelPath,
		"--host", cfg.Host, "--port", strconv.Itoa(port), "--gpu", cfg.GPU, "--no-webui"}
	args = append(args, m.role.Args(cfg)...)
	if mm := m.models.MMProjPath(modelID); mm != "" && m.role.Name == ChatRole.Name {
		args = append(args, "--mmproj", mm)
	}
	if cfg.Threads > 0 {
		args = append(args, "-t", strconv.Itoa(cfg.Threads))
	}

	cmd := tools.Command(context.Background(), bin, args...)
	cmd.Dir = filepath.Dir(modelPath)
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	m.appendLog(fmt.Sprintf("$ %s %s", filepath.Base(bin), strings.Join(args, " ")))
	if err := cmd.Start(); err != nil {
		m.setState(StateError, "Falha ao iniciar", err.Error())
		return err
	}

	exited := make(chan struct{})
	ready := make(chan struct{})
	m.mu.Lock()
	m.cmd, m.exited, m.ready, m.startErr = cmd, exited, ready, nil
	m.runHost, m.runPort = cfg.Host, port
	m.status.ModelID = modelID
	m.status.ModelName = m.models.DisplayName(modelID)
	m.status.State = StateStarting
	m.status.Message = "Carregando " + m.status.ModelName + "…"
	m.status.Error = ""
	m.mu.Unlock()
	m.emit()

	go m.pipeLogs(stdout)
	go func() {
		err := cmd.Wait()
		close(exited)
		m.mu.Lock()
		current := m.cmd == cmd
		if current {
			m.cmd = nil
		}
		wasRunning := m.status.State == StateRunning || m.status.State == StateStarting
		m.mu.Unlock()
		if !current {
			return
		}
		msg := "Servidor encerrado"
		if err != nil {
			msg = "O llamafile encerrou inesperadamente"
		}
		if wasRunning {
			if detail := lastError(m.Logs()); err != nil && detail != "" {
				m.setState(StateError, msg, detail)
			} else {
				m.setState(StateStopped, msg, "")
			}
		}
		m.finishStart(cmd, errors.New(msg))
	}()
	go m.waitHealthy(cmd, exited)
	return nil
}

func lastError(lines []string) string {
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-40; i-- {
		l := strings.ToLower(lines[i])
		if strings.Contains(l, "error") || strings.Contains(l, "failed") || strings.Contains(l, "erro") {
			return strings.TrimSpace(lines[i])
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[len(lines)-1])
	}
	return ""
}

func (m *Manager) pipeLogs(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		m.appendLog(sc.Text())
	}
}

func (m *Manager) finishStart(cmd *exec.Cmd, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd != cmd {
		return
	}
	if m.ready != nil {
		select {
		case <-m.ready:
		default:
			m.startErr = err
			close(m.ready)
		}
	}
}

func (m *Manager) waitHealthy(cmd *exec.Cmd, exited chan struct{}) {
	url := m.Status().BaseURL + "/health"
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-exited:
			return
		case <-time.After(500 * time.Millisecond):
		}
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			m.mu.Lock()
			if m.cmd != cmd {
				m.mu.Unlock()
				return
			}
			m.status.State = StateRunning
			m.status.Message = "Pronto"
			m.mu.Unlock()
			m.emit()
			m.finishStart(cmd, nil)
			return
		}
	}
	m.setState(StateError, "Tempo esgotado", "o modelo demorou demais para carregar")
	m.finishStart(cmd, errors.New("tempo esgotado ao carregar o modelo"))
}

// WaitReady blocks until the current start attempt succeeds or fails.
func (m *Manager) WaitReady(ctx context.Context) error {
	m.mu.Lock()
	ready := m.ready
	state := m.status.State
	m.mu.Unlock()
	if state == StateRunning {
		return nil
	}
	if ready == nil {
		return errors.New("o servidor não está em execução")
	}
	select {
	case <-ready:
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.startErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// EnsureModel makes sure the server is running modelID, (re)starting it if needed.
func (m *Manager) EnsureModel(ctx context.Context, modelID string) error {
	s := m.Status()
	if s.ModelID != modelID || (s.State != StateRunning && s.State != StateStarting) {
		if err := m.Start(modelID); err != nil {
			return err
		}
	}
	return m.WaitReady(ctx)
}

// Stop terminates the server (and its children) gracefully, then forcefully.
func (m *Manager) Stop() {
	m.mu.Lock()
	cmd, exited := m.cmd, m.exited
	m.cmd = nil
	m.mu.Unlock()
	if cmd == nil {
		return
	}
	sysproc.Terminate(cmd)
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		sysproc.Kill(cmd)
		<-exited
	}
	m.mu.Lock()
	if m.ready != nil {
		select {
		case <-m.ready:
		default:
			m.startErr = errors.New("servidor parado")
			close(m.ready)
		}
	}
	m.status.ModelID, m.status.ModelName = "", ""
	m.mu.Unlock()
	m.setState(StateStopped, "Parado", "")
	m.Detect()
}

// Restart relaunches the current model (e.g. after a port change).
func (m *Manager) Restart() error {
	id := m.Status().ModelID
	if id == "" {
		return errors.New("nenhum modelo carregado")
	}
	return m.Start(id)
}
