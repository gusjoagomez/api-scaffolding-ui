package server

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"

	"api-scaffolding/internal/models"
)

func (s *Server) handleProjectsList(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	rows, err := s.db.Query(fmt.Sprintf("SELECT projectname, envdir, rootdir, maindir, modeldir, actiondir, testdir FROM %s.project ORDER BY projectname", s.cfg.DBSchema))
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ProjectName, &p.EnvDir, &p.RootDir, &p.MainDir, &p.ModelDir, &p.ActionDir, &p.TestDir); err != nil {
			renderError(w, err, http.StatusInternalServerError)
			return
		}
		projects = append(projects, p)
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/projects_list.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, projects); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleProjectNew(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/layout.html", "templates/project_form.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, &models.Project{}); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleProjectEdit(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	if projectName == "" {
		http.Error(w, "projectname is required", http.StatusBadRequest)
		return
	}

	row := s.db.QueryRow(fmt.Sprintf("SELECT projectname, envdir, rootdir, maindir, modeldir, actiondir, testdir FROM %s.project WHERE projectname = $1", s.cfg.DBSchema), projectName)
	var p models.Project
	if err := row.Scan(&p.ProjectName, &p.EnvDir, &p.RootDir, &p.MainDir, &p.ModelDir, &p.ActionDir, &p.TestDir); err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/project_form.html")
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, p); err != nil {
		renderError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleProjectSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectName := r.FormValue("projectname")
	envDir := r.FormValue("envdir")
	rootDir := r.FormValue("rootdir")
	mainDir := r.FormValue("maindir")
	modelDir := r.FormValue("modeldir")
	actionDir := r.FormValue("actiondir")
	testDir := r.FormValue("testdir")
	isNew := r.FormValue("is_new") == "true"

	var err error
	if isNew {
		_, err = s.db.Exec(fmt.Sprintf(`
			INSERT INTO %s.project (projectname, envdir, rootdir, maindir, modeldir, actiondir, testdir)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, s.cfg.DBSchema),
			projectName, envDir, rootDir, mainDir, modelDir, actionDir, testDir)
		if err == nil {
			// Auto-create the default "public" subsystem for every new project.
			s.db.Exec(fmt.Sprintf(`
				INSERT INTO %s.subsystem (projectname, subsystem, details)
				VALUES ($1, 'public', 'Default subsystem')
				ON CONFLICT DO NOTHING`, s.cfg.DBSchema),
				projectName)
		}
	} else {
		_, err = s.db.Exec(fmt.Sprintf(`
			UPDATE %s.project 
			SET envdir=$2, rootdir=$3, maindir=$4, modeldir=$5, actiondir=$6, testdir=$7
			WHERE projectname=$1`, s.cfg.DBSchema),
			projectName, envDir, rootDir, mainDir, modelDir, actionDir, testDir)
	}

	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
