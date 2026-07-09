package json

import (
	"encoding/json"
	"fmt"
)

func ToJson(v interface{}) string {
	result, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("Error JSON-encoding: %v", err))
	}

	return string(result)
}

func ToPrettyJson(v interface{}) string {
	result, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("Error JSON-encoding: %v", err))
	}

	return string(result)
}
