/*
 * MinIO Cloud Storage, (C) 2018 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// MySQL Notifier implementation. Two formats, "namespace" and
// "access" are supported.
//
// * Namespace format
//
// On each create or update object event in MinIO Object storage
// server, a row is created or updated in the table in MySQL. On each
// object removal, the corresponding row is deleted from the table.
//
// A table with a specific structure (column names, column types, and
// primary key/uniqueness constraint) is used. The user may set the
// table name in the configuration. A sample SQL command that creates
// a command with the required structure is:
//
//     CREATE TABLE myminio (
//         key_name VARCHAR(2048),
//         value JSONB,
//         PRIMARY KEY (key_name),
//     );
//
// MySQL's "INSERT ... ON DUPLICATE ..." feature (UPSERT) is used
// here. The implementation has been tested with MySQL Ver 14.14
// Distrib 5.7.17.
//
// * Access format
//
// On each event, a row is appended to the configured table. There is
// no deletion or modification of existing rows.
//
// A different table schema is used for this format. A sample SQL
// commant that creates a table with the required structure is:
//
// CREATE TABLE myminio (
//     event_time TIMESTAMP WITH TIME ZONE NOT NULL,
//     event_data JSONB
// );

package target

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/minio/minio/pkg/event"
	xnet "github.com/minio/minio/pkg/net"
)

const (
	mysqlTableExists          = `SELECT 1 FROM %s;`
	mysqlCreateNamespaceTable = `CREATE TABLE %s (key_name VARCHAR(2048), value JSON, PRIMARY KEY (key_name));`
	mysqlCreateAccessTable    = `CREATE TABLE %s (event_time DATETIME NOT NULL, event_data JSON);`

	mysqlUpdateRow = `INSERT INTO %s (key_name, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value=VALUES(value);`
	mysqlDeleteRow = `DELETE FROM %s WHERE key_name = ?;`
	mysqlInsertRow = `INSERT INTO %s (event_time, event_data) VALUES (?, ?);`
)

// MySQL related constants
const (
	MySQLFormat     = "format"
	MySQLDSNString  = "dsn_string"
	MySQLTable      = "table"
	MySQLHost       = "host"
	MySQLPort       = "port"
	MySQLUsername   = "username"
	MySQLPassword   = "password"
	MySQLDatabase   = "database"
	MySQLQueueLimit = "queue_limit"
	MySQLQueueDir   = "queue_dir"

	EnvMySQLEnable     = "MINIO_NOTIFY_MYSQL_ENABLE"
	EnvMySQLFormat     = "MINIO_NOTIFY_MYSQL_FORMAT"
	EnvMySQLDSNString  = "MINIO_NOTIFY_MYSQL_DSN_STRING"
	EnvMySQLTable      = "MINIO_NOTIFY_MYSQL_TABLE"
	EnvMySQLHost       = "MINIO_NOTIFY_MYSQL_HOST"
	EnvMySQLPort       = "MINIO_NOTIFY_MYSQL_PORT"
	EnvMySQLUsername   = "MINIO_NOTIFY_MYSQL_USERNAME"
	EnvMySQLPassword   = "MINIO_NOTIFY_MYSQL_PASSWORD"
	EnvMySQLDatabase   = "MINIO_NOTIFY_MYSQL_DATABASE"
	EnvMySQLQueueLimit = "MINIO_NOTIFY_MYSQL_QUEUE_LIMIT"
	EnvMySQLQueueDir   = "MINIO_NOTIFY_MYSQL_QUEUE_DIR"
)

// MySQLArgs - MySQL target arguments.
type MySQLArgs struct {
	Enable     bool     `json:"enable"`
	Format     string   `json:"format"`
	DSN        string   `json:"dsnString"`
	Table      string   `json:"table"`
	Host       xnet.URL `json:"host"`
	Port       string   `json:"port"`
	User       string   `json:"user"`
	Password   string   `json:"password"`
	Database   string   `json:"database"`
	QueueDir   string   `json:"queueDir"`
	QueueLimit uint64   `json:"queueLimit"`
}

// Validate MySQLArgs fields
func (m MySQLArgs) Validate() error {
	if !m.Enable {
		return nil
	}

	if m.Format != "" {
		f := strings.ToLower(m.Format)
		if f != event.NamespaceFormat && f != event.AccessFormat {
			return fmt.Errorf("unrecognized format")
		}
	}

	if m.Table == "" {
		return fmt.Errorf("table unspecified")
	}

	if m.DSN != "" {
		if _, err := mysql.ParseDSN(m.DSN); err != nil {
			return err
		}
	} else {
		// Some fields need to be specified when DSN is unspecified
		if m.Port == "" {
			return fmt.Errorf("unspecified port")
		}
		if _, err := strconv.Atoi(m.Port); err != nil {
			return fmt.Errorf("invalid port")
		}
		if m.Database == "" {
			return fmt.Errorf("database unspecified")
		}
	}

	if m.QueueDir != "" {
		if !filepath.IsAbs(m.QueueDir) {
			return errors.New("queueDir path should be absolute")
		}
	}
	if m.QueueLimit > 10000 {
		return errors.New("queueLimit should not exceed 10000")
	}

	return nil
}

// MySQLTarget - MySQL target.
type MySQLTarget struct {
	id         event.TargetID
	args       MySQLArgs
	updateStmt *sql.Stmt
	deleteStmt *sql.Stmt
	insertStmt *sql.Stmt
	db         *sql.DB
	store      Store
	firstPing  bool
}

// ID - returns target ID.
func (target *MySQLTarget) ID() event.TargetID {
	return target.id
}

// IsActive - Return true if target is up and active
func (target *MySQLTarget) IsActive() (bool, error) {
	if err := target.db.Ping(); err != nil {
		if IsConnErr(err) {
			return false, errNotConnected
		}
		return false, err
	}
	return true, nil
}

// Save - saves the events to the store which will be replayed when the SQL connection is active.
func (target *MySQLTarget) Save(eventData event.Event) error {
	if target.store != nil {
		return target.store.Put(eventData)
	}
	_, err := target.IsActive()
	if err != nil {
		return err
	}
	return target.send(eventData)
}

// send - sends an event to the mysql.
func (target *MySQLTarget) send(eventData event.Event) error {
	if target.args.Format == event.NamespaceFormat {
		objectName, err := url.QueryUnescape(eventData.S3.Object.Key)
		if err != nil {
			return err
		}
		key := eventData.S3.Bucket.Name + "/" + objectName

		if eventData.EventName == event.ObjectRemovedDelete {
			_, err = target.deleteStmt.Exec(key)
		} else {
			var data []byte
			if data, err = json.Marshal(struct{ Records []event.Event }{[]event.Event{eventData}}); err != nil {
				return err
			}

			_, err = target.updateStmt.Exec(key, data)
		}

		return err
	}

	if target.args.Format == event.AccessFormat {
		eventTime, err := time.Parse(event.AMZTimeFormat, eventData.EventTime)
		if err != nil {
			return err
		}

		data, err := json.Marshal(struct{ Records []event.Event }{[]event.Event{eventData}})
		if err != nil {
			return err
		}

		_, err = target.insertStmt.Exec(eventTime, data)

		return err
	}

	return nil
}

// Send - reads an event from store and sends it to MySQL.
func (target *MySQLTarget) Send(eventKey string) error {

	_, err := target.IsActive()
	if err != nil {
		return err
	}

	if !target.firstPing {
		if err := target.executeStmts(); err != nil {
			if IsConnErr(err) {
				return errNotConnected
			}
			return err
		}
	}

	eventData, eErr := target.store.Get(eventKey)
	if eErr != nil {
		// The last event key in a successful batch will be sent in the channel atmost once by the replayEvents()
		// Such events will not exist and wouldve been already been sent successfully.
		if os.IsNotExist(eErr) {
			return nil
		}
		return eErr
	}

	if err := target.send(eventData); err != nil {
		if IsConnErr(err) {
			return errNotConnected
		}
		return err
	}

	// Delete the event from store.
	return target.store.Del(eventKey)
}

// Close - closes underneath connections to MySQL database.
func (target *MySQLTarget) Close() error {
	if target.updateStmt != nil {
		// FIXME: log returned error. ignore time being.
		_ = target.updateStmt.Close()
	}

	if target.deleteStmt != nil {
		// FIXME: log returned error. ignore time being.
		_ = target.deleteStmt.Close()
	}

	if target.insertStmt != nil {
		// FIXME: log returned error. ignore time being.
		_ = target.insertStmt.Close()
	}

	return target.db.Close()
}

// Executes the table creation statements.
func (target *MySQLTarget) executeStmts() error {

	_, err := target.db.Exec(fmt.Sprintf(mysqlTableExists, target.args.Table))
	if err != nil {
		createStmt := mysqlCreateNamespaceTable
		if target.args.Format == event.AccessFormat {
			createStmt = mysqlCreateAccessTable
		}

		if _, dbErr := target.db.Exec(fmt.Sprintf(createStmt, target.args.Table)); dbErr != nil {
			return dbErr
		}
	}

	switch target.args.Format {
	case event.NamespaceFormat:
		// insert or update statement
		if target.updateStmt, err = target.db.Prepare(fmt.Sprintf(mysqlUpdateRow, target.args.Table)); err != nil {
			return err
		}
		// delete statement
		if target.deleteStmt, err = target.db.Prepare(fmt.Sprintf(mysqlDeleteRow, target.args.Table)); err != nil {
			return err
		}
	case event.AccessFormat:
		// insert statement
		if target.insertStmt, err = target.db.Prepare(fmt.Sprintf(mysqlInsertRow, target.args.Table)); err != nil {
			return err
		}
	}

	return nil

}

// NewMySQLTarget - creates new MySQL target.
func NewMySQLTarget(id string, args MySQLArgs, doneCh <-chan struct{}, loggerOnce func(ctx context.Context, err error, id interface{}, kind ...interface{})) (*MySQLTarget, error) {
	var firstPing bool
	if args.DSN == "" {
		config := mysql.Config{
			User:                 args.User,
			Passwd:               args.Password,
			Net:                  "tcp",
			Addr:                 args.Host.String() + ":" + args.Port,
			DBName:               args.Database,
			AllowNativePasswords: true,
		}

		args.DSN = config.FormatDSN()
	}

	db, err := sql.Open("mysql", args.DSN)
	if err != nil {
		return nil, err
	}

	var store Store

	if args.QueueDir != "" {
		queueDir := filepath.Join(args.QueueDir, storePrefix+"-mysql-"+id)
		store = NewQueueStore(queueDir, args.QueueLimit)
		if oErr := store.Open(); oErr != nil {
			return nil, oErr
		}
	}

	target := &MySQLTarget{
		id:        event.TargetID{ID: id, Name: "mysql"},
		args:      args,
		db:        db,
		store:     store,
		firstPing: firstPing,
	}

	err = target.db.Ping()
	if err != nil {
		if target.store == nil || !(IsConnRefusedErr(err) || IsConnResetErr(err)) {
			return nil, err
		}
	} else {
		if err = target.executeStmts(); err != nil {
			return nil, err
		}
		target.firstPing = true
	}

	if target.store != nil {
		// Replays the events from the store.
		eventKeyCh := replayEvents(target.store, doneCh, loggerOnce, target.ID())
		// Start replaying events from the store.
		go sendEvents(target, eventKeyCh, doneCh, loggerOnce)
	}

	return target, nil
}
