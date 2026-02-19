package server

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"api-scaffolding/internal/config"
)

type Server struct {
	cfg *config.Config
	db  *sql.DB
}

func NewServer(cfg *config.Config, db *sql.DB) *Server {
	return &Server{
		cfg: cfg,
		db:  db,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Routes
	mux.HandleFunc("/", s.handleProjectsList)
	mux.HandleFunc("/projects/new", s.handleProjectNew)
	mux.HandleFunc("/projects/save", s.handleProjectSave) // Create and Update
	mux.HandleFunc("/projects/edit", s.handleProjectEdit)

	mux.HandleFunc("/connections", s.handleConnectionsList)
	mux.HandleFunc("/connections/new", s.handleConnectionNew)
	mux.HandleFunc("/connections/edit", s.handleConnectionEdit)
	mux.HandleFunc("/connections/save", s.handleConnectionSave)
	mux.HandleFunc("/connections/tables", s.handleTablesList)
	mux.HandleFunc("/connections/get-tables", s.handleGetInfoTables)
	mux.HandleFunc("/file-templates", s.handleFileTemplatesList)
	mux.HandleFunc("/connections/generate", s.handleGenerate)

	// Subsystems
	mux.HandleFunc("/subsystems", s.handleSubsystemsList)
	mux.HandleFunc("/subsystems/new", s.handleSubsystemNew)
	mux.HandleFunc("/subsystems/edit", s.handleSubsystemEdit)
	mux.HandleFunc("/subsystems/save", s.handleSubsystemSave)
	mux.HandleFunc("/subsystems/delete", s.handleSubsystemDelete)

	// Endpoints browser
	mux.HandleFunc("/endpoints", s.handleEndpoints)
	mux.HandleFunc("/endpoints/tree", s.handleEndpointsTree)
	mux.HandleFunc("/endpoints/read", s.handleEndpointsRead)
	mux.HandleFunc("/endpoints/save", s.handleEndpointsSave)

	addr := fmt.Sprintf(":%s", "4000") // Default to 4000 as requested
	log.Printf("Starting server on http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

func renderError(w http.ResponseWriter, err error, status int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), status)
}
