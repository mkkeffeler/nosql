package mysql

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"time"

	// import mysql driver anonymously (just run the init)
	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/smallstep/nosql/database"
)

// DB is a wrapper over *sql.DB,
type DB struct {
	db *sql.DB
}

// Open creates a Driver and connects to the database with the given address
// and access details.
func (db *DB) Open(dataSourceName string, opt ...database.Option) error {
	opts := &database.Options{}
	for _, o := range opt {
		if err := o(opts); err != nil {
			return err
		}
	}

	var err error
	_db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return errors.Wrap(err, "error connecting to mysql")
	}
	_, err = _db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", opts.Database))
	if err != nil {
		return errors.Wrapf(err, "error creating database %s (if not exists)", opts.Database)
	}
	db.db, err = sql.Open("mysql", dataSourceName+opts.Database)
	if err != nil {
		return errors.Wrapf(err, "error connecting to mysql database")
	}

	return nil
}

// Close shutsdown the database driver.
func (db *DB) Close() error {
	return errors.WithStack(db.db.Close())
}

func getQry(bucket []byte) string {
	return fmt.Sprintf("SELECT nvalue FROM `%s` WHERE nkey = ?", bucket)
}

func insertUpdateQry(bucket []byte) string {
	return fmt.Sprintf("INSERT INTO `%s`(nkey, nvalue) VALUES(?,?) ON DUPLICATE KEY UPDATE nvalue = ?", bucket)
}

// Count returns a number of entries in some table
func (db *DB) Count(bucket []byte) (int, error) {
	var count int
	err := db.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", bucket)).Scan(&count)

	if err != nil {
		return 0, errors.Wrapf(err, "Error getting count from '%s'", bucket)
	}

	return count, nil
}

func insertUpdateX509CertificateQry(bucket []byte) string {
	return fmt.Sprintf("INSERT INTO `%s`(nkey, nvalue, subjectNotBefore, subjectNotAfter, subjectState, subjectLocality, subjectCountry, subjectOrganization,subjectOrganizationalUnit, subjectCommonName, issuerDistinguishedName, provisionerName) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE nvalue = ?, subjectNotBefore = ?, subjectNotAfter = ?, subjectState = ?, subjectLocality = ?, subjectCountry = ?, subjectOrganization = ?,subjectOrganizationalUnit = ?, subjectCommonName = ?, issuerDistinguishedName = ?, provisionerName = ?", bucket)
}

func insertX509SansCertificateQry(bucket []byte) string {
	return fmt.Sprintf("INSERT INTO `%s`(nkey, sanText, sanType) VALUES(?,?,?);", bucket)
}

func insertX509ExtensionsCertificateQry(bucket []byte) string {
	return fmt.Sprintf("INSERT INTO `%s`(nkey, extensionOID) VALUES(?,?)", bucket)
}

func delQry(bucket []byte) string {
	return fmt.Sprintf("DELETE FROM `%s` WHERE nkey = ?", bucket)
}

func createTableQry(bucket []byte) string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`(nkey VARBINARY(255), nvalue BLOB, PRIMARY KEY (nkey));", bucket)
}

func createX509CertsTableQry(bucket []byte) string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`(nkey VARBINARY(255), nvalue BLOB, subjectNotBefore TIMESTAMP(0), subjectNotAfter TIMESTAMP(0), subjectState VARCHAR(255), subjectLocality VARCHAR(255), subjectCountry VARCHAR(255), subjectOrganization VARCHAR(255), subjectOrganizationalUnit VARCHAR(255),  subjectCommonName VARCHAR(255), issuerDistinguishedName VARCHAR(255), provisionerName VARCHAR(255), PRIMARY KEY (nkey));", bucket)
}

