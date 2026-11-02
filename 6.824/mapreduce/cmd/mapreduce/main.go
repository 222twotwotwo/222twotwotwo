package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	mr "example.com/minimapreduce"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mapreduce <coordinator|worker> [flags] [input-files]")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	switch args[0] {
	case "coordinator":
		addr := flags.String("addr", "127.0.0.1:9000", "TCP listen address")
		nReduce := flags.Int("reduce", 3, "number of reduce tasks")
		out := flags.String("out", "data/result", "new output directory (must not exist)")
		timeout := flags.Duration("timeout", mr.DefaultTaskTimeout, "task timeout before reassignment")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() == 0 {
			return fmt.Errorf("at least one input file or glob is required")
		}
		var files []string
		for _, pattern := range flags.Args() {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return err
			}
			if len(matches) == 0 {
				return fmt.Errorf("no input files match %q", pattern)
			}
			files = append(files, matches...)
		}
		listener, err := net.Listen("tcp", *addr)
		if err != nil {
			return err
		}
		defer listener.Close()
		coordinator, err := mr.NewCoordinator(files, *nReduce, *out, *timeout)
		if err != nil {
			return err
		}
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			<-runCtx.Done()
			listener.Close()
		}()
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return
				case <-ticker.C:
					if coordinator.Done() {
						log.Printf("job complete: %s/mr-out-*; press Ctrl+C to stop coordinator", *out)
						return
					}
				}
			}
		}()
		log.Printf("coordinator listening on %s; maps=%d reduces=%d", listener.Addr(), len(files), *nReduce)
		if err := coordinator.Serve(listener); ctx.Err() == nil {
			return err
		}
		return ctx.Err()
	case "worker":
		addr := flags.String("coordinator", "127.0.0.1:9000", "coordinator TCP address")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("usage: mapreduce worker [-coordinator host:port]")
		}
		return mr.Worker(ctx, *addr, mr.WordCountMap, mr.WordCountReduce)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
