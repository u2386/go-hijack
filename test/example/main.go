package main

import (
	"context"
	"time"

	gohijack "github.com/u2386/go-hijack"
)

var ctx = context.Background()

func init() {
	if err := gohijack.Hijack(ctx); err != nil {
		panic(err)
	}
}

func main() {
	time.Sleep(1 * time.Hour)
}
