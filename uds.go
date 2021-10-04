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

	"github.com/u2386/go-hijack/runtime"
	"golang.org/x/sync/errgroup"
)

type uds struct {
	Addr    string
	Parser  Parser
	Runtime *runtime.Runtime
}

func (s *uds) Run(ctx context.Context) error {
	listener, err := net.Listen("unix", s.Addr)
	if err != nil {
		return err
	}
	critical("serving on %s", s.Addr)

	cleanup := func() {
		listener.Close()
		if _, err := os.Stat(s.Addr); err == nil {
			if err := os.RemoveAll(s.Addr); err != nil {
				critical("unexcepted error:%s", err)
			}
		}
		critical("server closed %s", s.Addr)
	}

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
			s.serve(conn)
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
		switch args {
		case "funcs":
			ns := s.Runtime.Funcs()
			io.Copy(conn, strings.NewReader(fmt.Sprint("funcs:", strings.Join(ns, "\n"))))
		case "points":
			ns := s.Runtime.Points()
			io.Copy(conn, strings.NewReader(fmt.Sprint("points:", strings.Join(ns, "\n"))))
		}

	case "/post":
		point := s.Parser.Parse(args)
		if point == nil {
			io.Copy(conn, strings.NewReader("error: parse error"))
			return
		}
		if err := s.Runtime.Hijack(point); err != nil {
			io.Copy(conn, strings.NewReader(fmt.Sprintf("error:%s", err)))
			return
		}
		io.Copy(conn, strings.NewReader("ok"))

	case "/delete":
		s.Runtime.Release(args)
		io.Copy(conn, strings.NewReader("ok"))

	default:
		io.Copy(conn, strings.NewReader(fmt.Sprint("unknown:", line)))
	}
}