func createX509CertsSANSTableQry(bucket []byte) string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`(id MEDIUMINT NOT NULL AUTO_INCREMENT, nkey VARBINARY(255), sanText VARCHAR(255), sanType VARCHAR(255), PRIMARY KEY (id), CONSTRAINT CertSAN FOREIGN KEY (nkey) REFERENCES x509_certs(nkey));", bucket)
}

func createX509CertsExtensionsTableQry(bucket []byte) string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`(id MEDIUMINT NOT NULL AUTO_INCREMENT, nkey VARBINARY(255), extensionOID VARCHAR(255), PRIMARY KEY (id), CONSTRAINT CertExtension FOREIGN KEY (nkey) REFERENCES x509_certs(nkey));", bucket)
}

func alterX509CertsTableQry(bucket []byte) string {
	return fmt.Sprintf("ALTER TABLE `%s` ADD subjectNotBefore TIMESTAMP(0), ADD subjectNotAfter TIMESTAMP(0), ADD subjectState VARCHAR(255), ADD subjectLocality VARCHAR(255), ADD subjectCountry VARCHAR(255), ADD subjectOrganization VARCHAR(255), ADD subjectOrganizationalUnit VARCHAR(255), ADD subjectCommonName VARCHAR(255), ADD issuerDistinguishedName VARCHAR(255), ADD provisionerName VARCHAR(255);", bucket)
}

func alterX509CertsTableQryV3(bucket []byte) string {
	return fmt.Sprintf("ALTER TABLE `%s` ADD provisionerName VARCHAR(255);", bucket)
}

func deleteTableQry(bucket []byte) string {
	return fmt.Sprintf("DROP TABLE `%s`", bucket)
}

// Get retrieves the column/row with given key.
func (db *DB) Get(bucket, key []byte) ([]byte, error) {
	var val string
	err := db.db.QueryRow(getQry(bucket), key).Scan(&val)
	switch {
	case err == sql.ErrNoRows:
		return nil, errors.Wrapf(database.ErrNotFound, "%s/%s not found", bucket, key)
	case err != nil:
		return nil, errors.Wrapf(err, "failed to get %s/%s", bucket, key)
	default:
		return []byte(val), nil
	}
}

// Set inserts the key and value into the given bucket(column).
func (db *DB) Set(bucket, key, value []byte) error {
	_, err := db.db.Exec(insertUpdateQry(bucket), key, value, value)
	if err != nil {
		return errors.Wrapf(err, "failed to set %s/%s", bucket, key)
	}
	return nil
}

// Set inserts the key and value into the given bucket(column).
func (db *DB) SetX509Certificate(bucket, key, value []byte, notBefore time.Time, notAfter time.Time, province []string, locality []string, country []string, organization []string, organizationalUnit []string, commonName string, issuer string, extensions []map[interface{}]interface{}, sans []map[interface{}]interface{}, extensionBucket []byte, dnsNameBucket []byte, provisionerName string) error {
	_, err := db.db.Exec(insertUpdateX509CertificateQry(bucket), key, value, notBefore, notAfter, strings.Join(province, " "), strings.Join(locality, " "), strings.Join(country, " "), strings.Join(organization, " "), strings.Join(organizationalUnit, " "), commonName, issuer, provisionerName, value, notBefore, notAfter, strings.Join(province, " "), strings.Join(locality, " "), strings.Join(country, " "), strings.Join(organization, " "), strings.Join(organizationalUnit, " "), commonName, issuer, provisionerName)
	if err != nil {
		return errors.Wrapf(err, "failed to set %s/%s", bucket, key)
	}
	for _, extension := range extensions {
		_, err := db.db.Exec(insertX509ExtensionsCertificateQry(extensionBucket), key, extension["value"])
		if err != nil {
			return errors.Wrapf(err, "failed to set %s/%s", extensionBucket, key)
		}
	}
	for _, san := range sans {
		_, err := db.db.Exec(insertX509SansCertificateQry(dnsNameBucket), key, san["value"], san["type"])
		if err != nil {
			return errors.Wrapf(err, "failed to set %s/%s", dnsNameBucket, key)
		}
	}

	return nil
}

