package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type Scanner struct {
	db     *sql.DB
	driver string
}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) Connect(config *DatabaseConfig) error {
	var dsn string

	switch config.Driver {
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
			config.Host, config.Port, config.Username, config.Password,
			config.Database, config.SSLMode, config.Timezone)
		s.driver = "postgres"

	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=%s",
			config.Username, config.Password, config.Host, config.Port,
			config.Database, config.Timezone)
		s.driver = "mysql"

	default:
		return fmt.Errorf("unsupported database driver: %s", config.Driver)
	}

	db, err := sql.Open(config.Driver, dsn)
	if err != nil {
		return fmt.Errorf("error connecting to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("error pinging database: %v", err)
	}

	s.db = db
	return nil
}

func (s *Scanner) Disconnect() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *Scanner) GetTables(schema string, tableFilter []string) ([]Table, error) {
	switch s.driver {
	case "postgres":
		return s.getPostgresTables(schema, tableFilter)
	case "mysql":
		return s.getMySQLTables(schema, tableFilter)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", s.driver)
	}
}

func (s *Scanner) getPostgresTables(schema string, tableFilter []string) ([]Table, error) {
	var tables []Table

	// Obtener todas las tablas
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = $1 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := s.db.Query(query, schema)
	if err != nil {
		return nil, fmt.Errorf("error querying tables: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}

		// Filtrar si es necesario
		if len(tableFilter) > 0 && tableFilter[0] != "*" {
			found := false
			for _, filter := range tableFilter {
				if strings.EqualFold(filter, tableName) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Obtener columnas
		columns, err := s.getPostgresColumns(schema, tableName)
		if err != nil {
			return nil, err
		}

		// Obtener claves primarias
		primaryKeys, err := s.getPostgresPrimaryKeys(schema, tableName)
		if err != nil {
			return nil, err
		}

		// Obtener claves foráneas
		foreignKeys, err := s.getPostgresForeignKeys(schema, tableName)
		if err != nil {
			return nil, err
		}

		table := Table{
			Name:        tableName,
			Schema:      schema,
			Columns:     columns,
			PrimaryKeys: primaryKeys,
			ForeignKeys: foreignKeys,
		}

		tables = append(tables, table)
	}

	return tables, nil
}

func (s *Scanner) getMySQLTables(schema string, tableFilter []string) ([]Table, error) {
	var tables []Table

	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = ?
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := s.db.Query(query, schema)
	if err != nil {
		return nil, fmt.Errorf("error querying tables: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}

		// Filtrar si es necesario
		if len(tableFilter) > 0 && tableFilter[0] != "*" {
			found := false
			for _, filter := range tableFilter {
				if strings.EqualFold(filter, tableName) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Obtener columnas
		columns, err := s.getMySQLColumns(schema, tableName)
		if err != nil {
			return nil, err
		}

		// Obtener claves primarias
		primaryKeys, err := s.getMySQLPrimaryKeys(schema, tableName)
		if err != nil {
			return nil, err
		}

		// Obtener claves foráneas
		foreignKeys, err := s.getMySQLForeignKeys(schema, tableName)
		if err != nil {
			return nil, err
		}

		table := Table{
			Name:        tableName,
			Schema:      schema,
			Columns:     columns,
			PrimaryKeys: primaryKeys,
			ForeignKeys: foreignKeys,
		}

		tables = append(tables, table)
	}

	return tables, nil
}

func (s *Scanner) getPostgresColumns(schema, tableName string) ([]Column, error) {
	query := `
		SELECT 
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			c.character_maximum_length,
			pgd.description as column_comment
		FROM information_schema.columns c
		LEFT JOIN pg_catalog.pg_statio_all_tables st 
			ON c.table_schema = st.schemaname AND c.table_name = st.relname
		LEFT JOIN pg_catalog.pg_description pgd 
			ON pgd.objoid = st.relid AND pgd.objsubid = c.ordinal_position
		WHERE c.table_schema = $1 
			AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	rows, err := s.db.Query(query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var isNullable string
		var defaultValue, maxLength, comment sql.NullString

		if err := rows.Scan(
			&col.Name,
			&col.DataType,
			&isNullable,
			&defaultValue,
			&maxLength,
			&comment,
		); err != nil {
			return nil, err
		}

		col.IsNullable = (isNullable == "YES")

		if defaultValue.Valid {
			val := defaultValue.String
			col.DefaultValue = &val
		}

		if maxLength.Valid && maxLength.String != "" {
			var length int
			fmt.Sscanf(maxLength.String, "%d", &length)
			col.MaxLength = &length
		}

		if comment.Valid {
			col.Comment = comment.String
		}

		columns = append(columns, col)
	}

	return columns, nil
}

func (s *Scanner) getMySQLColumns(schema, tableName string) ([]Column, error) {
	query := `
		SELECT 
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length,
			column_comment
		FROM information_schema.columns
		WHERE table_schema = ?
			AND table_name = ?
		ORDER BY ordinal_position
	`

	rows, err := s.db.Query(query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var isNullable string
		var defaultValue, maxLength sql.NullString
		var comment string

		if err := rows.Scan(
			&col.Name,
			&col.DataType,
			&isNullable,
			&defaultValue,
			&maxLength,
			&comment,
		); err != nil {
			return nil, err
		}

		col.IsNullable = (isNullable == "YES")

		if defaultValue.Valid {
			val := defaultValue.String
			col.DefaultValue = &val
		}

		if maxLength.Valid && maxLength.String != "" {
			var length int
			fmt.Sscanf(maxLength.String, "%d", &length)
			col.MaxLength = &length
		}

		col.Comment = comment

		columns = append(columns, col)
	}

	return columns, nil
}

func (s *Scanner) getPostgresPrimaryKeys(schema, tableName string) ([]string, error) {
	query := `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = $1::regclass
			AND i.indisprimary
	`

	rows, err := s.db.Query(query, fmt.Sprintf("%s.%s", schema, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var primaryKeys []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		primaryKeys = append(primaryKeys, columnName)
	}

	return primaryKeys, nil
}

func (s *Scanner) getMySQLPrimaryKeys(schema, tableName string) ([]string, error) {
	query := `
		SELECT column_name
		FROM information_schema.key_column_usage
		WHERE constraint_name = 'PRIMARY'
			AND table_schema = ?
			AND table_name = ?
		ORDER BY ordinal_position
	`

	rows, err := s.db.Query(query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var primaryKeys []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		primaryKeys = append(primaryKeys, columnName)
	}

	return primaryKeys, nil
}

func (s *Scanner) GetForeignKeys(schema, tableName string) ([]ForeignKey, error) {
	switch s.driver {
	case "postgres":
		return s.getPostgresForeignKeys(schema, tableName)
	case "mysql":
		return s.getMySQLForeignKeys(schema, tableName)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", s.driver)
	}
}

func (s *Scanner) getPostgresForeignKeys(schema, tableName string) ([]ForeignKey, error) {
	query := `
		SELECT
			kcu.column_name,
			ccu.table_name AS referenced_table,
			ccu.column_name AS referenced_column,
			tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1
			AND tc.table_name = $2
	`

	rows, err := s.db.Query(query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foreignKeys []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(
			&fk.ColumnName,
			&fk.ReferencedTable,
			&fk.ReferencedColumn,
			&fk.ConstraintName,
		); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, nil
}

func (s *Scanner) getMySQLForeignKeys(schema, tableName string) ([]ForeignKey, error) {
	query := `
		SELECT
			column_name,
			referenced_table_name,
			referenced_column_name,
			constraint_name
		FROM information_schema.key_column_usage
		WHERE constraint_schema = ?
			AND table_name = ?
			AND referenced_table_name IS NOT NULL
	`

	rows, err := s.db.Query(query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foreignKeys []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(
			&fk.ColumnName,
			&fk.ReferencedTable,
			&fk.ReferencedColumn,
			&fk.ConstraintName,
		); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, nil
}
