package standart

import (
	"strconv"
)

func GetStringValue(param *string, defaultValue *string) string {
	if param != nil && param != defaultValue {
		return *param
	}
	return ""
}

func GetUint64Value(param *uint64, defaultValue *uint64) string {
	if param != nil && param != defaultValue {
		return strconv.FormatUint(*param, 10)
	}
	return ""
}
