package server

import (
	"fmt"
	"html/template"
	"net/http"

	"api-scaffolding/internal/models"
)

func (s *Server) handleTablesList(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	connName := r.URL.Query().Get("connection")
	if projectName == "" || connName == "" {
		http.Error(w, "projectname and connection are required", http.StatusBadRequest)
		return
	}

	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT projectname, connection, dbname, dbschema, tablename, entityname, detail 
		FROM %s.tables 
		WHERE projectname = $1 AND connection = $2 ORDER BY tablename`, s.cfg.DBSchema), projectName, connName)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []models.Table
	for rows.Next() {
		var t models.Table
		if err := rows.Scan(
			&t.ProjectName, &t.Connection, &t.DbName, &t.DbSchema, &t.TableName, &t.EntityName, &t.Detail,
		); err != nil {
			renderError(w, err, http.StatusInternalServerError)
			return
		}
		tables = append(tables, t)
	}

	// Fetch subsystems for this project
	subRows, err := s.db.Query(
		fmt.Sprintf("SELECT projectname, subsystem, details FROM %s.subsystem WHERE projectname = $1 ORDER BY subsystem", s.cfg.DBSchema),
		projectName,
	)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	defer subRows.Close()

	var subsystems []models.Subsystem
	for subRows.Next() {
		var sub models.Subsystem
		if err := subRows.Scan(&sub.ProjectName, &sub.Subsystem, &sub.Details); err != nil {
			renderError(w, err, http.StatusInternalServerError)
			return
		}
		subsystems = append(subsystems, sub)
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/tables_list.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	data := struct {
		ProjectName string
		Connection  string
		Tables      []models.Table
		Subsystems  []models.Subsystem
	}{
		ProjectName: projectName,
		Connection:  connName,
		Tables:      tables,
		Subsystems:  subsystems,
	}

	if err := tmpl.Execute(w, data); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}
