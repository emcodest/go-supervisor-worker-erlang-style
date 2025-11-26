package workerpool

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emcodest/go-supervisor"
)

var (
	ErrQueueFull = errors.New("workerpool: queue is full")
)

type Task struct {
	Fn      func(ctx context.Context) error
	Retries int
	Timeout time.Duration
	result  chan error
}

type Pool struct {
	size    int
	queue   chan *Task
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started int32
	cfg     supervisor.Config
	logger  *log.Logger

	completed uint64
	failed    uint64
}

func New(size int, queueSize int) *Pool {
	if size <= 0 {
		size = 1
	}
	if queueSize <= 0 {
		queueSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		size:   size,
		queue:  make(chan *Task, queueSize),
		ctx:    ctx,
		cancel: cancel,
		cfg:    supervisor.DefaultConfig(),
		logger: supervisor.DefaultConfig().Logger,
	}

	p.start()
	return p
}

func (p *Pool) start() {
	if !atomic.CompareAndSwapInt32(&p.started, 0, 1) {
		return
	}

	for i := 0; i < p.size; i++ {
		p.wg.Add(1)
		idx := i
		p.cfg.Logger.Printf("[workerpool] starting worker %d", idx)
		supervisor.Start(p.ctx, p.cfg, func(ctx context.Context) {
			defer p.wg.Done()
			p.workerLoop(ctx, idx)
		})
	}
}

func (p *Pool) workerLoop(ctx context.Context, idx int) {
	p.cfg.Logger.Printf("[workerpool] worker %d running", idx)
	for {
		select {
		case <-ctx.Done():
			p.cfg.Logger.Printf("[workerpool] worker %d shutdown", idx)
			return
		case t := <-p.queue:
			p.executeTask(ctx, t)
		}
	}
}

func (p *Pool) executeTask(ctx context.Context, t *Task) {
	try := 0
	for {
		try++
		tctx := ctx
		var cancel context.CancelFunc
		if t.Timeout > 0 {
			tctx, cancel = context.WithTimeout(ctx, t.Timeout)
		} else {
			tctx, cancel = context.WithCancel(ctx)
		}

		errCh := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					p.cfg.Logger.Printf("[workerpool] task panic: %v", r)
					errCh <- errors.New("panic in task")
				}
			}()
			errCh <- t.Fn(tctx)
		}()

		select {
		case <-tctx.Done():
			cancel()
			atomic.AddUint64(&p.failed, 1)
			if try-1 < t.Retries {
				continue
			}
			if t.result != nil {
				t.result <- tctx.Err()
			}
			return
		case err := <-errCh:
			cancel()
			if err != nil {
				atomic.AddUint64(&p.failed, 1)
				if try-1 < t.Retries {
					continue
				}
				if t.result != nil {
					t.result <- err
				}
				return
			}
			atomic.AddUint64(&p.completed, 1)
			if t.result != nil {
				t.result <- nil
			}
			return
		}
	}
}

func (p *Pool) Submit(fn func(ctx context.Context) error) error {
	return p.SubmitWithOptions(fn, 0, 0)
}

func (p *Pool) SubmitWithOptions(fn func(ctx context.Context) error, retries int, timeout time.Duration) error {
	t := &Task{Fn: fn, Retries: retries, Timeout: timeout}
	select {
	case p.queue <- t:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

func (p *Pool) TrySubmit(fn func(ctx context.Context) error) error {
	return p.TrySubmitWithOptions(fn, 0, 0)
}

func (p *Pool) TrySubmitWithOptions(fn func(ctx context.Context) error, retries int, timeout time.Duration) error {
	t := &Task{Fn: fn, Retries: retries, Timeout: timeout}
	select {
	case p.queue <- t:
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *Pool) SubmitAndWait(fn func(ctx context.Context) error, retries int, timeout time.Duration) error {
	t := &Task{Fn: fn, Retries: retries, Timeout: timeout, result: make(chan error, 1)}
	select {
	case p.queue <- t:
		select {
		case err := <-t.result:
			return err
		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

func (p *Pool) Shutdown() {
	p.cancel()
	p.wg.Wait()
}

func (p *Pool) Completed() uint64 { return atomic.LoadUint64(&p.completed) }
func (p *Pool) Failed() uint64    { return atomic.LoadUint64(&p.failed) }
