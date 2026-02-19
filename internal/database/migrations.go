package database

import (
	"database/sql"
	"fmt"
	"log"
)

func InitSchema(db *sql.DB, schema string) error {
	queries := []string{
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.project (
			projectname varchar(50) NOT NULL,
			envdir varchar(300) NULL,
			rootdir varchar(300) NULL,
			maindir varchar(300) NULL,
			modeldir varchar(300) NULL,
			actiondir varchar(300) NULL,
			testdir varchar(300) NULL,
			CONSTRAINT xch_tmpprojectres_pkey PRIMARY KEY (projectname)
		);`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.dbconn (
			projectname varchar(50) NOT NULL,
			connection varchar(30) NOT NULL,
			dbtype varchar(20) NULL,
			dbhost varchar(100) NULL,
			dbport varchar(5) NULL,
			dbuser varchar(50) NULL,
			dbpass varchar(100) NULL,
			dbname varchar(50) NULL,
			dbschema varchar(50) NULL,
			dbsslmode varchar(10) NULL,
			dbtimezone varchar(50) NULL default 'UTC',
			CONSTRAINT xch_dbconn_pkey PRIMARY KEY (projectname, connection)
		);`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tables (
			projectname varchar(50) NOT NULL,
			connection varchar(30) NULL,
			dbname varchar(50) NULL,
			dbschema character varying(50) NOT NULL,
			tablename character varying(50) NOT NULL,
			entityname character varying(50),
			detail character varying(1014) DEFAULT '-'::character varying
		);`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tablesfields (
			projectname varchar(50) NOT NULL,
			connection varchar(30) NULL,
			dbname varchar(50) NULL,
			dbschema character varying(50) NOT NULL,
			tablename character varying(50) NOT NULL,
			fieldname character varying(50) NOT NULL,
			typename character varying(50),
			defaultvalue varchar(100),
			is_null character varying(3),
			pk character varying(3),
			unq character varying(3),
			ftable character varying(45),
			fkey character varying(45),
			label character varying(100),
			labelhelp character varying(100),
			orderlist integer DEFAULT 0,
			inlist smallint DEFAULT 1 NOT NULL,
			incrud smallint DEFAULT 1 NOT NULL,
			val_length character varying(45),
			auditoria boolean,
			detail character varying(1024)
		);`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.tablesrels (
			"connection" varchar(30) NULL,
			dbname varchar(50) NULL,
			dbschema varchar(50) NOT NULL,
			tablename varchar(50) NOT NULL,
			typerel varchar(3) NOT NULL,
			fname varchar(100) NOT NULL,
			tabler varchar(50) NOT NULL,
			fnamepk varchar(100) NOT NULL,
			is_null int4 DEFAULT 1 NULL
		);`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.file_templates (
			id         serial PRIMARY KEY,
			version    varchar(50),
			grouptype  varchar(50),
			category   varchar(100),
			name       varchar(100),
			path       varchar(500),
			file       varchar(200),
			template   varchar(300),
			source     varchar(300),
			orderlist  integer DEFAULT 0,
			visible    smallint DEFAULT 1,
			typefile   varchar(5)
		);`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.subsystem (
			projectname varchar(50) NOT NULL,
			subsystem varchar(15) NOT NULL,
			details varchar(1024) NULL,
			CONSTRAINT subsystem_pkey PRIMARY KEY (projectname, subsystem)
		);`, schema),
		// Seed default file_templates (idempotent)
		fmt.Sprintf(`INSERT INTO %s.file_templates (version,grouptype,category,name,path,file,template,source,orderlist,visible,typefile) VALUES
			('api-loader','crud','list-entity',   'List',         '[rootprj]/[entity]/','[entity]_list.yaml',         'templatesgen/entidad_list.tpl',         '',1,1,'M'),
			('api-loader','crud','get-entity',    'Get',          '[rootprj]/[entity]/','[entity]_get.yaml',          'templatesgen/entidad_get.tpl',          '',2,1,'M'),
			('api-loader','crud','new-entity',    'New',          '[rootprj]/[entity]/','[entity]_new.yaml',          'templatesgen/entidad_new.tpl',          '',3,1,'M'),
			('api-loader','crud','update-entity', 'Update',       '[rootprj]/[entity]/','[entity]_update.yaml',       'templatesgen/entidad_update.tpl',       '',4,1,'M'),
			('api-loader','crud','delete-entity', 'Delete',       '[rootprj]/[entity]/','[entity]_delete.yaml',       'templatesgen/entidad_delete.tpl',       '',5,1,'M'),
			('api-loader','crud','report-entity', 'Report',       '[rootprj]/[entity]/','[entity]_report.yaml',       'templatesgen/entidad_report.tpl',       '',6,1,'M'),
			('api-loader','crud','upload-local',  'Upload-local', '[rootprj]/[entity]/','[entity]_upload_local.yaml', 'templatesgen/entidad_upload_local.tpl', '',7,1,'M'),
			('api-loader','crud','upload-s3',     'Upload-s3',    '[rootprj]/[entity]/','[entity]_upload_s3.yaml',    'templatesgen/entidad_upload_s3.tpl',    '',8,1,'M'),
			('api-loader','crud','auth-entity',   'Auth',         '[rootprj]/[entity]/','[entity]_auth.yaml',         'templatesgen/entidad_auth.tpl',         '',9,1,'M'),
			('api-loader','crud','custom-entity', 'Custom',       '[rootprj]/[entity]/','[entity]_custom.yaml',       'templatesgen/entidad_custom.tpl',       '',10,1,'M'),
			('api-loader','crud','demo-plugin',   'Demo Plugin',  '[rootprj]/[entity]/','[entity]_demo_plugin.yaml',  'templatesgen/entidad_demo_plugin.tpl',  '',11,1,'M')
		ON CONFLICT DO NOTHING;`, schema),
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			log.Printf("Error executing schema init query: %v", err)
			// Don't fail, maybe it exists or permission error
			// return err
		}
	}
	return nil
}
