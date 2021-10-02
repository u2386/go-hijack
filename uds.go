package gohijack

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"strings"

	"github.com/drone/signal"
	"golang.org/x/sync/errgroup"
)

type uds struct {
	addr   string
}

func (s *uds) Run(ctx context.Context) error {
	listener, err := net.Listen("unix", s.addr)
	if err != nil {
		return err
	}
	critical("serving on %s", s.addr)

	ctx, cancel := context.WithCancel(ctx)
	cleanup := func() {
		listener.Close()
		if _, err := os.Stat(s.addr); err == nil {
			if err := os.RemoveAll(s.addr); err != nil {
				critical("unexcepted error:%s", err)
			}
		}
	}
	signal.WithContextFunc(ctx, func() { cancel() })

	var g errgroup.Group
	g.Go(func() error {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					critical("unexcepted error: %s", err)
					return err
				}
				return nil
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
	defer conn.Close()

	reader := textproto.NewReader(bufio.NewReader(conn))
	line, err := reader.ReadLine()
	if err != nil {
		critical("error: %s", err)
		return
	}
	critical("receive:%s", line)

	v := strings.SplitN(line, " ", 2)
	if len(v) != 2 {
		io.Copy(conn, bytes.NewReader([]byte(fmt.Sprint("unknown:", line))))
		return
	}

	switch comm, args := strings.TrimSpace(v[0]), strings.TrimSpace(v[1]); comm {
	case "/echo":
		io.Copy(conn, bytes.NewReader([]byte(args)))

	case "/get":
		var ns []string
		io.Copy(conn, strings.NewReader(fmt.Sprint("points:", strings.Join(ns, ", "))))

	case "/post":
		io.Copy(conn, strings.NewReader("error: parse error"))

	case "/delete":
		io.Copy(conn, strings.NewReader("ok"))

	default:
		io.Copy(conn, strings.NewReader(fmt.Sprint("unknown:", line)))
	}
}
