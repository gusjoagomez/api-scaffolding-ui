package server

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"

	"api-scaffolding/internal/models"
)

func (s *Server) handleSubsystemsList(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	if projectName == "" {
		http.Error(w, "projectname is required", http.StatusBadRequest)
		return
	}

	rows, err := s.db.Query(
		fmt.Sprintf("SELECT projectname, subsystem, details FROM %s.subsystem WHERE projectname = $1 ORDER BY subsystem", s.cfg.DBSchema),
		projectName,
	)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var subsystems []models.Subsystem
	for rows.Next() {
		var sub models.Subsystem
		if err := rows.Scan(&sub.ProjectName, &sub.Subsystem, &sub.Details); err != nil {
			renderError(w, err, http.StatusInternalServerError)
			return
		}
		subsystems = append(subsystems, sub)
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/subsystems_list.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	data := struct {
		ProjectName string
		Subsystems  []models.Subsystem
	}{
		ProjectName: projectName,
		Subsystems:  subsystems,
	}

	if err := tmpl.Execute(w, data); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleSubsystemNew(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	if projectName == "" {
		http.Error(w, "projectname is required", http.StatusBadRequest)
		return
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/subsystem_form.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	data := struct {
		ProjectName string
		Subsystem   *models.Subsystem
	}{
		ProjectName: projectName,
		Subsystem:   nil,
	}

	if err := tmpl.Execute(w, data); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleSubsystemEdit(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	subsystemName := r.URL.Query().Get("subsystem")
	if projectName == "" || subsystemName == "" {
		http.Error(w, "projectname and subsystem are required", http.StatusBadRequest)
		return
	}

	row := s.db.QueryRow(
		fmt.Sprintf("SELECT projectname, subsystem, details FROM %s.subsystem WHERE projectname = $1 AND subsystem = $2", s.cfg.DBSchema),
		projectName, subsystemName,
	)
	var sub models.Subsystem
	if err := row.Scan(&sub.ProjectName, &sub.Subsystem, &sub.Details); err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/subsystem_form.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	data := struct {
		ProjectName string
		Subsystem   *models.Subsystem
	}{
		ProjectName: projectName,
		Subsystem:   &sub,
	}

	if err := tmpl.Execute(w, data); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleSubsystemSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectName := r.FormValue("projectname")
	subsystemName := r.FormValue("subsystem")
	details := r.FormValue("details")
	isNew := r.FormValue("is_new") == "true"

	if projectName == "" || subsystemName == "" {
		http.Error(w, "projectname and subsystem are required", http.StatusBadRequest)
		return
	}

	var err error
	if isNew {
		_, err = s.db.Exec(
			fmt.Sprintf("INSERT INTO %s.subsystem (projectname, subsystem, details) VALUES ($1, $2, $3)", s.cfg.DBSchema),
			projectName, subsystemName, details,
		)
	} else {
		_, err = s.db.Exec(
			fmt.Sprintf("UPDATE %s.subsystem SET details = $3 WHERE projectname = $1 AND subsystem = $2", s.cfg.DBSchema),
			projectName, subsystemName, details,
		)
	}

	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/subsystems?projectname="+projectName, http.StatusSeeOther)
}

func (s *Server) handleSubsystemDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectName := r.FormValue("projectname")
	subsystemName := r.FormValue("subsystem")

	if projectName == "" || subsystemName == "" {
		http.Error(w, "projectname and subsystem are required", http.StatusBadRequest)
		return
	}

	// Prevent deletion of the "public" subsystem.
	if subsystemName == "public" {
		http.Error(w, "Cannot delete the default 'public' subsystem", http.StatusForbidden)
		return
	}

	// Ensure at least one subsystem remains after deletion.
	var count int
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM %s.subsystem WHERE projectname = $1", s.cfg.DBSchema),
		projectName,
	).Scan(&count)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	if count <= 1 {
		http.Error(w, "Cannot delete the last subsystem. At least one must exist.", http.StatusForbidden)
		return
	}

	_, err = s.db.Exec(
		fmt.Sprintf("DELETE FROM %s.subsystem WHERE projectname = $1 AND subsystem = $2", s.cfg.DBSchema),
		projectName, subsystemName,
	)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
