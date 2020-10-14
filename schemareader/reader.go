package schemareader

import (
	"database/sql"
	"log"
	"strings"
)

func readTableNames(db *sql.DB) []string {
	return []string{
		"rhnchannel",
		"rhnchannelarch",
		"rhnchannelerrata",
		"rhnchannelfamily",
		"rhnchannelfamilymembers",
		"rhnerrata",
		"rhnchannelproduct",
		"suseproducts",
		"rhnproductname",

		// dictionaries
		"rhnpackagearch",
		"rhnerrataseverity",
		"rhnchecksumtype",
		"rhnarchtype",

		// to be added later: rhnpackage, rhnchannelpackage, rhnerratapackage
	}
}

func readColumnNames(db *sql.DB, tableName string) []string {
	sql := `SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position;`

	rows, err := db.Query(sql, tableName)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var columnName string
		err := rows.Scan(&columnName)
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, columnName)
	}

	return result
}

func readPKColumnNames(db *sql.DB, tableName string) []string {
	// https://wiki.postgresql.org/wiki/Retrieve_primary_key_columns
	sql := `SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid
			AND a.attnum = ANY(i.indkey)
		WHERE  i.indrelid = $1::regclass
		AND    i.indisprimary;`

	rows, err := db.Query(sql, tableName)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var columnName string
		err := rows.Scan(&columnName)
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, columnName)
	}

	return result
}

func readUniqueIndexNames(db *sql.DB, tableName string) []string {
	sql := `SELECT DISTINCT indexrelid::regclass
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid
			AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = $1::regclass
		AND i.indisunique AND NOT i.indisprimary;`

	rows, err := db.Query(sql, tableName)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, name)
	}

	return result
}

func readIndexColumns(db *sql.DB, indexName string) []string {
	sql := `SELECT DISTINCT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid
			AND a.attnum = ANY(i.indkey)
		WHERE indexrelid::regclass = $1::regclass;`

	rows, err := db.Query(sql, indexName)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, name)
	}

	return result
}

func readReferenceConstraintNames(db *sql.DB, tableName string) []string {
	sql := `SELECT DISTINCT tc.constraint_name
		FROM information_schema.table_constraints AS tc
			JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
				AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1;`

	rows, err := db.Query(sql, tableName)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, name)
	}

	return result
}

func readReferencedTable(db *sql.DB, tableName string, referenceConstraintName string) string {
	sql := `SELECT DISTINCT ccu.table_name
	FROM information_schema.constraint_column_usage AS ccu
	WHERE ccu.constraint_name = $1;`

	rows, err := db.Query(sql, referenceConstraintName)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var name string
	rows.Next()
	rows.Scan(&name)

	return name
}

func readReferenceConstraints(db *sql.DB, tableName string, referenceConstraintName string) map[string]string {
	sql := `SELECT DISTINCT kcu.column_name, ccu.column_name AS foreign_column_name
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
			AND tc.table_name = kcu.table_name
		JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
			AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
			AND tc.constraint_name = $2;`

	rows, err := db.Query(sql, tableName, referenceConstraintName)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var columnName string
		var foreignColumnName string
		err := rows.Scan(&columnName, &foreignColumnName)
		if err != nil {
			log.Fatal(err)
		}
		result[columnName] = foreignColumnName
	}

	return result
}

func findIndex(indexes map[string]UniqueIndex, columnName string) string {
	for name, index := range indexes {
		for _, column := range index.Columns {
			if strings.Compare(column, columnName) == 0 {
				return name
			}
		}
	}
	return ""
}

func readPKSequence(db *sql.DB, tableName string) string {
	sql := `WITH sequences AS (
		SELECT sequence_name
			FROM information_schema.sequences
			WHERE sequence_schema = 'public'
		),
		id_constraints AS (
			SELECT
				tc.constraint_name,
				tc.table_name,
				kcu.column_name
			FROM
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
					ON tc.constraint_name = kcu.constraint_name
			WHERE tc.constraint_schema = 'public'
				AND constraint_type = 'PRIMARY KEY'
				AND kcu.ordinal_position = 1
				AND column_name = 'id'
				AND tc.table_name = $1
		)
		SELECT sequence_name
			FROM id_constraints
			JOIN sequences
				ON replace(regexp_replace(constraint_name, '(_id)?_pk(ey)?', ''), '_', '') = replace(regexp_replace(sequence_name, '(_id)?_seq', ''), '_', '')`

	rows, err := db.Query(sql, tableName)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var name string
	rows.Next()
	rows.Scan(&name)

	return name
}

// ReadTables inspects the DB and returns a list of tables
func ReadTables(db *sql.DB) []Table {
	tableNames := readTableNames(db)

	result := make([]Table, 0)
	for _, tableName := range tableNames {
		columns := readColumnNames(db, tableName)

		pkColumns := readPKColumnNames(db, tableName)
		pkColumnMap := make(map[string]bool)
		for _, column := range pkColumns {
			pkColumnMap[column] = true
		}

		pkSequence := readPKSequence(db, tableName)

		indexNames := readUniqueIndexNames(db, tableName)
		indexes := make(map[string]UniqueIndex)
		for _, indexName := range indexNames {
			indexColumns := readIndexColumns(db, indexName)
			indexes[indexName] = UniqueIndex{Name: indexName, Columns: indexColumns}
		}

		mainUniqueIndexName := ""
		if len(indexNames) == 1 {
			mainUniqueIndexName = indexNames[0]
		} else if len(indexNames) > 1 {
			mainUniqueIndexName = findIndex(indexes, "label")
			if len(mainUniqueIndexName) == 0 {
				mainUniqueIndexName = findIndex(indexes, "name")
				if len(mainUniqueIndexName) == 0 {
					mainUniqueIndexName = indexNames[0]
				}
			}
		}

		constraintNames := readReferenceConstraintNames(db, tableName)
		references := make([]Reference, 0)
		for _, constraintName := range constraintNames {
			columnMap := readReferenceConstraints(db, tableName, constraintName)
			referencedTable := readReferencedTable(db, tableName, constraintName)
			references = append(references, Reference{TableName: referencedTable, ColumnMapping: columnMap})
		}

		result = append(result, Table{Name: tableName, Columns: columns, PKColumns: pkColumnMap, PKSequence: pkSequence, UniqueIndexes: indexes, MainUniqueIndexName: mainUniqueIndexName, References: references})
	}

	// main indexes might not cover columns which are populated with sequences
	for i, table := range result {
		if len(table.MainUniqueIndexName) > 0 {
			for _, column := range table.UniqueIndexes[table.MainUniqueIndexName].Columns {
				for _, reference := range table.References {
					referencedColumn := reference.ColumnMapping[column]
					if strings.Compare(referencedColumn, "id") == 0 {
						for _, referencedTable := range result {
							if strings.Compare(referencedTable.Name, reference.TableName) == 0 {
								if strings.Compare(referencedTable.PKSequence, "") != 0 {
									result[i].MainUniqueIndexName = ""
								}
							}
						}
					}
				}
			}
		}
	}

	return result
}