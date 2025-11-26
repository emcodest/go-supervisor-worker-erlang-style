# workerpool

A small, reliable, production-ready worker pool for Go built on top of a supervisor.

Features:

- Fixed number of supervised workers (auto-restart on panic)
- Task queue with configurable size
- Submit, SubmitWithTimeout, SubmitWithRetry, TrySubmit (non-blocking)
- Per-task deadlines and retries
- Graceful shutdown
- Metrics (completed/failed counts)

## Installation

```bash
go get github.com/emcodest/go-supervisor-worker-erlang-style
