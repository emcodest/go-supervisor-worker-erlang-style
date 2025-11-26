package main

import (
	"context"
	"log"
	"time"

	workerpool "github.com/emcodest/go-supervisor-worker-erlang-style"
)

func main() {
	p := workerpool.New(5, 50)
	defer p.Shutdown()

	for i := 0; i < 20; i++ {
		i := i
		_ = p.Submit(func(ctx context.Context) error {
			log.Println("job", i, "started")
			time.Sleep(5000 * time.Millisecond)
			log.Println("job", i, "done")
			return nil
		})
	}

	time.Sleep(3 * time.Second)
	log.Println("exiting")
}
