// internal/services/job_queue.go
package services

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrJobQueueStopped   = errors.New("job queue stopped")
	ErrTaskAlreadyExists = errors.New("task already exists")
)

type JobFunc func(ctx context.Context) error

type queuedJob struct {
	taskID string
	ctx    context.Context
	fn     JobFunc
}

type taskState struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// JobQueue 是一个最小 worker pool：支持 Submit/Cancel/Wait，用 context 贯穿取消。
// 设计目标：Phase1 先保证可用闭环；重试/持久化/优先级等后续再加。
type JobQueue struct {
	ctx    context.Context
	cancel context.CancelFunc

	jobs chan queuedJob
	wg   sync.WaitGroup

	mu      sync.RWMutex
	tasks   map[string]*taskState
	stopped bool
}

func NewJobQueue(workerCount int, queueSize int) *JobQueue {
	if workerCount <= 0 {
		workerCount = 1
	}
	if queueSize <= 0 {
		queueSize = workerCount * 8
	}

	ctx, cancel := context.WithCancel(context.Background())
	q := &JobQueue{
		ctx:    ctx,
		cancel: cancel,
		jobs:   make(chan queuedJob, queueSize),
		tasks:  make(map[string]*taskState),
	}

	for i := 0; i < workerCount; i++ {
		q.wg.Add(1)
		go q.worker()
	}

	return q
}

func (q *JobQueue) worker() {
	defer q.wg.Done()

	for job := range q.jobs {
		func() {
			defer func() {
				q.mu.Lock()
				st := q.tasks[job.taskID]
				if st != nil {
					select {
					case <-st.done:
					default:
						close(st.done)
					}
				}
				delete(q.tasks, job.taskID)
				q.mu.Unlock()
			}()

			if job.fn != nil {
				_ = job.fn(job.ctx)
			}
		}()
	}
}

func (q *JobQueue) Submit(taskID string, fn JobFunc) error {
	if taskID == "" {
		return errors.New("taskID required")
	}

	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return ErrJobQueueStopped
	}
	if _, exists := q.tasks[taskID]; exists {
		q.mu.Unlock()
		return ErrTaskAlreadyExists
	}

	taskCtx, taskCancel := context.WithCancel(q.ctx)
	q.tasks[taskID] = &taskState{cancel: taskCancel, done: make(chan struct{})}
	jobs := q.jobs
	q.mu.Unlock()

	// Enqueue outside the lock to avoid deadlocks when the queue is full.
	// If the queue is concurrently stopped/closed, roll back the task state.
	defer func() {
		if r := recover(); r != nil {
			q.mu.Lock()
			st := q.tasks[taskID]
			if st != nil {
				select {
				case <-st.done:
				default:
					close(st.done)
				}
				delete(q.tasks, taskID)
				st.cancel()
			}
			q.mu.Unlock()
		}
	}()

	select {
	case jobs <- queuedJob{taskID: taskID, ctx: taskCtx, fn: fn}:
		return nil
	case <-q.ctx.Done():
		q.mu.Lock()
		st := q.tasks[taskID]
		if st != nil {
			select {
			case <-st.done:
			default:
				close(st.done)
			}
			delete(q.tasks, taskID)
			st.cancel()
		}
		q.mu.Unlock()
		return ErrJobQueueStopped
	}
}

func (q *JobQueue) Cancel(taskID string) bool {
	q.mu.RLock()
	st := q.tasks[taskID]
	q.mu.RUnlock()
	if st == nil {
		return false
	}
	st.cancel()
	return true
}

func (q *JobQueue) Wait(ctx context.Context, taskID string) error {
	q.mu.RLock()
	st := q.tasks[taskID]
	q.mu.RUnlock()
	if st == nil {
		return nil
	}

	select {
	case <-st.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *JobQueue) Stop() {
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return
	}
	q.stopped = true
	q.mu.Unlock()

	q.cancel()
	close(q.jobs)
	q.wg.Wait()
}
