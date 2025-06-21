package utils

import (
	stdjson "encoding/json"
	"log/slog"
	"os"

	json "github.com/json-iterator/go"
)

var Json = json.ConfigCompatibleWithStandardLibrary

// WriteJsonToFile write struct to json file
func WriteJsonToFile(dst string, data interface{}, std ...bool) bool {
	str, err := json.MarshalIndent(data, "", "  ")
	if len(std) > 0 && std[0] {
		str, err = stdjson.MarshalIndent(data, "", "  ")
	}
	if err != nil {
		slog.Error("failed to convert Conf to []byte", "error", err)
		return false
	}
	err = os.WriteFile(dst, str, 0777)
	if err != nil {
		slog.Error("failed to write json file", "error", err)
		return false
	}
	return true
}
