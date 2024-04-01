package builder_query // Name of your package

import (
	"database/sql" // VariantHistory represents a single row from the query
	"fmt"
	"strings"
)

type VariantHistory struct {
	VariantID     string `db:"variant_id"`
	VariantKey    []byte `db:"variant_key"`
	ExperimentID  string `db:"experiment_id"`
	ExperimentKey []byte `db:"experiment_key"`
	Tanggal       string `db:"tanggal"`
	Impression    uint   `db:"impression"`
	CTA           uint   `db:"cta"`
	Lead          uint   `db:"lead"`
	Mql           uint   `db:"mql"`
	Prospek       uint   `db:"prospek"`
	Purchase      uint   `db:"purchase"`
}

type Rotator struct {
	PageID     string
	PageKey    []byte
	RotatorID  string
	RotatorKey []byte
}

// GetVariantHistoryByExperimentKey takes a database connection, table name, and experiment key in hex format and returns an array of results
func GetVariantHistoryByExperimentKey(db *sql.DB, tableName string, experimentKeyHex string) ([]VariantHistory, error) {
	query := `SELECT vh.variant_id,sum(vh.impression) as impression,sum(vh.cta) as cta,sum(vh.lead) as lead,sum(vh.mql) as mql,sum(vh.prospek) as prospek,sum(vh.purchase) as purchase  FROM ` + tableName + ` as vh WHERE experiment_key = UNHEX(?)  GROUP BY variant_key ;`
	rows, err := db.Query(query, experimentKeyHex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []VariantHistory
	for rows.Next() {
		var result VariantHistory
		err := rows.Scan(&result.VariantID, &result.Impression, &result.CTA, &result.Lead, &result.Mql, &result.Prospek, &result.Purchase)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return results, nil
}

type Experiment struct {
	ExperimentID  string
	ExperimentKey string // Changed to string for hex representation
	AdsName       string
	RotatorID     string
	RotatorKey    string // Changed to string for hex representation
	Status        int
}

func SelectFromZRotatorExperiment(db *sql.DB, experimentKey string) (Experiment, error) {
	var row Experiment

	// Prepare the SQL query with HEX function on experiment_key and rotator_key columns
	query := "SELECT experiment_id, HEX(experiment_key), ads_name, rotator_id, HEX(rotator_key), status FROM z_rotator_experiment WHERE experiment_key = UNHEX(?) LIMIT 1"

	// Execute the query
	err := db.QueryRow(query, experimentKey).Scan(&row.ExperimentID, &row.ExperimentKey, &row.AdsName, &row.RotatorID, &row.RotatorKey, &row.Status)
	if err != nil {
		return Experiment{}, err
	}

	return row, nil
}

func GetPagesByRotatorKey(db *sql.DB, rotatorKeyHex string) ([]Rotator, error) {
	var rotators []Rotator
	query := "SELECT * FROM z_rotator WHERE rotator_key = UNHEX(?)"
	rows, err := db.Query(query, rotatorKeyHex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rotator Rotator
		if err := rows.Scan(&rotator.PageID, &rotator.PageKey, &rotator.RotatorID, &rotator.RotatorKey); err != nil {
			return nil, err
		}
		rotators = append(rotators, rotator)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rotators, nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func InsertIntoTableUnhex(db *sql.DB, tableName string, data map[string]string, hexColumns []string) (bool, error) {
	if len(data) == 0 {
		return false, nil // No data to insert
	}

	// Construct the SQL query
	var columns, placeholders []string
	var values []interface{}
	for key, value := range data {
		columns = append(columns, key)
		placeholders = append(placeholders, "?")
		if contains(hexColumns, key) {
			values = append(values, fmt.Sprintf("UNHEX('%s')", value))
		} else {
			values = append(values, value)
		}
	}
	query := fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	// Prepare the SQL statement
	stmt, err := db.Prepare(query)
	if err != nil {
		return false, err
	}
	defer stmt.Close()

	// Execute the SQL statement with parameters
	result, err := stmt.Exec(values...)
	if err != nil {
		return false, err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	// If rows were affected, get the last inserted ID
	if rowsAffected > 0 {
		return true, nil
	}

	// If no rows were affected, return false, indicating that the insertion was ignored due to a duplicate key error
	return false, nil
}
func InsertIntoTable(db *sql.DB, tableName string, data map[string]string) (bool, error) {
	if len(data) == 0 {
		return false, nil // No data to insert
	}

	// Construct the SQL query
	var columns, placeholders []string
	var values []interface{}
	for key, value := range data {
		columns = append(columns, key)
		placeholders = append(placeholders, "?")
		values = append(values, value)
	}
	query := fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	// Prepare the SQL statement
	stmt, err := db.Prepare(query)
	if err != nil {
		return false, err
	}
	defer stmt.Close()

	// Execute the SQL statement with parameters
	result, err := stmt.Exec(values...)
	if err != nil {
		return false, err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	// If rows were affected, get the last inserted ID
	if rowsAffected > 0 {

		return true, nil
	}

	// If no rows were affected, return false, indicating that the insertion was ignored due to a duplicate key error
	return false, nil
}
