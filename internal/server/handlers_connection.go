package server

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"

	"api-scaffolding/internal/models"
)

func (s *Server) handleConnectionsList(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	if projectName == "" {
		http.Error(w, "projectname is required", http.StatusBadRequest)
		return
	}

	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT projectname, connection, dbtype, dbhost, dbport, dbuser, dbpass, dbname, dbschema, dbsslmode, dbtimezone 
		FROM %s.dbconn 
		WHERE projectname = $1 ORDER BY connection`, s.cfg.DBSchema), projectName)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var conns []models.DbConn
	for rows.Next() {
		var c models.DbConn
		if err := rows.Scan(
			&c.ProjectName, &c.Connection, &c.DbType, &c.DbHost, &c.DbPort, &c.DbUser, &c.DbPass,
			&c.DbName, &c.DbSchema, &c.DbSSLMode, &c.DbTimezone,
		); err != nil {
			renderError(w, err, http.StatusInternalServerError)
			return
		}
		conns = append(conns, c)
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/connections_list.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	data := struct {
		ProjectName string
		Connections []models.DbConn
	}{
		ProjectName: projectName,
		Connections: conns,
	}

	if err := tmpl.Execute(w, data); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleConnectionNew(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	if projectName == "" {
		http.Error(w, "projectname is required", http.StatusBadRequest)
		return
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/connection_form.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	data := struct {
		ProjectName string
		Connection  *models.DbConn
	}{
		ProjectName: projectName,
		Connection:  nil, // New connection
	}

	if err := tmpl.Execute(w, data); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleConnectionEdit(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	connName := r.URL.Query().Get("connection")
	if projectName == "" || connName == "" {
		http.Error(w, "projectname and connection are required", http.StatusBadRequest)
		return
	}

	row := s.db.QueryRow(fmt.Sprintf(`
		SELECT projectname, connection, dbtype, dbhost, dbport, dbuser, dbpass, dbname, dbschema, dbsslmode, dbtimezone 
		FROM %s.dbconn 
		WHERE projectname = $1 AND connection = $2`, s.cfg.DBSchema), projectName, connName)

	var c models.DbConn
	if err := row.Scan(
		&c.ProjectName, &c.Connection, &c.DbType, &c.DbHost, &c.DbPort, &c.DbUser, &c.DbPass,
		&c.DbName, &c.DbSchema, &c.DbSSLMode, &c.DbTimezone,
	); err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/connection_form.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	data := struct {
		ProjectName string
		Connection  *models.DbConn
	}{
		ProjectName: projectName,
		Connection:  &c,
	}

	if err := tmpl.Execute(w, data); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleConnectionSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectName := r.FormValue("projectname")
	connection := r.FormValue("connection")
	dbType := r.FormValue("dbtype")
	dbHost := r.FormValue("dbhost")
	dbPort := r.FormValue("dbport")
	dbUser := r.FormValue("dbuser")
	dbPass := r.FormValue("dbpass")
	dbName := r.FormValue("dbname")
	dbSchema := r.FormValue("dbschema")
	dbSSLMode := r.FormValue("dbsslmode")
	dbTimezone := r.FormValue("dbtimezone")
	isNew := r.FormValue("is_new") == "true"

	var err error
	if isNew {
		_, err = s.db.Exec(fmt.Sprintf(`
			INSERT INTO %s.dbconn (projectname, connection, dbtype, dbhost, dbport, dbuser, dbpass, dbname, dbschema, dbsslmode, dbtimezone)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, s.cfg.DBSchema),
			projectName, connection, dbType, dbHost, dbPort, dbUser, dbPass, dbName, dbSchema, dbSSLMode, dbTimezone)
	} else {
		_, err = s.db.Exec(fmt.Sprintf(`
			UPDATE %s.dbconn 
			SET dbtype=$3, dbhost=$4, dbport=$5, dbuser=$6, dbpass=$7, dbname=$8, dbschema=$9, dbsslmode=$10, dbtimezone=$11
			WHERE projectname=$1 AND connection=$2`, s.cfg.DBSchema),
			projectName, connection, dbType, dbHost, dbPort, dbUser, dbPass, dbName, dbSchema, dbSSLMode, dbTimezone)
	}

	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/connections?projectname=%s", projectName), http.StatusSeeOther)
}
