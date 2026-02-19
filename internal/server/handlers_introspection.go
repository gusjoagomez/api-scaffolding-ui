package server

import (
	"database/sql"
	"fmt"
	"net/http"

	"api-scaffolding/internal/database"
)

func (s *Server) handleGetInfoTables(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("projectname")
	connName := r.URL.Query().Get("connection")
	if projectName == "" || connName == "" {
		http.Error(w, "projectname and connection are required", http.StatusBadRequest)
		return
	}

	// 1. Get Connection Details
	var c database.DatabaseConfig

	row := s.db.QueryRow(fmt.Sprintf(`
		SELECT dbtype, dbhost, dbport, dbuser, dbpass, dbname, dbschema, dbsslmode, dbtimezone 
		FROM %s.dbconn 
		WHERE projectname = $1 AND connection = $2`, s.cfg.DBSchema), projectName, connName)

	var dbType, dbHost, dbPort, dbUser, dbPass, dbNameNS, dbSchemaNS, dbSslMode, dbTimezone sql.NullString
	if err := row.Scan(&dbType, &dbHost, &dbPort, &dbUser, &dbPass, &dbNameNS, &dbSchemaNS, &dbSslMode, &dbTimezone); err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	c.Driver = dbType.String
	c.Host = dbHost.String
	c.Port = dbPort.String
	c.Username = dbUser.String
	c.Password = dbPass.String
	c.Database = dbNameNS.String
	c.SSLMode = dbSslMode.String
	c.Timezone = dbTimezone.String

	// 2. Connect to Target DB
	scanner := database.NewScanner()
	if err := scanner.Connect(&c); err != nil {
		renderError(w, fmt.Errorf("failed to connect to target db: %v", err), http.StatusInternalServerError)
		return
	}
	defer scanner.Disconnect()

	// 3. Get Tables
	// Use the scanned schema
	targetSchema := dbSchemaNS.String
	if targetSchema == "" {
		targetSchema = "public" // Default
	}

	tables, err := scanner.GetTables(targetSchema, []string{"*"})
	if err != nil {
		renderError(w, fmt.Errorf("failed to get tables: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Transaction to save metadata
	tx, err := s.db.Begin()
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Delete existing for this connection to avoid dupes/stale data
	_, err = tx.Exec(fmt.Sprintf("DELETE FROM %s.tables WHERE projectname=$1 AND connection=$2", s.cfg.DBSchema), projectName, connName)
	if err != nil {
		renderError(w, fmt.Errorf("failed to clear tables: %v", err), http.StatusInternalServerError)
		return
	}
	_, err = tx.Exec(fmt.Sprintf("DELETE FROM %s.tablesfields WHERE projectname=$1 AND connection=$2", s.cfg.DBSchema), projectName, connName)
	if err != nil {
		renderError(w, fmt.Errorf("failed to clear tablesfields: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete rels
	// Note: using dbNameNS.String not dbName
	_, err = tx.Exec(fmt.Sprintf("DELETE FROM %s.tablesrels WHERE connection=$1 AND dbname=$2 AND dbschema=$3", s.cfg.DBSchema), connName, dbNameNS.String, targetSchema)
	if err != nil {
		renderError(w, fmt.Errorf("failed to clear tablesrels: %v", err), http.StatusInternalServerError)
		return
	}

	for _, t := range tables {
		// Insert Table
		_, err := tx.Exec(fmt.Sprintf(`
			INSERT INTO %s.tables (projectname, connection, dbname, dbschema, tablename, entityname, detail)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, s.cfg.DBSchema),
			projectName, connName, dbNameNS.String, targetSchema, t.Name, t.Name, t.Comment)
		if err != nil {
			renderError(w, fmt.Errorf("failed to insert table %s: %v", t.Name, err), http.StatusInternalServerError)
			return
		}

		// Insert Fields
		for i, col := range t.Columns {
			isNull := "0"
			if col.IsNullable {
				isNull = "1"
			}
			pk := ""
			// Check if PK
			isPk := false
			for _, p := range t.PrimaryKeys {
				if p == col.Name {
					isPk = true
					break
				}
			}
			if isPk {
				pk = "1"
			}

			// Check FK
			var fTable, fKey sql.NullString
			for _, fk := range t.ForeignKeys {
				if fk.ColumnName == col.Name {
					fTable = sql.NullString{String: fk.ReferencedTable, Valid: true}
					fKey = sql.NullString{String: fk.ReferencedColumn, Valid: true}
					break
				}
			}

			// Default Value
			var defVal sql.NullString
			if col.DefaultValue != nil {
				defVal = sql.NullString{String: *col.DefaultValue, Valid: true}
			}

			valLength := ""
			if col.MaxLength != nil {
				valLength = fmt.Sprintf("%d", *col.MaxLength)
			}

			// Defaults
			inList := 1
			inCrud := 1

			_, err := tx.Exec(fmt.Sprintf(`
				INSERT INTO %s.tablesfields (
					projectname, connection, dbname, dbschema, tablename, fieldname, 
					typename, defaultvalue, is_null, pk, unq, 
					ftable, fkey, label, labelhelp, orderlist, 
					inlist, incrud, val_length, detail
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`, s.cfg.DBSchema),
				projectName, connName, dbNameNS.String, targetSchema, t.Name, col.Name,
				col.DataType, defVal, isNull, pk, "", // unq set empty
				fTable, fKey, col.Name, col.Name, i+1,
				inList, inCrud, valLength, col.Comment,
			)
			if err != nil {
				renderError(w, fmt.Errorf("failed to insert field %s.%s: %v", t.Name, col.Name, err), http.StatusInternalServerError)
				return
			}
		}

		// Insert Rels
		for _, fk := range t.ForeignKeys {
			// typeRel defaults to 1:N
			typeRel := "1:N"

			_, err := tx.Exec(fmt.Sprintf(`
				INSERT INTO %s.tablesrels (
					connection, dbname, dbschema, tablename, 
					typerel, fname, tabler, fnamepk, is_null
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, s.cfg.DBSchema),
				connName, dbNameNS.String, targetSchema, t.Name,
				typeRel, fk.ColumnName, fk.ReferencedTable, fk.ReferencedColumn, 1,
			)
			if err != nil {
				renderError(w, fmt.Errorf("failed to insert rel %s->%s: %v", t.Name, fk.ReferencedTable, err), http.StatusInternalServerError)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	// Redirect back to connection list
	http.Redirect(w, r, fmt.Sprintf("/connections/tables?projectname=%s&connection=%s", projectName, connName), http.StatusSeeOther)
}
