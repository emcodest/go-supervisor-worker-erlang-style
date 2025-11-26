package workerpool

import (
    "context"
    "errors"
    "sync/atomic"
    "testing"
    "time"
)

func TestSubmitAndExecute(t *testing.T) {
    p := New(2, 10)
    defer p.Shutdown()

    var ran int32

    err := p.SubmitAndWait(func(ctx context.Context) error {
        atomic.StoreInt32(&ran, 1)
        return nil
    }, 0, 0)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if atomic.LoadInt32(&ran) != 1 {
        t.Fatalf("task did not run")
    }
}

func TestRetryOnFailure(t *testing.T) {
    p := New(1, 10)
    defer p.Shutdown()

    var tries int32

    err := p.SubmitAndWait(func(ctx context.Context) error {
        if atomic.AddInt32(&tries, 1) < 3 {
            return errors.New("fail")
        }
        return nil
    }, 5, 0)

    if err != nil {
        t.Fatalf("expected success after retries, got %v", err)
    }
}

func TestTrySubmitFullQueue(t *testing.T) {
    p := New(1, 1)
    defer p.Shutdown()

    _ = p.Submit(func(ctx context.Context) error { time.Sleep(100 * time.Millisecond); return nil })

    err := p.TrySubmit(func(ctx context.Context) error { return nil })
    if err != ErrQueueFull {
        t.Fatalf("expected ErrQueueFull, got %v", err)
    }
}

func TestShutdownStopsWorkers(t *testing.T) {
    p := New(1, 10)

    var ran int32
    _ = p.Submit(func(ctx context.Context) error {
        atomic.StoreInt32(&ran, 1)
        return nil
    })

    p.Shutdown()
    if atomic.LoadInt32(&ran) != 1 {
        t.Fatalf("task did not run before shutdown")
    }
}
