package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	gfs "example.com/minigfs"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gfs <master|chunkserver|put|get|ls> [flags] [arguments]")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	switch args[0] {
	case "master":
		addr := flags.String("addr", "127.0.0.1:8000", "listen address")
		servers := flags.String("servers", "http://127.0.0.1:8001,http://127.0.0.1:8002", "comma-separated chunkserver URLs")
		replicas := flags.Int("replicas", 2, "replicas per chunk")
		chunkSize := flags.Int("chunk-size", 1<<20, "chunk size in bytes")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		master, err := gfs.NewMaster(strings.Split(*servers, ","), *chunkSize, *replicas)
		if err != nil {
			return err
		}
		return serve(*addr, master.Handler())
	case "chunkserver":
		addr := flags.String("addr", "127.0.0.1:8001", "listen address")
		dir := flags.String("dir", "data/chunk1", "chunk storage directory")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		handler, err := gfs.NewChunkServer(*dir)
		if err != nil {
			return err
		}
		return serve(*addr, handler)
	case "put", "get", "ls":
		master := flags.String("master", "http://127.0.0.1:8000", "master URL")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		client := gfs.NewClient(*master)
		ctx := context.Background()
		if args[0] == "ls" {
			if flags.NArg() != 0 {
				return fmt.Errorf("usage: gfs ls [-master URL]")
			}
			files, err := client.List(ctx)
			if err != nil {
				return err
			}
			for _, file := range files {
				fmt.Printf("%d\t%s\n", file.Size, file.Name)
			}
			return nil
		}
		if flags.NArg() != 2 {
			return fmt.Errorf("usage: gfs put [-master URL] <local-file> <gfs-name>; gfs get [-master URL] <gfs-name> <local-file>")
		}
		if args[0] == "put" {
			data, err := os.ReadFile(flags.Arg(0))
			if err != nil {
				return err
			}
			return client.Put(ctx, flags.Arg(1), data)
		}
		data, err := client.Get(ctx, flags.Arg(0))
		if err != nil {
			return err
		}
		return os.WriteFile(flags.Arg(1), data, 0644)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func serve(addr string, handler http.Handler) error {
	log.Printf("listening on http://%s", addr)
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	return server.ListenAndServe()
}
