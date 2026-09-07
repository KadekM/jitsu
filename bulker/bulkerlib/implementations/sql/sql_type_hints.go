package sql

import (
	"regexp"
	"strings"

	"github.com/jitsucom/bulker/bulkerlib/types"
)

var (
	postgresSQLTypeHint  = regexp.MustCompile(`(?i)^(?:text|varchar(?:\([1-9][0-9]{0,5}\))?|character +varying(?:\([1-9][0-9]{0,5}\))?|uuid|bigint|double +precision|timestamp(?:\([0-6]\))?(?: +with(?:out)? +time +zone)?|date|boolean|jsonb?)$`)
	mysqlSQLTypeHint     = regexp.MustCompile(`(?i)^(?:text|varchar(?:\([1-9][0-9]{0,5}\))?|bigint|double|timestamp(?:\([0-6]\))?|boolean|tinyint\(1\)|json)$`)
	redshiftSQLTypeHint  = regexp.MustCompile(`(?i)^(?:character +varying(?:\([1-9][0-9]{0,5}\))?|varchar(?:\([1-9][0-9]{0,5}\))?|bigint|double +precision|timestamp(?:\([0-6]\))?(?: +with(?:out)? +time +zone)?|boolean|super)$`)
	snowflakeSQLTypeHint = regexp.MustCompile(
		`(?i)^(?:text|varchar(?:\([1-9][0-9]{0,7}\))?|bigint|number(?:\([1-9][0-9]?(?:,[0-9]{1,2})?\))?|double +precision|float|timestamp(?:_(?:tz|ntz|ltz))?(?:\([0-9]\))?|boolean|variant|object|array)$`,
	)
	bigquerySQLTypeHint = regexp.MustCompile(`(?i)^(?:string|bytes|integer|int64|float|float64|decimal|numeric|bigdecimal|bignumeric|boolean|bool|timestamp|record|struct|date|time|datetime|geography|interval|json|range)$`)
	duckdbSQLTypeHint   = regexp.MustCompile(`(?i)^(?:text|varchar(?:\([1-9][0-9]{0,5}\))?|uuid|bigint|int|double|float|timestamp(?:\([0-6]\))?(?: +with(?:out)? +time +zone)?|boolean|bool|json)$`)

	clickhouseScalarSQLTypeHint = regexp.MustCompile(
		`(?i)^(?:string|fixedstring\([1-9][0-9]{0,5}\)|uuid|int(?:8|16|32|64|128|256)?|uint(?:8|16|32|64|128|256)?|float(?:32|64)?|decimal(?:32|64|128|256)?\([0-9]{1,3}(?:,[0-9]{1,3})?\)|date(?:32)?|datetime(?:64(?:\([0-9]\))?)?|json)$`,
	)
)

var sqlTypeHintDrivers = [...]string{
	PostgresBulkerTypeId,
	MySQLBulkerTypeId,
	RedshiftBulkerTypeId,
	SnowflakeBulkerTypeId,
	BigqueryBulkerTypeId,
	ClickHouseBulkerTypeId,
	DuckDBBulkerTypeId,
}

// filterSQLTypesHints removes event-provided type hints that are not valid for
// the destination driver. Stream-level ColumnTypes options are merged after
// this filter and are deliberately outside the __sql_type trust boundary.
func filterSQLTypesHints(driver string, hints types.SQLTypes) types.SQLTypes {
	for name, hint := range hints {
		if !isSQLColumnHintAllowed(driver, hint) {
			delete(hints, name)
		}
	}
	return hints
}

// ProcessEvents predates adapter-aware processing. Keep it safe by accepting a
// hint only when one supported driver allows both its cast and DDL types.
func filterSQLTypesHintsForAnyDriver(hints types.SQLTypes) types.SQLTypes {
	for name, hint := range hints {
		allowed := false
		for _, driver := range sqlTypeHintDrivers {
			if isSQLColumnHintAllowed(driver, hint) {
				allowed = true
				break
			}
		}
		if !allowed {
			delete(hints, name)
		}
	}
	return hints
}

func isSQLColumnHintAllowed(driver string, hint types.SQLColumn) bool {
	if !isSQLTypeHintAllowed(driver, hint.Type) {
		return false
	}
	return hint.DdlType == "" || isSQLTypeHintAllowed(driver, hint.DdlType)
}

func isSQLTypeHintAllowed(driver, sqlType string) bool {
	if sqlType == "" || sqlType != strings.TrimSpace(sqlType) {
		return false
	}
	switch driver {
	case PostgresBulkerTypeId:
		return postgresSQLTypeHint.MatchString(sqlType)
	case MySQLBulkerTypeId:
		return mysqlSQLTypeHint.MatchString(sqlType)
	case RedshiftBulkerTypeId:
		return redshiftSQLTypeHint.MatchString(sqlType)
	case SnowflakeBulkerTypeId:
		return snowflakeSQLTypeHint.MatchString(sqlType)
	case BigqueryBulkerTypeId:
		return bigquerySQLTypeHint.MatchString(sqlType)
	case ClickHouseBulkerTypeId:
		return isClickHouseSQLTypeHintAllowed(sqlType)
	case DuckDBBulkerTypeId:
		return duckdbSQLTypeHint.MatchString(sqlType)
	default:
		return false
	}
}

func isClickHouseSQLTypeHintAllowed(sqlType string) bool {
	if clickhouseScalarSQLTypeHint.MatchString(sqlType) {
		return true
	}
	if inner, ok := unwrapSQLType(sqlType, "Nullable"); ok {
		if clickhouseScalarSQLTypeHint.MatchString(inner) {
			return true
		}
		arrayInner, array := unwrapSQLType(inner, "Array")
		return array && strings.EqualFold(arrayInner, "JSON")
	}
	if inner, ok := unwrapSQLType(sqlType, "LowCardinality"); ok {
		if clickhouseScalarSQLTypeHint.MatchString(inner) {
			return true
		}
		nullableInner, nullable := unwrapSQLType(inner, "Nullable")
		return nullable && clickhouseScalarSQLTypeHint.MatchString(nullableInner)
	}
	if inner, ok := unwrapSQLType(sqlType, "Array"); ok {
		return strings.EqualFold(inner, "JSON")
	}
	return false
}

func unwrapSQLType(sqlType, wrapper string) (string, bool) {
	prefixLength := len(wrapper) + 1
	if len(sqlType) <= prefixLength || !strings.EqualFold(sqlType[:len(wrapper)], wrapper) || sqlType[len(wrapper)] != '(' || sqlType[len(sqlType)-1] != ')' {
		return "", false
	}
	return sqlType[prefixLength : len(sqlType)-1], true
}
