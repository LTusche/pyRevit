package persistence

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"../cli"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	uuid "github.com/satori/go.uuid"
)

type GenericSQLConnection struct {
	DatabaseConnection
}

func (w GenericSQLConnection) WriteScriptTelemetry(logrec *ScriptTelemetryRecord, logger *cli.Logger) (*Result, error) {
	// generate generic sql insert query
	logger.Debug("generating query")
	query, qErr := generateScriptInsertQuery(w.Config.ScriptTarget, logrec, logger)
	if qErr != nil {
		return nil, qErr
	}

	return commitSQL(w.Config.Backend, w.Config.ConnString, query, logger)
}

func (w GenericSQLConnection) WriteEventTelemetry(logrec *EventTelemetryRecord, logger *cli.Logger) (*Result, error) {
	// generate generic sql insert query
	logger.Debug("generating query")
	query, qErr := generateEventInsertQuery(w.Config.EventTarget, logrec, logger)
	if qErr != nil {
		return nil, qErr
	}

	return commitSQL(w.Config.Backend, w.Config.ConnString, query, logger)
}

func commitSQL(backend DBBackendName, connStr string, query string, logger *cli.Logger) (*Result, error) {
	// open connection
	db, err := openConnection(backend, connStr, logger)
	if err != nil {
		logger.Debug("error opening connection")
		return nil, err
	}
	defer db.Close()

	// start transaction
	logger.Debug("opening transaction")
	tx, beginErr := db.Begin()
	if beginErr != nil {
		logger.Debug("error opening transaction")
		return nil, beginErr
	}
	defer tx.Rollback()

	// run the insert query
	logger.Debug("executing insert query")
	_, eErr := db.Exec(query)
	if eErr != nil {
		return nil, eErr
	}

	// commit transaction
	logger.Debug("commiting transaction")
	txnErr := tx.Commit()
	if txnErr != nil {
		return nil, txnErr
	}

	logger.Debug("preparing report")
	return &Result{
		Message: "successfully inserted usage record",
	}, nil
}

func openConnection(backend DBBackendName, connStr string, logger *cli.Logger) (*sql.DB, error) {
	// open connection
	logger.Debug(fmt.Sprintf("opening %s connection", backend))
	cleanConnStr := connStr
	if backend == Sqlite || backend == MySql {
		cleanConnStr = strings.Replace(connStr, string(backend)+":", "", 1)
	}
	return sql.Open(string(backend), cleanConnStr)
}

func generateScriptInsertQuery(table string, logrec *ScriptTelemetryRecord, logger *cli.Logger) (string, error) {
	// read csv file and build sql insert query
	var querystr strings.Builder

	logger.Debug("generating insert query with-out headers")
	querystr.WriteString(fmt.Sprintf("INSERT INTO %s values ", table))

	// build sql data info
	logger.Debug("building insert query for data")
	datalines := make([]string, 0)

	cresults, merr := json.Marshal(logrec.CommandResults)
	if merr != nil {
		logger.Debug("error logging command results")
	}

	// create record based on schema
	var record []string

	// generate record id, panic if error
	recordId := uuid.Must(uuid.NewV4())

	if logrec.LogMeta.SchemaVersion == "" {
		re := regexp.MustCompile(`(\d+:\d+:\d+)`)
		record = []string{
			logrec.Date,
			re.FindString(logrec.Time),
			logrec.UserName,
			logrec.RevitVersion,
			logrec.RevitBuild,
			logrec.SessionId,
			logrec.PyRevitVersion,
			strconv.FormatBool(logrec.IsDebugMode),
			strconv.FormatBool(logrec.IsConfigMode),
			logrec.CommandName,
			logrec.BundleName,
			logrec.ExtensionName,
			logrec.CommandUniqueName,
			strconv.Itoa(logrec.ResultCode),
			string(cresults),
			logrec.ScriptPath,
			logrec.TraceInfo.EngineInfo.Version,
			logrec.TraceInfo.IronPythonTraceDump,
			logrec.TraceInfo.CLRTraceDump,
		}

	} else if logrec.LogMeta.SchemaVersion == "2.0" {
		record = []string{
			recordId.String(),
			logrec.TimeStamp,
			logrec.UserName,
			logrec.RevitVersion,
			logrec.RevitBuild,
			logrec.SessionId,
			logrec.PyRevitVersion,
			logrec.Clone,
			strconv.FormatBool(logrec.IsDebugMode),
			strconv.FormatBool(logrec.IsConfigMode),
			strconv.FormatBool(logrec.IsExecFromGUI),
			strconv.FormatBool(logrec.NeedsCleanEngine),
			strconv.FormatBool(logrec.NeedsFullFrameEngine),
			logrec.CommandName,
			logrec.BundleName,
			logrec.ExtensionName,
			logrec.CommandUniqueName,
			logrec.DocumentName,
			logrec.DocumentPath,
			strconv.Itoa(logrec.ResultCode),
			string(cresults),
			logrec.ScriptPath,
			logrec.TraceInfo.EngineInfo.Type,
			logrec.TraceInfo.EngineInfo.Version,
			logrec.TraceInfo.Message,
		}
	}
	datalines = append(datalines, ToSql(&record, true))

	// add csv records to query string
	all_datalines := strings.Join(datalines, ", ")
	logger.Trace(all_datalines)
	querystr.WriteString(all_datalines)
	querystr.WriteString(";\n")
	logger.Debug("building query completed")

	// execute query
	full_query := querystr.String()
	logger.Trace(full_query)
	return full_query, nil
}

func generateEventInsertQuery(table string, logrec *EventTelemetryRecord, logger *cli.Logger) (string, error) {
	// read csv file and build sql insert query
	var querystr strings.Builder

	logger.Debug("generating insert query with-out headers")
	querystr.WriteString(fmt.Sprintf("INSERT INTO %s values ", table))

	// build sql data info
	logger.Debug("building insert query for data")
	datalines := make([]string, 0)

	// create record based on schema
	var record []string

	// generate record id, panic if error
	recordId := uuid.Must(uuid.NewV4())

	if logrec.LogMeta.SchemaVersion == "2.0" {
		record = []string{
			recordId.String(),
			logrec.TimeStamp,
			logrec.EventType,
			logrec.UserName,
		}
	}
	datalines = append(datalines, ToSql(&record, true))

	// add csv records to query string
	all_datalines := strings.Join(datalines, ", ")
	logger.Trace(all_datalines)
	querystr.WriteString(all_datalines)
	querystr.WriteString(";\n")
	logger.Debug("building query completed")

	// execute query
	full_query := querystr.String()
	logger.Trace(full_query)
	return full_query, nil
}
