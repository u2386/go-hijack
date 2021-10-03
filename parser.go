package gohijack

import (
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/u2386/go-hijack/runtime"
)

type (
	Parser interface {
		Parse(string) *runtime.HijackPoint
	}

	simple struct{}
)

func SimpleParser() *simple {
	return &simple{}
}

func (*simple) Parse(content string) *runtime.HijackPoint {
	m := make(map[string]interface{})
	for _, s := range strings.Split(content, ",") {
		v := strings.Split(strings.TrimSpace(s), ":")
		m[strings.TrimSpace(v[0])] = strings.TrimSpace(v[1])
	}
	var p runtime.HijackPoint
	if err := mapstructure.Decode(m, &p); err != nil {
		debug("parse error: %s", err)
		return nil
	}
	return &p
}