// Del deletes a row from the database.
func (db *DB) Del(bucket, key []byte) error {
	_, err := db.db.Exec(delQry(bucket), key)
	return errors.Wrapf(err, "failed to delete %s/%s", bucket, key)
}

// List returns the full list of entries in a column.
func (db *DB) rowsToList(rows *sql.Rows, err error, bucket []byte) ([]*database.Entry, error) {
	if err != nil {
		estr := err.Error()
		if strings.HasPrefix(estr, "Error 1146:") {
			return nil, errors.Wrapf(database.ErrNotFound, estr)
		}
		return nil, errors.Wrapf(err, "error querying table %s", bucket)
	}
	defer rows.Close()
	var (
		key, value                string
		subjectNotBefore          sql.NullString
		subjectNotAfter           sql.NullString
		subjectState              sql.NullString
		subjectLocality           sql.NullString
		subjectCountry            sql.NullString
		subjectOrganization       sql.NullString
		subjectOrganizationalUnit sql.NullString
		subjectCommonName         sql.NullString
		issuerDistinguishedName   sql.NullString
		provisionerName           sql.NullString
		entries                   []*database.Entry
	)
	for rows.Next() {
		err := rows.Scan(&key, &value, &subjectNotBefore, &subjectNotAfter, &subjectState, &subjectLocality, &subjectCountry, &subjectOrganization, &subjectOrganizationalUnit, &subjectCommonName, &issuerDistinguishedName, &provisionerName)
		if err != nil {
			return nil, errors.Wrap(err, "error getting key and value from row")

		}
		entries = append(entries, &database.Entry{
			Bucket:                    bucket,
			Key:                       []byte(key),
			Value:                     []byte(value),
			SubjectNotBefore:          subjectNotBefore,
			SubjectNotAfter:           subjectNotAfter,
			SubjectState:              subjectState,
			SubjectLocality:           subjectLocality,
			SubjectCountry:            subjectCountry,
			SubjectOrganization:       subjectOrganization,
			SubjectOrganizationalUnit: subjectOrganizationalUnit,
			SubjectCommonName:         subjectCommonName,
			ProvisionerName:           provisionerName,
			IssuerDistinguishedName:   issuerDistinguishedName,
		})
	}
	err = rows.Err()
	if err != nil {
		return nil, errors.Wrap(err, "error accessing row")
	}
	return entries, nil
}

// List returns the full list of entries in a column.
func (db *DB) List(bucket []byte) ([]*database.Entry, error) {
	rows, err := db.db.Query(fmt.Sprintf("SELECT * FROM `%s`", bucket))
	entries, rowsErr := db.rowsToList(rows, err, bucket)
	if rowsErr != nil {
		return nil, errors.Wrap(rowsErr, "error converting row to list")
	}

	return entries, nil
}

// ListPage returns a page worth of entries, whatever page size is specified. Better for performance on large DBs.
func (db *DB) ListPage(bucket []byte, limit int, offset int) ([]*database.Entry, error) {
	rows, err := db.db.Query(fmt.Sprintf("SELECT * FROM `%s` LIMIT %b OFFSET %b", bucket, limit, offset))
	if err != nil {
		return nil, errors.Wrap(err, "error Getting a page of results")
	}
	entries, rowsErr := db.rowsToList(rows, err, bucket)
	if rowsErr != nil {
		return nil, errors.Wrap(rowsErr, "error converting row to list")
	}

	return entries, nil
}

