package models

import (
	"database/sql"
)

type Project struct {
	ProjectName string         `json:"projectname"`
	EnvDir      sql.NullString `json:"envdir"`
	RootDir     sql.NullString `json:"rootdir"`
	MainDir     sql.NullString `json:"maindir"`
	ModelDir    sql.NullString `json:"modeldir"`
	ActionDir   sql.NullString `json:"actiondir"`
	TestDir     sql.NullString `json:"testdir"`
}

type DbConn struct {
	ProjectName string         `json:"projectname"`
	Connection  string         `json:"connection"` // Nullable in DDL but PK so handled as string, though DDL says varchar(30) NULL, usually PK cannot be null. User DDL says PRIMARY KEY (projectname, connection)
	DbType      sql.NullString `json:"dbtype"`
	DbHost      sql.NullString `json:"dbhost"`
	DbPort      sql.NullString `json:"dbport"`
	DbUser      sql.NullString `json:"dbuser"`
	DbPass      sql.NullString `json:"dbpass"`
	DbName      sql.NullString `json:"dbname"`
	DbSchema    sql.NullString `json:"dbschema"`
	DbSSLMode   sql.NullString `json:"dbsslmode"`
	DbTimezone  sql.NullString `json:"dbtimezone"`
}

type Table struct {
	ProjectName string         `json:"projectname"`
	Connection  sql.NullString `json:"connection"`
	DbName      sql.NullString `json:"dbname"`
	DbSchema    string         `json:"dbschema"`
	TableName   string         `json:"tablename"`
	EntityName  sql.NullString `json:"entityname"`
	Detail      sql.NullString `json:"detail"`
}

type TableField struct {
	ProjectName  string         `json:"projectname"`
	Connection   sql.NullString `json:"connection"`
	DbName       sql.NullString `json:"dbname"`
	DbSchema     string         `json:"dbschema"`
	TableName    string         `json:"tablename"`
	FieldName    string         `json:"fieldname"`
	TypeName     sql.NullString `json:"typename"`
	DefaultValue sql.NullString `json:"defaultvalue"`
	IsNull       sql.NullString `json:"is_null"`
	Pk           sql.NullString `json:"pk"`
	Unq          sql.NullString `json:"unq"`
	FTable       sql.NullString `json:"ftable"`
	FKey         sql.NullString `json:"fkey"`
	Label        sql.NullString `json:"label"`
	LabelHelp    sql.NullString `json:"labelhelp"`
	OrderList    sql.NullInt32  `json:"orderlist"`
	InList       int16          `json:"inlist"`
	InCrud       int16          `json:"incrud"`
	ValLength    sql.NullString `json:"val_length"`
	Auditoria    sql.NullBool   `json:"auditoria"`
	Detail       sql.NullString `json:"detail"`
}

type FileTemplate struct {
	ID        int            `json:"id"`
	Version   sql.NullString `json:"version"`
	GroupType sql.NullString `json:"grouptype"`
	Category  sql.NullString `json:"category"`
	Name      sql.NullString `json:"name"`
	Path      sql.NullString `json:"path"`
	File      sql.NullString `json:"file"`
	Template  sql.NullString `json:"template"`
	Source    sql.NullString `json:"source"`
	OrderList sql.NullInt32  `json:"orderlist"`
	Visible   sql.NullInt32  `json:"visible"`
	TypeFile  sql.NullString `json:"typefile"`
}

type Subsystem struct {
	ProjectName string         `json:"projectname"`
	Subsystem   string         `json:"subsystem"`
	Details     sql.NullString `json:"details"`
}

type TableRel struct {
	Connection string         `json:"connection"`
	DbName     sql.NullString `json:"dbname"`
	DbSchema   string         `json:"dbschema"`
	TableName  string         `json:"tablename"`
	TypeRel    string         `json:"typerel"`
	FName      string         `json:"fname"`
	TableR     string         `json:"tabler"`
	FNamePk    string         `json:"fnamepk"`
	IsNull     sql.NullInt32  `json:"is_null"`
}
