package sql

import (
	"testing"

	"github.com/jitsucom/bulker/bulkerlib/types"
	"github.com/stretchr/testify/require"
)

func TestSQLTypeHintAllowlist(t *testing.T) {
	testCases := []struct {
		name    string
		driver  string
		sqlType string
		allowed bool
	}{
		{name: "postgres varchar", driver: PostgresBulkerTypeId, sqlType: "varchar(4)", allowed: true},
		{name: "postgres timestamp", driver: PostgresBulkerTypeId, sqlType: "timestamp with time zone", allowed: true},
		{name: "mysql json", driver: MySQLBulkerTypeId, sqlType: "JSON", allowed: true},
		{name: "redshift super", driver: RedshiftBulkerTypeId, sqlType: "super", allowed: true},
		{name: "snowflake variant", driver: SnowflakeBulkerTypeId, sqlType: "VARIANT", allowed: true},
		{name: "snowflake number", driver: SnowflakeBulkerTypeId, sqlType: "NUMBER(38,0)", allowed: true},
		{name: "bigquery lowercase json", driver: BigqueryBulkerTypeId, sqlType: "json", allowed: true},
		{name: "bigquery float", driver: BigqueryBulkerTypeId, sqlType: "FLOAT", allowed: true},
		{name: "clickhouse float alias", driver: ClickHouseBulkerTypeId, sqlType: "FLOAT", allowed: true},
		{name: "clickhouse nullable datetime", driver: ClickHouseBulkerTypeId, sqlType: "Nullable(DateTime64(6))", allowed: true},
		{name: "clickhouse low cardinality nullable", driver: ClickHouseBulkerTypeId, sqlType: "LowCardinality(Nullable(Int64))", allowed: true},
		{name: "clickhouse nullable json array", driver: ClickHouseBulkerTypeId, sqlType: "Nullable(Array(JSON))", allowed: true},
		{name: "duckdb timestamp", driver: DuckDBBulkerTypeId, sqlType: "timestamp without time zone", allowed: true},

		{name: "sql statement", driver: PostgresBulkerTypeId, sqlType: "TEXT); DROP TABLE users;--", allowed: false},
		{name: "default expression", driver: PostgresBulkerTypeId, sqlType: "varchar(4) default text", allowed: false},
		{name: "quoted expression", driver: ClickHouseBulkerTypeId, sqlType: "Enum8('a'=1)", allowed: false},
		{name: "extra clickhouse clause", driver: ClickHouseBulkerTypeId, sqlType: "String DEFAULT now()", allowed: false},
		{name: "unbalanced wrapper", driver: ClickHouseBulkerTypeId, sqlType: "Nullable(String))", allowed: false},
		{name: "unknown wrapper", driver: ClickHouseBulkerTypeId, sqlType: "SimpleAggregateFunction(sum, Int64)", allowed: false},
		{name: "wrong driver", driver: BigqueryBulkerTypeId, sqlType: "varchar(4)", allowed: false},
		{name: "outer whitespace", driver: BigqueryBulkerTypeId, sqlType: " JSON", allowed: false},
		{name: "unknown driver", driver: "unknown", sqlType: "text", allowed: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.allowed, isSQLTypeHintAllowed(testCase.driver, testCase.sqlType))
		})
	}
}

func TestFilterSQLTypesHints(t *testing.T) {
	hints := types.SQLTypes{
		"safe":         {Type: "String", Override: true},
		"safe_pair":    {Type: "JSON", DdlType: "Nullable(JSON)", Override: true},
		"wrong_driver": {Type: "super", Override: true},
		"mixed_pair":   {Type: "String", DdlType: "super", Override: true},
		"expression":   {Type: "String); DROP TABLE events;--", Override: true},
	}

	filterSQLTypesHints(ClickHouseBulkerTypeId, hints)

	require.Equal(t, types.SQLTypes{
		"safe":      {Type: "String", Override: true},
		"safe_pair": {Type: "JSON", DdlType: "Nullable(JSON)", Override: true},
	}, hints)
}

func TestExtractSQLTypesHintsIgnoresMalformedValues(t *testing.T) {
	event := types.NewObject(8)
	event.Set("safe", "value")
	event.Set("__sql_type_safe", "JSON")
	event.Set("__sql_type_legacy_array", []any{"super"})
	event.Set("__sql_type_pair", []any{"String", "Nullable(String)"})
	event.Set("__sql_type_empty", []any{})
	event.Set("__sql_type_too_many", []any{"String", "String", "String"})
	event.Set("__sql_type_non_string", []any{"String", 42})
	event.Set("__sql_type_number", 42)

	hints, err := extractSQLTypesHints(event)
	require.NoError(t, err)
	require.Equal(t, types.SQLTypes{
		"safe":         {Type: "JSON", Override: true},
		"legacy_array": {Type: "super", Override: true},
		"pair":         {Type: "String", DdlType: "Nullable(String)", Override: true},
	}, hints)
	require.Equal(t, 1, event.Len())
	value, ok := event.Get("safe")
	require.True(t, ok)
	require.Equal(t, "value", value)
}

func TestProcessEventsDropsUnsafeHint(t *testing.T) {
	event := types.NewObject(2)
	event.Set("name", "test")
	event.Set("__sql_type_name", "text default current_user")

	header, processed, err := ProcessEvents("events", event, nil, func(value string) string { return value }, true, true, nil, false)
	require.NoError(t, err)
	require.Len(t, header.Fields, 1)
	_, overridden := header.Fields[0].GetSuggestedSQLType()
	require.False(t, overridden)
	_, hintPresent := processed.Get("__sql_type_name")
	require.False(t, hintPresent)
}

func TestFilterSQLTypesHintsForAnyDriverRequiresOneCompatibleDriver(t *testing.T) {
	hints := types.SQLTypes{
		"bigquery":   {Type: "FLOAT", Override: true},
		"redshift":   {Type: "super", Override: true},
		"mixed_pair": {Type: "FLOAT", DdlType: "super", Override: true},
		"expression": {Type: "text default current_user", Override: true},
	}

	filterSQLTypesHintsForAnyDriver(hints)

	require.Equal(t, types.SQLTypes{
		"bigquery": {Type: "FLOAT", Override: true},
		"redshift": {Type: "super", Override: true},
	}, hints)
}
