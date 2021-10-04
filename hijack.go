package gohijack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/u2386/go-hijack/runtime"
)

const (
	UDSAddress = "/tmp/gohijack.sock"
)

var (
	DEBUG          = ""
	ErrSetupFailed = errors.New("setup failed")
)

func critical(format string, args ...interface{}) {
	os.Stderr.Sync()
	fmt.Fprintf(os.Stderr, "GOHIJACK: "+format+"\n", args...)
}

func Hijack(ctx context.Context) error {
	pid := os.Getpid()
	r, err := runtime.New(pid)
	if err != nil {
		return fmt.Errorf("%s:%s", ErrSetupFailed, err)
	}
	go r.Run(ctx)

	server := &uds{
		Addr:    UDSAddress,
		Runtime: r,
		Parser:  JsonParser(),
	}

	go func() { err = server.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)
	return err
}
