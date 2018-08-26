package oracle

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/tsdb"
)

//const rsString = `(?:"([^"]*)")`;
const rsIdentifier = `([_a-zA-Z0-9]+)`
const sExpr = `\$` + rsIdentifier + `\(([^\)]*)\)`
const TimeFormat = `YYYY-MM-DD"T"HH24:MI:SS"Z"`
const DaySeconds = 86400

var OracleDateFormatMap = map[string]string{
	"CC":    "CC",
	"DAY":   "DAY",
	"1D":    "D",
	"D":     "D",
	"DD":    "DD",
	"DDD":   "DDD",
	"DY":    "DY",
	"HH":    "HH",
	"HH12":  "HH12",
	"HH24":  "HH24",
	"1W":    "IW",
	"IW":    "IW",
	"IYYY":  "IYYY",
	"IYY":   "IYY",
	"IY":    "IY",
	"I":     "I",
	"J":     "J",
	"MI":    "MI",
	"MM":    "MM",
	"MON":   "MON",
	"MONTH": "MONTH",
	"Q":     "Q",
	"RM":    "RM",
	"RR":    "RR",
	"RRRR":  "RRRR",
	"W":     "W",
	"WW":    "WW",
	"Y,YYY": "Y,YYY",
	"YYYY":  "YYYY",
	"SYYYY": "SYYYY",
	"YYY":   "YYY",
	"YY":    "YY",
	"Y":     "Y",
}

type oracleMacroEngine struct {
	timeRange   *tsdb.TimeRange
	query       *tsdb.Query
	timescaledb bool
}

func newOracleMacroEngine(timescaledb bool) tsdb.SqlMacroEngine {
	return &oracleMacroEngine{timescaledb: timescaledb}
}

func (m *oracleMacroEngine) Interpolate(query *tsdb.Query, timeRange *tsdb.TimeRange, sql string) (string, error) {
	m.timeRange = timeRange
	m.query = query
	rExp, _ := regexp.Compile(sExpr)
	var macroError error

	sql = replaceAllStringSubmatchFunc(rExp, sql, func(groups []string) string {

		args := strings.Split(groups[2], ",")
		for i, arg := range args {
			args[i] = strings.Trim(arg, " ")
		}
		res, err := m.evaluateMacro(groups[1], args)
		if err != nil && macroError == nil {
			macroError = err
			return "macro_error()"
		}
		return res
	})

	if macroError != nil {
		return "", macroError
	}

	return sql, nil
}

func replaceAllStringSubmatchFunc(re *regexp.Regexp, str string, repl func([]string) string) string {
	result := ""
	lastIndex := 0

	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
		groups := []string{}
		for i := 0; i < len(v); i += 2 {
			groups = append(groups, str[v[i]:v[i+1]])
		}

		result += str[lastIndex:v[0]] + repl(groups)
		lastIndex = v[1]
	}

	return result + str[lastIndex:]
}

func (m *oracleMacroEngine) evaluateMacro(name string, args []string) (string, error) {
	switch name {
	case "__time":
		if len(args) == 0 {
			return "", fmt.Errorf("missing time column argument for macro %v", name)
		}
		return fmt.Sprintf("%s AS \"time\"", args[0]), nil
	case "__timeFilter":
		if len(args) == 0 {
			return "", fmt.Errorf("missing time column argument for macro %v", name)
		}
		return fmt.Sprintf("%s BETWEEN TO_TIMESTAMP('%s', '%s') AND TO_TIMESTAMP('%s', '%s')", args[0], m.timeRange.GetFromAsTimeUTC().Format(time.RFC3339), TimeFormat, m.timeRange.GetToAsTimeUTC().Format(time.RFC3339), TimeFormat), nil
	case "__timeFrom":
		return fmt.Sprintf("'%s'", m.timeRange.GetFromAsTimeUTC().Format(time.RFC3339)), nil
	case "__timeTo":
		return fmt.Sprintf("'%s'", m.timeRange.GetToAsTimeUTC().Format(time.RFC3339)), nil
	case "__timeGroup":
		if len(args) < 2 {
			return "", fmt.Errorf("macro %v needs time column and interval and optional fill value", name)
		}
		// if it is oracle date format
		if val, ok := OracleDateFormatMap[strings.Trim(args[1], `'`)]; ok {
			return fmt.Sprintf("trunc(%s, '%s')", args[0], val), nil
		}
		interval, err := time.ParseDuration(strings.Trim(args[1], `'`))
		if err != nil {
			return "", fmt.Errorf("error parsing interval %v", args[1])
		}
		if len(args) == 3 {
			err := tsdb.SetupFillmode(m.query, interval, args[2])
			if err != nil {
				return "", err
			}
		}
		seconds := DaySeconds / interval.Seconds()
		if seconds < 1 {
			return "", fmt.Errorf("Maxium internal cannot be smaller than 1, use Oracle Date Format instand. seconds: %v", seconds)
		}

		return fmt.Sprintf("trunc( (CAST(%s as DATE) - trunc(%s)) * %v )/ %v + trunc( %s )", args[0], args[0], seconds, seconds, args[0]), nil
	case "__timeGroupAlias":
		tg, err := m.evaluateMacro("__timeGroup", args)
		if err == nil {
			return tg + " AS \"time\"", err
		}
		return "", err
	default:
		return "", fmt.Errorf("Unknown macro %v", name)
	}
}
