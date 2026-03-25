package app

import (
	"fmt"
	"sync"

	"claude-squad/config"
	ptyPkg "claude-squad/pty"
)

type SessionAPIOptions struct {
	DataDir string
}

type CreateOptions struct {
	Title   string `json:"title"`
	Path    string `json:"path"`
	Program string `json:"program"`
	Branch  string `json:"branch"`
	AutoYes bool   `json:"autoYes"`
	InPlace bool   `json:"inPlace"`
}

type SessionInfo struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Program string `json:"program"`
	Status  string `json:"status"`
}

type SessionStatus struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Branch    string    `json:"branch"`
	DiffStats DiffStats `json:"diffStats"`
	HasPrompt bool      `json:"hasPrompt"`
}

type DiffStats struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

type SessionAPI struct {
	mu         sync.RWMutex
	sessions   map[string]*managedSession
	ptyManager *ptyPkg.Manager
	wsServer   *ptyPkg.WebSocketServer
	wsPort     int
	cfg        *config.Config
}

type managedSession struct {
	info  SessionInfo
	ptyID string
}

func NewSessionAPI(opts SessionAPIOptions) (*SessionAPI, error) {
	mgr := ptyPkg.NewManager()
	ws := ptyPkg.NewWebSocketServer(mgr)

	port, err := ws.ListenAndServe()
	if err != nil {
		return nil, fmt.Errorf("start websocket server: %w", err)
	}

	cfg := config.LoadConfig()

	return &SessionAPI{
		sessions:   make(map[string]*managedSession),
		ptyManager: mgr,
		wsServer:   ws,
		wsPort:     port,
		cfg:        cfg,
	}, nil
}

func (api *SessionAPI) CreateSession(opts CreateOptions) (*SessionInfo, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	program := opts.Program
	if program == "" {
		program = api.cfg.DefaultProgram
	}

	id := fmt.Sprintf("session-%s", opts.Title)

	info := SessionInfo{
		ID:      id,
		Title:   opts.Title,
		Path:    opts.Path,
		Branch:  opts.Branch,
		Program: program,
		Status:  "loading",
	}

	api.sessions[id] = &managedSession{
		info: info,
	}

	return &info, nil
}

func (api *SessionAPI) LoadSessions() ([]SessionInfo, error) {
	api.mu.RLock()
	defer api.mu.RUnlock()

	result := make([]SessionInfo, 0, len(api.sessions))
	for _, s := range api.sessions {
		result = append(result, s.info)
	}
	return result, nil
}

func (api *SessionAPI) GetWebSocketPort() int {
	return api.wsPort
}

func (api *SessionAPI) GetConfig() (*config.Config, error) {
	return api.cfg, nil
}

func (api *SessionAPI) Close() {
	api.ptyManager.Close()
}
