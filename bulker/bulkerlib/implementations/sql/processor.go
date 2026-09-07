package sql

import (
	"github.com/jitsucom/bulker/bulkerlib/implementations"
	"github.com/jitsucom/bulker/bulkerlib/types"
	types2 "github.com/jitsucom/bulker/jitsubase/types"
	"github.com/jitsucom/bulker/jitsubase/utils"
	"strings"
)

// ProcessEvents processes events objects without applying mapping rules
// returns table headerm array of processed objects
// or error if at least 1 was occurred
func ProcessEvents(tableName string, event types.Object, customTypes types.SQLTypes, nameTransformer func(string) string, omitNils bool, stringifyObjects bool, notFlatteningKeys types2.Set[string], skipTypeHints bool) (*TypesHeader, types.Object, error) {
	_ = event.Delete("JITSU_TABLE_NAME")
	notFlatKeys := notFlatteningKeys
	var sqlTypesHints types.SQLTypes
	var err error
	if !skipTypeHints {
		sqlTypesHints, err = extractSQLTypesHints(event)
		if err != nil {
			return nil, nil, err
		}
		sqlTypesHints = filterSQLTypesHintsForAnyDriver(sqlTypesHints)
	}
	if len(customTypes) > 0 {
		if sqlTypesHints == nil {
			sqlTypesHints = types.SQLTypes{}
		}
		for k, v := range customTypes {
			sqlTypesHints[k] = v
		}
	}
	if len(sqlTypesHints) > 0 {
		notFlatKeys = types2.NewSet[string]()
		for key := range sqlTypesHints {
			notFlatKeys.Put(key)
		}
		notFlatKeys.PutSet(notFlatteningKeys)
	}
	flatObject, err := implementations.NewFlattener(nameTransformer, omitNils, stringifyObjects).FlattenObject(event, notFlatKeys)
	if err != nil {
		return nil, nil, err
	}
	fields, err := DefaultTypeResolver.Resolve(flatObject, sqlTypesHints)
	if err != nil {
		return nil, nil, err
	}
	bh := &TypesHeader{TableName: tableName, Fields: fields}

	return bh, flatObject, nil
}

func extractSQLTypesHints(object types.Object) (types.SQLTypes, error) {
	result := types.SQLTypes{}
	err := _extractSQLTypesHints("", object, result)
	return result, err
}

func _extractSQLTypesHints(key string, object types.Object, result types.SQLTypes) error {
	for el := object.Front(); el != nil; {
		k := el.Key
		v := el.Value
		curEl := el
		// move to the next element before deleting the current one. otherwise iteration will be broken
		el = el.Next()
		//if column has __sql_type_ prefix
		if strings.HasPrefix(k, types2.SqlTypePrefix) {
			object.DeleteElement(curEl)
			columnName := strings.TrimPrefix(k[len(types2.SqlTypePrefix):], "_")
			//when columnName is empty it means that provided sql type is meant for the whole object
			//e.g. to map nested object to sql JSON type you can add the following property to nested object: "__sql_type_": "JSON" )
			mappedColumnName := utils.JoinNonEmptyStrings("_", key, columnName)
			if hint, ok := parseSQLTypeHint(v); ok {
				result[mappedColumnName] = hint
			}
		} else if val, ok := v.(types.Object); ok {
			err := _extractSQLTypesHints(utils.JoinNonEmptyStrings("_", key, k), val, result)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func parseSQLTypeHint(value any) (types.SQLColumn, bool) {
	switch value := value.(type) {
	case string:
		if value == "" {
			return types.SQLColumn{}, false
		}
		return types.SQLColumn{Type: value, Override: true}, true
	case []any:
		values := make([]string, len(value))
		for i, item := range value {
			stringValue, ok := item.(string)
			if !ok {
				return types.SQLColumn{}, false
			}
			values[i] = stringValue
		}
		return parseSQLTypeHintArray(values)
	case []string:
		return parseSQLTypeHintArray(value)
	default:
		return types.SQLColumn{}, false
	}
}

func parseSQLTypeHintArray(value []string) (types.SQLColumn, bool) {
	if len(value) < 1 || len(value) > 2 || value[0] == "" {
		return types.SQLColumn{}, false
	}
	hint := types.SQLColumn{Type: value[0], Override: true}
	if len(value) == 2 {
		if value[1] == "" {
			return types.SQLColumn{}, false
		}
		hint.DdlType = value[1]
	}
	return hint, true
}

//
//// foldLongFields replace all column names with truncated values if they exceed the limit
//// uses cutName under the hood
//func foldLongFields(header *TypesHeader, object types.Object) (*TypesHeader, types.Object, error) {
//	//TODO: Get maxColumnNameLen from storage config
//	maxColumnNameLen := 0
//
//	if maxColumnNameLen <= 0 {
//		return header, object, nil
//	}
//
//	changes := map[string]string{}
//	for name := range header.Fields {
//		if len(name) > maxColumnNameLen {
//			newName := cutName(name, maxColumnNameLen)
//			if name != newName {
//				changes[name] = newName
//			}
//		}
//	}
//
//	for oldName, newName := range changes {
//		field, _ := header.Fields[oldName]
//		delete(header.Fields, oldName)
//		header.Fields[newName] = field
//
//		if value, ok := object[oldName]; ok {
//			delete(object, oldName)
//			object[newName] = value
//		}
//	}
//
//	return header, object, nil
//}
//
//// cutName converts input name that exceeds maxLen to lower length string by cutting parts between '_' to 2 symbols.
//// if name len is still greater than returns maxLen symbols from the end of the name
//func cutName(name string, maxLen int) string {
//	if len(name) <= maxLen {
//		return name
//	}
//
//	//just cut from the beginning
//	if !strings.Contains(name, "_") {
//		return name[len(name)-maxLen:]
//	}
//
//	var replaced bool
//	replace := ""
//	for _, part := range strings.Split(name, "_") {
//		if replace != "" {
//			replace += "_"
//		}
//
//		if len(part) > 2 {
//			newPart := part[:2]
//			name = strings.ReplaceAll(name, replace+part, replace+newPart)
//			replaced = true
//			break
//		} else {
//			replace += part
//		}
//	}
//
//	if !replaced {
//		//case when ab_ac_ad and maxLen = 6
//		return name[len(name)-maxLen:]
//	}
//
//	return cutName(name, maxLen)
//}
