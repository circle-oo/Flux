package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/circle-oo/flux/internal/vault"
)

// registerKnowledgeRoutes registers all /api/knowledge/* routes.
func (s *Server) registerKnowledgeRoutes() {
	if s.vault == nil {
		return
	}

	s.mux.Handle("GET /api/knowledge/notes", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeListNotes)))
	s.mux.Handle("GET /api/knowledge/notes/{path...}", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeReadNote)))
	s.mux.Handle("POST /api/knowledge/notes", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeCreateNote)))
	s.mux.Handle("PUT /api/knowledge/notes/{path...}", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeUpdateNote)))
	s.mux.Handle("DELETE /api/knowledge/notes/{path...}", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeDeleteNote)))
	s.mux.Handle("GET /api/knowledge/search", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeSearch)))
	s.mux.Handle("GET /api/knowledge/daily", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeDaily)))
	s.mux.Handle("POST /api/knowledge/daily/append", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeDailyAppend)))
	s.mux.Handle("GET /api/knowledge/stats", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeStats)))
	s.mux.Handle("GET /api/knowledge/frontmatter/{path...}", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeFrontmatter)))
	s.mux.Handle("GET /api/knowledge/folders", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeFolders)))
	s.mux.Handle("GET /api/knowledge/recent", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeRecent)))
	s.mux.Handle("GET /api/knowledge/orphans", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeOrphans)))
	s.mux.Handle("GET /api/knowledge/health", s.authMiddleware(http.HandlerFunc(s.handleKnowledgeHealth)))
}

// handleKnowledgeListNotes handles GET /api/knowledge/notes
func (s *Server) handleKnowledgeListNotes(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folder")
	notes, err := s.vault.List(folder)
	if err != nil {
		serverError(w, "failed to list notes", "error", err)
		return
	}
	if notes == nil {
		notes = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"notes": notes})
}

// handleKnowledgeReadNote handles GET /api/knowledge/notes/{path...}
func (s *Server) handleKnowledgeReadNote(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	content, err := s.vault.Read(notePath)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("note not found: %s", notePath))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":    notePath,
		"content": content,
	})
}

// handleKnowledgeCreateNote handles POST /api/knowledge/notes
func (s *Server) handleKnowledgeCreateNote(w http.ResponseWriter, r *http.Request) {
	if s.vaultWriter == nil {
		writeError(w, http.StatusServiceUnavailable, "vault writer not available")
		return
	}

	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	if err := s.vaultWriter.Write(req.Path, req.Content, vault.ModeCreate); err != nil {
		serverError(w, "failed to create note", "path", req.Path, "error", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"path":    req.Path,
		"content": req.Content,
	})
}

// handleKnowledgeUpdateNote handles PUT /api/knowledge/notes/{path...}
func (s *Server) handleKnowledgeUpdateNote(w http.ResponseWriter, r *http.Request) {
	if s.vaultWriter == nil {
		writeError(w, http.StatusServiceUnavailable, "vault writer not available")
		return
	}

	notePath := r.PathValue("path")
	if notePath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if err := s.vaultWriter.Write(notePath, req.Content, vault.ModeReplace); err != nil {
		serverError(w, "failed to update note", "path", notePath, "error", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":    notePath,
		"content": req.Content,
	})
}

// handleKnowledgeDeleteNote handles DELETE /api/knowledge/notes/{path...}
func (s *Server) handleKnowledgeDeleteNote(w http.ResponseWriter, r *http.Request) {
	if s.vaultWriter == nil {
		writeError(w, http.StatusServiceUnavailable, "vault writer not available")
		return
	}

	notePath := r.PathValue("path")
	if notePath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	if err := s.vaultWriter.Delete(notePath); err != nil {
		serverError(w, "failed to delete note", "path", notePath, "error", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": notePath})
}

// handleKnowledgeSearch handles GET /api/knowledge/search?q=query
func (s *Server) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q parameter is required")
		return
	}

	results, err := s.vault.Search(query)
	if err != nil {
		serverError(w, "failed to search vault", "error", err)
		return
	}

	var matches []string
	if results != "" {
		matches = strings.Split(strings.TrimRight(results, "\n"), "\n")
	}
	if matches == nil {
		matches = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"query":   query,
		"results": matches,
	})
}

// handleKnowledgeDaily handles GET /api/knowledge/daily
func (s *Server) handleKnowledgeDaily(w http.ResponseWriter, r *http.Request) {
	today := time.Now().Format("2006-01-02")
	dailyPath := fmt.Sprintf("daily/%s", today)

	content, err := s.vault.Read(dailyPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"date":    today,
			"path":    dailyPath,
			"content": "",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"date":    today,
		"path":    dailyPath,
		"content": content,
	})
}

