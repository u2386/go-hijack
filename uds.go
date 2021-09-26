package gohijack

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/drone/signal"
	"golang.org/x/sync/errgroup"
)

type uds struct {
	addr string
}

func (s *uds) Run(ctx context.Context) error {
	listener, err := net.Listen("unix", s.addr)
	if err != nil {
		return err
	}

	cleanup := func() {
		listener.Close()
		if _, err := os.Stat(s.addr); err == nil {
			if err := os.RemoveAll(s.addr); err != nil {
				fmt.Fprintf(os.Stderr, "unexcepted error:%s", err)
			}
		}
	}
	signal.WithContextFunc(ctx, cleanup)

	var g errgroup.Group
	g.Go(func() error {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					fmt.Fprintf(os.Stderr, "unexcepted error: %s", err)
				}
				return err
			}
			go s.serve(conn)
		}
	})
	g.Go(func() error {
		<-ctx.Done()
		cleanup()
		return nil
	})
	return g.Wait()
}

func (s *uds) serve(conn net.Conn) {

}
