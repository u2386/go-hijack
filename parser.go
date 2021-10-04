package gohijack

import (
	"encoding/json"
	"github.com/u2386/go-hijack/runtime"
)

type (
	Parser interface {
		Parse(string) runtime.Request
	}

	jsonparser struct{}
)

func JsonParser() *jsonparser {
	return &jsonparser{}
}

func (*jsonparser) Parse(content string) runtime.Request {
    var m map[string]interface{}
	if err := json.Unmarshal([]byte(content), &m); err != nil {
		return nil
	}
	return m
}