// CmpAndSwap modifies the value at the given bucket and key (to newValue)
// only if the existing (current) value matches oldValue.
func (db *DB) CmpAndSwap(bucket, key, oldValue, newValue []byte) ([]byte, bool, error) {
	sqlTx, err := db.db.Begin()
	if err != nil {
		return nil, false, errors.WithStack(err)
	}

	val, swapped, err := cmpAndSwap(sqlTx, bucket, key, oldValue, newValue)
	switch {
	case err != nil:
		if err := sqlTx.Rollback(); err != nil {
			return nil, false, errors.Wrapf(err, "failed to execute CmpAndSwap transaction on %s/%s and failed to rollback transaction", bucket, key)
		}
		return nil, false, err
	case swapped:
		if err := sqlTx.Commit(); err != nil {
			return nil, false, errors.Wrapf(err, "failed to commit badger transaction")
		}
		return val, swapped, nil
	default:
		if err := sqlTx.Rollback(); err != nil {
			return nil, false, errors.Wrapf(err, "failed to rollback read-only CmpAndSwap transaction on %s/%s", bucket, key)
		}
		return val, swapped, err
	}
}

func cmpAndSwap(sqlTx *sql.Tx, bucket, key, oldValue, newValue []byte) ([]byte, bool, error) {
	var current []byte
	err := sqlTx.QueryRow(getQry(bucket), key).Scan(&current)

	if err != nil && err != sql.ErrNoRows {
		return nil, false, err
	}
	if !bytes.Equal(current, oldValue) {
		return current, false, nil
	}

	if _, err = sqlTx.Exec(insertUpdateQry(bucket), key, newValue, newValue); err != nil {
		return nil, false, errors.Wrapf(err, "failed to set %s/%s", bucket, key)
	}
	return newValue, true, nil
}

// Update performs multiple commands on one read-write transaction.
func (db *DB) Update(tx *database.Tx) error {
	sqlTx, err := db.db.Begin()
	if err != nil {
		return errors.WithStack(err)
	}
	rollback := func(err error) error {
		if rollbackErr := sqlTx.Rollback(); rollbackErr != nil {
			return errors.Wrap(err, "UPDATE failed, unable to rollback transaction")
		}
		return errors.Wrap(err, "UPDATE failed")
	}
	for _, q := range tx.Operations {
		// create or delete buckets
		switch q.Cmd {
		case database.CreateTable:
			_, err := sqlTx.Exec(createTableQry(q.Bucket))
			if err != nil {
				return rollback(errors.Wrapf(err, "failed to create table %s", q.Bucket))
			}
		case database.DeleteTable:
			_, err := sqlTx.Exec(deleteTableQry(q.Bucket))
			if err != nil {
				estr := err.Error()
				if strings.HasPrefix(err.Error(), "Error 1051:") {
					return errors.Wrapf(database.ErrNotFound, estr)
				}
				return errors.Wrapf(err, "failed to delete table %s", q.Bucket)
			}
		case database.Get:
			var val string
			err := sqlTx.QueryRow(getQry(q.Bucket), q.Key).Scan(&val)
			switch {
			case err == sql.ErrNoRows:
				return rollback(errors.Wrapf(database.ErrNotFound, "%s/%s not found", q.Bucket, q.Key))
			case err != nil:
				return rollback(errors.Wrapf(err, "failed to get %s/%s", q.Bucket, q.Key))
			default:
				q.Result = []byte(val)
			}
		case database.Set:
			if _, err = sqlTx.Exec(insertUpdateQry(q.Bucket), q.Key, q.Value, q.Value); err != nil {
				return rollback(errors.Wrapf(err, "failed to set %s/%s", q.Bucket, q.Key))
			}
		case database.SetX509Certificate:
			if _, err = sqlTx.Exec(insertUpdateX509CertificateQry(q.Bucket), q.Key, q.Value, q.Value); err != nil {
				return rollback(errors.Wrapf(err, "failed to set %s/%s", q.Bucket, q.Key))
			}
		case database.Delete:
			if _, err = sqlTx.Exec(delQry(q.Bucket), q.Key); err != nil {
				return rollback(errors.Wrapf(err, "failed to delete %s/%s", q.Bucket, q.Key))
			}
		case database.CmpAndSwap:
			q.Result, q.Swapped, err = cmpAndSwap(sqlTx, q.Bucket, q.Key, q.CmpValue, q.Value)
			if err != nil {
				return rollback(errors.Wrapf(err, "failed to load-or-store %s/%s", q.Bucket, q.Key))
			}
		case database.CmpOrRollback:
			return database.ErrOpNotSupported
		default:
			return database.ErrOpNotSupported
		}
	}

	if err = errors.WithStack(sqlTx.Commit()); err != nil {
		return rollback(err)
	}
	return nil
}

