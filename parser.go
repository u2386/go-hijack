package gohijack

import (
	"encoding/json"
	"github.com/u2386/go-hijack/runtime"
)

type (
	Parser interface {
		Parse(string) *runtime.HijackPoint
	}

	jsonparser struct{}
)

func JsonParser() *jsonparser {
	return &jsonparser{}
}

func (*jsonparser) Parse(content string) *runtime.HijackPoint {
	p := &runtime.HijackPoint{}
	if err := json.Unmarshal([]byte(content), p); err != nil {
		return nil
	}
	return p
}
