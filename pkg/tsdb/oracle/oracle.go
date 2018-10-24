package oracle

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/go-xorm/core"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb"
)

func init() {
	tsdb.RegisterTsdbQueryEndpoint("oci8", newOracleQueryEndpoint)
}

func newOracleQueryEndpoint(datasource *models.DataSource) (tsdb.TsdbQueryEndpoint, error) {
	logger := log.New("tsdb.oracle")

	cnnstr := generateConnectionString(datasource)

	logger.Debug("getEngine", "connection", cnnstr)

	config := tsdb.SqlQueryEndpointConfiguration{
		DriverName:        "oci8",
		ConnectionString:  cnnstr,
		Datasource:        datasource,
		MetricColumnTypes: []string{"CHAR", "VARCHAR", "VARCHAR2", "TEXT", "NUMBER"},
	}

	rowTransformer := oracleRowTransformer{
		log: logger,
	}

	timescaledb := datasource.JsonData.Get("timescaledb").MustBool(false)

	return tsdb.NewSqlQueryEndpoint(&config, &rowTransformer, newOracleMacroEngine(timescaledb), logger)
}

func generateConnectionString(datasource *models.DataSource) string {
	password := ""
	for key, value := range datasource.SecureJsonData.Decrypt() {
		if key == "password" {
			password = value
			break
		}
	}

	cnnstr := fmt.Sprintf("%s/%s@%s/%s",
		datasource.User,
		password,
		datasource.Url,
		datasource.Database,
	)

	return cnnstr
}

type oracleRowTransformer struct {
	log log.Logger
}

func (t *oracleRowTransformer) Transform(columnTypes []*sql.ColumnType, rows *core.Rows) (tsdb.RowValues, error) {
	values := make([]interface{}, len(columnTypes))
	valuePtrs := make([]interface{}, len(columnTypes))

	for i := 0; i < len(columnTypes); i++ {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}
	// convert types not handled by lib/pq
	// unhandled types are returned as []byte
	for i := 0; i < len(columnTypes); i++ {
		typeName := columnTypes[i].DatabaseTypeName()
		switch typeName {
		case "SQLT_NUM":
			if values[i] == nil {
				values[i] = float64(0)
			} else if v, err := strconv.ParseFloat(values[i].(string), 64); err == nil {
				values[i] = v
			} else {
				t.log.Debug("Rows", "Error converting numeric to float", values[i].(string))
			}
		case "SQLT_CHR":
			if values[i] == nil {
				values[i] = ""
			} else {
				values[i] = values[i].(string)
			}
		case "SQLT_AFC":
			if values[i] == nil {
				values[i] = ""
			} else {
				values[i] = values[i].(string)
			}
		default:

		}
	}

	return values, nil
}
