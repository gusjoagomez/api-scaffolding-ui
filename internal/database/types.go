package database

type Column struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
	IsForeignKey bool
	DefaultValue *string
	MaxLength    *int
	Comment      string
}

type Table struct {
	Name        string
	Schema      string
	Columns     []Column
	PrimaryKeys []string
	ForeignKeys []ForeignKey
	Comment     string
}

type ForeignKey struct {
	ColumnName       string
	ReferencedTable  string
	ReferencedColumn string
	ConstraintName   string
}

type DatabaseScanner interface {
	Connect(config *DatabaseConfig) error
	Disconnect()
	GetTables(schema string, tableFilter []string) ([]Table, error)
	GetForeignKeys(schema, tableName string) ([]ForeignKey, error)
}

type DatabaseConfig struct {
	Driver   string
	Host     string
	Port     string
	Username string
	Password string
	Database string
	SSLMode  string
	Timezone string
}
