package titles

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/regutierrez/traicr/internal/store"
)

// Namer produces a title for a trace excerpt; *Generator is the real one.
type Namer interface {
	Generate(ctx context.Context, traceID int64, prompt string) (string, error)
}

// Worker names untitled traces one at a time.
type Worker struct {
	Store  *store.Store
	Namer  Namer
	Logger *slog.Logger
	// Idle is how long a trace must go without a new revision before it is
	// named, so live sessions are not named mid-conversation.
	Idle time.Duration
	// Pause is the wait between model requests.
	Pause time.Duration
	// Poll is the wait before looking again when nothing needs a title.
	Poll time.Duration
	Now  func() time.Time

	// accepted is set after the API first returns a title. Until then a
	// rejection may be a configuration problem, so it is not blamed on the trace.
	accepted bool
}

const (
	batchSize  = 20
	minBackoff = time.Minute
	maxBackoff = 30 * time.Minute
)

// NewWorker returns a worker with the production timings.
func NewWorker(database *store.Store, namer Namer, logger *slog.Logger) *Worker {
	return &Worker{Store: database, Namer: namer, Logger: logger, Idle: 30 * time.Minute, Pause: 2 * time.Second, Poll: 5 * time.Minute, Now: time.Now}
}

// Run names traces until ctx is cancelled, backing off while the API fails.
func (w *Worker) Run(ctx context.Context) {
	backoff := minBackoff
	for {
		named, err := w.RunOnce(ctx)
		wait := w.Poll
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			wait = backoff
			var requestErr *RequestError
			if errors.As(err, &requestErr) && requestErr.RetryAfter > wait {
				wait = requestErr.RetryAfter
			}
			backoff = min(backoff*2, maxBackoff)
			w.Logger.Error("trace naming paused", "error", err.Error(), "retry_in", wait)
		case named > 0:
			backoff = minBackoff
			wait = 0
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// RunOnce names one batch of candidates and reports how many it recorded. It
// stops at the first error that would also fail for the next trace.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	candidates, err := w.Store.TitleCandidates(ctx, w.Now().Add(-w.Idle), batchSize)
	if err != nil {
		return 0, err
	}
	recorded := 0
	for i, candidate := range candidates {
		if i > 0 && w.Pause > 0 {
			select {
			case <-ctx.Done():
				return recorded, ctx.Err()
			case <-time.After(w.Pause):
			}
		}
		title, err := w.name(ctx, candidate.TraceID)
		if err != nil {
			return recorded, fmt.Errorf("trace %d: %w", candidate.TraceID, err)
		}
		if err := w.Store.SetGeneratedTitle(ctx, candidate.TraceID, candidate.RevisionID, title); err != nil {
			return recorded, err
		}
		recorded++
		if title == "" {
			w.Logger.Info("trace left untitled", "trace_id", candidate.TraceID, "revision_id", candidate.RevisionID)
		} else {
			w.Logger.Info("trace titled", "trace_id", candidate.TraceID, "revision_id", candidate.RevisionID)
		}
	}
	return recorded, nil
}

// name returns "" for a trace that cannot be named, so it is recorded and skipped.
func (w *Worker) name(ctx context.Context, traceID int64) (string, error) {
	source, err := w.Store.TitleSource(ctx, traceID)
	if err != nil {
		return "", err
	}
	prompt := Prompt(source)
	if prompt == "" {
		return "", nil
	}
	title, err := w.Namer.Generate(ctx, traceID, prompt)
	var requestErr *RequestError
	if err != nil && w.accepted && errors.As(err, &requestErr) && requestErr.Rejected() {
		w.Logger.Warn("title API rejected trace", "trace_id", traceID, "status", requestErr.Status)
		return "", nil
	}
	if err != nil {
		return "", err
	}
	w.accepted = true
	return title, nil
}