// handleKnowledgeDailyAppend handles POST /api/knowledge/daily/append
func (s *Server) handleKnowledgeDailyAppend(w http.ResponseWriter, r *http.Request) {
	if s.vaultWriter == nil {
		writeError(w, http.StatusServiceUnavailable, "vault writer not available")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	if err := s.vaultWriter.DailyAppend(req.Content); err != nil {
		serverError(w, "failed to append to daily note", "error", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "appended"})
}

// handleKnowledgeStats handles GET /api/knowledge/stats
func (s *Server) handleKnowledgeStats(w http.ResponseWriter, r *http.Request) {
	notes, err := s.vault.List("")
	if err != nil {
		serverError(w, "failed to get vault stats", "error", err)
		return
	}

	folders := make(map[string]bool)
	var totalSize int64
	for _, note := range notes {
		dir := filepath.Dir(note)
		if dir != "." {
			folders[dir] = true
		}
		full := filepath.Join(s.config.Vault.Path, note)
		if info, err := os.Stat(full); err == nil {
			totalSize += info.Size()
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"note_count":   len(notes),
		"folder_count": len(folders),
		"total_size":   totalSize,
		"mode":         s.vault.Mode(),
		"healthy":      s.vault.IsHealthy(),
	})
}

// handleKnowledgeFrontmatter handles GET /api/knowledge/frontmatter/{path...}
func (s *Server) handleKnowledgeFrontmatter(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	fm, err := s.vault.Frontmatter(notePath)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("note not found: %s", notePath))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":        notePath,
		"frontmatter": fm,
	})
}

// handleKnowledgeFolders handles GET /api/knowledge/folders
func (s *Server) handleKnowledgeFolders(w http.ResponseWriter, r *http.Request) {
	notes, err := s.vault.List("")
	if err != nil {
		serverError(w, "failed to list folders", "error", err)
		return
	}

	folderSet := make(map[string]bool)
	for _, note := range notes {
		parts := strings.SplitN(note, "/", 2)
		if len(parts) > 1 {
			folderSet[parts[0]] = true
		}
	}

	folders := make([]string, 0, len(folderSet))
	for f := range folderSet {
		folders = append(folders, f)
	}
	sort.Strings(folders)

	writeJSON(w, http.StatusOK, map[string]interface{}{"folders": folders})
}

// handleKnowledgeRecent handles GET /api/knowledge/recent
func (s *Server) handleKnowledgeRecent(w http.ResponseWriter, r *http.Request) {
	notes, err := s.vault.List("")
	if err != nil {
		serverError(w, "failed to list notes", "error", err)
		return
	}

	type noteInfo struct {
		Path    string `json:"path"`
		ModTime string `json:"mod_time"`
	}

	var infos []noteInfo
	for _, note := range notes {
		full := filepath.Join(s.config.Vault.Path, note)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		infos = append(infos, noteInfo{
			Path:    note,
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ModTime > infos[j].ModTime
	})

	if len(infos) > 20 {
		infos = infos[:20]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"notes": infos})
}

// handleKnowledgeOrphans handles GET /api/knowledge/orphans
func (s *Server) handleKnowledgeOrphans(w http.ResponseWriter, r *http.Request) {
	notes, err := s.vault.List("")
	if err != nil {
		serverError(w, "failed to list notes", "error", err)
		return
	}

	// Check which notes are linked from other notes via [[wikilinks]]
	linked := make(map[string]bool)
	for _, note := range notes {
		content, err := s.vault.Read(note)
		if err != nil {
			continue
		}
		for i := 0; i < len(content)-3; i++ {
			if content[i] == '[' && content[i+1] == '[' {
				end := strings.Index(content[i+2:], "]]")
				if end >= 0 {
					link := content[i+2 : i+2+end]
					if pipeIdx := strings.Index(link, "|"); pipeIdx >= 0 {
						link = link[:pipeIdx]
					}
					linked[link] = true
				}
			}
		}
	}

	var orphans []string
	for _, note := range notes {
		name := strings.TrimSuffix(filepath.Base(note), ".md")
		nameWithDir := strings.TrimSuffix(note, ".md")
		if !linked[name] && !linked[nameWithDir] {
			orphans = append(orphans, note)
		}
	}
	if orphans == nil {
		orphans = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"orphans": orphans})
}

// handleKnowledgeHealth handles GET /api/knowledge/health
func (s *Server) handleKnowledgeHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mode":    s.vault.Mode(),
		"healthy": s.vault.IsHealthy(),
	})
}