// CreateTable creates a table in the database.
func (db *DB) CreateTable(bucket []byte) error {
	_, err := db.db.Exec(createTableQry(bucket))
	if err != nil {
		return errors.Wrapf(err, "failed to create table %s", bucket)
	}
	return nil
}

// CreateX509CertificateTable creates a table in the database, and also handles upgrading the schema of the table if it doesn't match this new schema (to support users who upgrade to this version.)
func (db *DB) CreateX509CertificateTable(bucket []byte) error {
	rows, colErr := db.db.Query(fmt.Sprintf("SELECT * FROM `%s`", bucket))

	//If it doesn't exist, we can create it the right way the first time.
	if colErr != nil {
		if strings.HasPrefix(colErr.Error(), "Error 1146:") { //Error 1146 means it doesn't exist. So we create it using the new schema.
			_, err := db.db.Exec(createX509CertsTableQry(bucket))
			if err != nil {
				return errors.Wrapf(err, "failed to create table %s", bucket)

			}
		} else { //Some other error occurred.
			return errors.Wrapf(colErr, "failed to get length of columns %s", bucket)
		}

	} else { //The table exists, we need to check if our columns are present.
		columnsList, colListErr := rows.Columns()
		if colListErr != nil {
			return errors.Wrapf(colErr, "failed to get length of columns %s", bucket)
		}

		// if len(rows.Columns()) == 2 we are using the old x509_certs schema, and we need to alter the table schema.
		if len(columnsList) == 2 {
			_, alterErr := db.db.Exec(alterX509CertsTableQry(bucket))
			if alterErr != nil {
				return errors.Wrapf(colErr, "failed to alter table to new schema %s", bucket)
			}
		}
		if len(columnsList) == 11 {
			_, alterErr := db.db.Exec(alterX509CertsTableQryV3(bucket))
			if alterErr != nil {
				return errors.Wrapf(colErr, "failed to alter table to new schema V3 %s", bucket)
			}
		}
	}
	return nil
}

// CreateX509CertificateSansTable creates a table to store the sans for certificates in a table so we can establish the one to many relationship of a certificate to SANS. This is needed so we can search by Sans as well.
func (db *DB) CreateX509CertificateSansTable(bucket []byte) error {
	_, err := db.db.Exec(createX509CertsSANSTableQry(bucket))
	if err != nil {
		return errors.Wrapf(err, "failed to create table %s", bucket)

	}
	return nil
}

// CreateX509CertificateExtensionsTable creates a table to store the sans for certificates in a table so we can establish the one to many relationship of a certificate to it's extensions. This is needed so we can search by Extensions as well.
func (db *DB) CreateX509CertificateExtensionsTable(bucket []byte) error {
	_, err := db.db.Exec(createX509CertsExtensionsTableQry(bucket))
	if err != nil {
		return errors.Wrapf(err, "failed to create table %s", bucket)

	}
	return nil
}

// DeleteTable deletes a table in the database.
func (db *DB) DeleteTable(bucket []byte) error {
	_, err := db.db.Exec(deleteTableQry(bucket))
	if err != nil {
		estr := err.Error()
		if strings.HasPrefix(err.Error(), "Error 1051:") {
			return errors.Wrapf(database.ErrNotFound, estr)
		}
		return errors.Wrapf(err, "failed to delete table %s", bucket)
	}
	return nil
}
