package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/drone/signal"
	"github.com/gorilla/mux"
	gohijack "github.com/u2386/go-hijack"
)

//go:noinline
func DoingSomething(s string) string {
	return fmt.Sprintf("foo:%s", s)
}

func EchoHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	data := DoingSomething(string(body))
	fmt.Fprintf(w, "echo:%s", []byte(data))
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	if err := gohijack.Hijack(ctx); err != nil {
		panic(err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/echo", EchoHandler)
	http.Handle("/", r)

	srv := &http.Server{
		Handler: r,
		Addr:    "127.0.0.1:8000",
	}
	go func() {
		fmt.Fprintf(os.Stderr, "serving on:%s\n", srv.Addr)
		fmt.Fprintf(os.Stderr, "server exits:%s\n", srv.ListenAndServe())
	}()
	signal.WithContextFunc(ctx, func() { srv.Shutdown(ctx); cancel() })
	<-ctx.Done()
	time.Sleep(3 * time.Second)
}
