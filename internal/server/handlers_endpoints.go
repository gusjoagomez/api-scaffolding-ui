package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileEntry represents a file or directory in the tree.
type FileEntry struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Children []FileEntry `json:"children,omitempty"`
}

// handleEndpoints renders the file-browser page for a project's rootdir.
func (s *Server) handleEndpoints(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	if projectName == "" {
		http.Error(w, "projectname is required", http.StatusBadRequest)
		return
	}

	// Fetch rootdir from the database.
	var rootDir sql.NullString
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT rootdir FROM %s.project WHERE projectname = $1", s.cfg.DBSchema),
		projectName,
	).Scan(&rootDir)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	if !rootDir.Valid || rootDir.String == "" {
		http.Error(w, "rootdir is not configured for this project", http.StatusBadRequest)
		return
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/endpoints_browser.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	data := struct {
		ProjectName string
		RootDir     string
	}{
		ProjectName: projectName,
		RootDir:     rootDir.String,
	}

	if err := tmpl.Execute(w, data); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

// handleEndpointsTree returns a JSON tree of yaml/json files under the rootdir.
func (s *Server) handleEndpointsTree(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	if projectName == "" {
		http.Error(w, "projectname is required", http.StatusBadRequest)
		return
	}

	var rootDir sql.NullString
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT rootdir FROM %s.project WHERE projectname = $1", s.cfg.DBSchema),
		projectName,
	).Scan(&rootDir)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if !rootDir.Valid || rootDir.String == "" {
		http.Error(w, "rootdir not configured", http.StatusBadRequest)
		return
	}

	root := rootDir.String

	// Verify directory exists
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]FileEntry{})
		return
	}

	tree := buildTree(root, root)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

// buildTree recursively scans a directory and returns entries for dirs and yaml/json files.
func buildTree(basePath, currentPath string) []FileEntry {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil
	}

	var result []FileEntry

	// Sort: directories first, then files, both alphabetically.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(currentPath, name)

		// Get relative path from basePath for display
		relPath, _ := filepath.Rel(basePath, fullPath)

		if entry.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(name, ".") {
				continue
			}
			children := buildTree(basePath, fullPath)
			// Only include directory if it has matching files (directly or nested)
			if len(children) > 0 {
				result = append(result, FileEntry{
					Name:     name,
					Path:     relPath,
					IsDir:    true,
					Children: children,
				})
			}
		} else {
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".yaml" || ext == ".yml" || ext == ".json" {
				result = append(result, FileEntry{
					Name:  name,
					Path:  relPath,
					IsDir: false,
				})
			}
		}
	}

	return result
}

// handleEndpointsRead returns the content of a file as JSON.
func (s *Server) handleEndpointsRead(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	filePath := r.URL.Query().Get("path")
	if projectName == "" || filePath == "" {
		http.Error(w, "projectname and path are required", http.StatusBadRequest)
		return
	}

	rootDir, err := s.getProjectRootDir(projectName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Resolve and validate path is under rootDir.
	absPath := filepath.Join(rootDir, filePath)
	absPath, err = filepath.Abs(absPath)
	if err != nil || !strings.HasPrefix(absPath, rootDir) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		http.Error(w, "cannot read file", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"content": string(content),
		"path":    filePath,
	})
}

// handleEndpointsSave writes updated content to a file.
func (s *Server) handleEndpointsSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProjectName string `json:"projectname"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.ProjectName == "" || req.Path == "" {
		http.Error(w, "projectname and path are required", http.StatusBadRequest)
		return
	}

	rootDir, err := s.getProjectRootDir(req.ProjectName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	absPath := filepath.Join(rootDir, req.Path)
	absPath, err = filepath.Abs(absPath)
	if err != nil || !strings.HasPrefix(absPath, rootDir) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		http.Error(w, "cannot create directory", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(absPath, []byte(req.Content), 0644); err != nil {
		http.Error(w, "cannot write file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "File saved successfully",
	})
}

// getProjectRootDir fetches and validates the rootdir for a project.
func (s *Server) getProjectRootDir(projectName string) (string, error) {
	var rootDir sql.NullString
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT rootdir FROM %s.project WHERE projectname = $1", s.cfg.DBSchema),
		projectName,
	).Scan(&rootDir)
	if err != nil {
		return "", fmt.Errorf("project not found")
	}
	if !rootDir.Valid || rootDir.String == "" {
		return "", fmt.Errorf("rootdir not configured")
	}

	absRoot, err := filepath.Abs(rootDir.String)
	if err != nil {
		return "", fmt.Errorf("invalid rootdir")
	}
	return absRoot, nil
}
