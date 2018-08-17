// Command gracedemo implements a demo server showing how to gracefully
// terminate an HTTP server using grace.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/johntech-o/redeo"
	"github.com/johntech-o/redeo/resp"

	"os/signal"
	"syscall"

	"sync"

	"github.com/johntech-o/grace/gracenet"
)

var (
	address0 = flag.String("a0", ":48567", "Zero address to bind to.")
	address1 = flag.String("a1", ":48568", "First address to bind to.")
	address2 = flag.String("a2", ":48569", "Second address to bind to.")
	address3 = flag.String("a3", ":48570", "Third address to bind to.")
	now      = time.Now()
)

var mynet = gracenet.Net{}

var logger = log.New(os.Stderr, "", log.LstdFlags)

func signalHandler(server *redeo.Server, liss ...net.Listener) {
	ch := make(chan os.Signal, 10)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGUSR2)
	for {
		sig := <-ch
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			// this ensures a subsequent INT/TERM will trigger standard go behaviour of
			// terminating.
			signal.Stop(ch)
			println("receive signal TERM")
			println("start process")
			for _, lis := range liss {
				lis.Close()
			}
			timeout := time.After(time.Second * 5)
			tick := time.Tick(time.Second)
		Loop1:
			for {
				select {
				case <-tick:
					println("time", server.Info().TotalConnections())
					if server.Info().TotalConnections() == 0 {
						println("connection == 0 ")
						break Loop1
					}
				case <-timeout:
					println("time out graceful stop fail")
					break Loop1
				}
			}
			println("end start process")

			//		a.term(wg)
			return
		case syscall.SIGUSR2:
			//err := a.preStartProcess()
			//if err != nil {
			//	a.errors <- err
			//}
			// we only return here if there's an error, otherwise the new process
			// will send us a TERM when it's ready to trigger the actual shutdown.
			println("receive signal USR2")
			println("start process")
			count, err := mynet.StartProcess()
			if err != nil {
				panic(err)
			}
			for _, lis := range liss {
				lis.Close()
			}
			timeout := time.After(time.Second * 5)
			tick := time.Tick(time.Second)
		Loop:
			for {
				select {
				case <-tick:
					println("time", server.Info().TotalConnections())
					if server.Info().TotalConnections() == 0 {
						println("connection == 0 ")
						break Loop
					}
				case <-timeout:
					println("time out graceful restart fail")
					break Loop
				}
			}
			println("end start process")
			println(count, err)
		}
	}
}

func main() {
	println("start main", os.Getpid())
	flag.Parse()
	conf := &redeo.Config{
		Timeout:      time.Second * 30,
		IdleTimeout:  time.Second * 20,
		TCPKeepAlive: time.Second * 20,
	}
	server := redeo.NewServer(conf)
	server.HandleFunc("ping", ping)
	lis, err := mynet.Listen("tcp", *address3)
	if err != nil {
		panic(err)
	}
	lis2, err := mynet.Listen("tcp", *address0)
	if err != nil {
		println(err)
	}
	go signalHandler(server, lis, lis2)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := server.Serve(lis)
		if err != nil {
			println(err)
		}
		println("gateway done")
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		s1 := http.Server{Addr: *address0, Handler: newHandler("Zero  ")}

		s1.Serve(lis2)
		println("http server done")
	}()
	wg.Wait()
	println("end main", os.Getpid())

}

func ping(w resp.ResponseWriter, c *resp.Command) {
	time.Sleep(time.Millisecond * 5000)
	str := fmt.Sprintf("%s started at %s slept for %d nanoseconds from pid %d.\n", "redeo", now, 500, os.Getpid())
	w.AppendInlineString(str)
}

func newHandler(name string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/sleep/", func(w http.ResponseWriter, r *http.Request) {
		duration, err := time.ParseDuration(r.FormValue("duration"))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		time.Sleep(duration)
		fmt.Fprintf(
			w,
			"%s started at %s slept for %d nanoseconds from pid %d.\n",
			name,
			now,
			duration.Nanoseconds(),
			os.Getpid(),
		)
	})

	mux.HandleFunc("/stop/", func(w http.ResponseWriter, r *http.Request) {
		if err := syscall.Kill(os.Getpid(), syscall.SIGUSR2); err != nil {
			fmt.Errorf("failed to close parent: %s", err)
		}
		fmt.Fprintf(
			w,
			"%s started at %s slept for %d nanoseconds from pid %d.\n",
			name,
			now,
			now.Nanosecond(),
			os.Getpid(),
		)
	})
	return mux
}
