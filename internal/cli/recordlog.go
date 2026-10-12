package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	ax "github.com/rshade/ax-go"
)

// decideLogEnv names the decision log. Recording is off while it is unset or
// empty, and nothing else can choose the file.
const decideLogEnv = "DECIDE_LOG"

const logFormatVersion = 1

const (
	kindDecision = "decision"
	kindRun      = "run"
	kindOutcome  = "outcome"
)

// maxIDDraws bounds how often a clashing decision id is redrawn.
const maxIDDraws = 100

// logLine is one line of the decision log. The kind decides which of the
// optional parts it carries; they stay raw so a line reads back as written.
type logLine struct {
	V            int             `json:"v"`
	Kind         string          `json:"kind"`
	ID           string          `json:"id"`
	RecordedAt   string          `json:"recorded_at"`
	TraceID      string          `json:"trace_id"`
	Instructions string          `json:"instructions,omitempty"`
	Decision     json.RawMessage `json:"decision,omitempty"`
	Path         string          `json:"path,omitempty"`
	PreScreen    json.RawMessage `json:"pre_screen,omitempty"`
	Debate       json.RawMessage `json:"debate,omitempty"`
	Truth        json.RawMessage `json:"truth,omitempty"`
}

// recorder appends to the decision log named by DECIDE_LOG. One recorder is
// shared by every record command of a process, so its mutex serialises the
// read, check and append of concurrent MCP calls.
type recorder struct {
	mu  sync.Mutex
	env Env
}

func newRecorder(env Env) *recorder {
	return &recorder{env: env.withClock()}
}

func (r *recorder) path() string { return r.env.Getenv(decideLogEnv) }

func (r *recorder) on() bool { return r.path() != "" }

// commit reads the log, lets build check it and produce the next line, and
// appends that line with one append-mode write. A log inside a git working
// tree is written only with --yes. Every failure is returned; nothing is
// reported as written unless the write and the close succeeded.
func (r *recorder) commit(ctx context.Context, build func([]logLine) (logLine, error)) (logLine, error) {
	path := r.path()
	if err := guardWorkTree(ctx, path); err != nil {
		return logLine{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	line, err := r.readAndBuild(ctx, build)
	if err != nil {
		return logLine{}, err
	}
	line.V = logFormatVersion
	line.RecordedAt = r.env.Now().UTC().Format(time.RFC3339)
	line.TraceID = ax.TraceIDFromContext(ctx)

	raw, err := json.Marshal(line)
	if err != nil {
		return logLine{}, writeFailure(ctx, err)
	}
	if err := appendLine(path, append(raw, '\n')); err != nil {
		return logLine{}, writeFailure(ctx, err)
	}
	return line, nil
}

// validate is the dry run of commit: it applies the same work tree check,
// reads the log and lets build check it, and writes nothing.
func (r *recorder) validate(ctx context.Context, build func([]logLine) (logLine, error)) error {
	if err := guardWorkTree(ctx, r.path()); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err := r.readAndBuild(ctx, build)
	return err
}

// logError reports a log that cannot be read or does not parse, naming DECIDE_LOG.
func logError(ctx context.Context, err error) error {
	return newValidationError(ctx, fmt.Sprintf("%s: %v", decideLogEnv, err), decideLogEnv)
}

// readAndBuild reads the log and passes it to build. The caller holds the lock.
func (r *recorder) readAndBuild(ctx context.Context, build func([]logLine) (logLine, error)) (logLine, error) {
	lines, err := readLogLines(r.path())
	if err != nil {
		return logLine{}, logError(ctx, err)
	}
	return build(lines)
}

// read returns every line of the log, under the same lock as commit, so a
// read never sees half of a line another call is appending.
func (r *recorder) read() ([]logLine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return readLogLines(r.path())
}

func appendLine(path string, raw []byte) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); err == nil {
			err = cerr
		}
	}()
	_, err = file.Write(raw)
	return err
}

func writeFailure(ctx context.Context, err error) error {
	return ax.NewError(ctx, "internal_error", "writing the decision log: "+err.Error(),
		ax.WithErrorExitCode(ax.ExitInternal))
}

// newDecisionID draws the UTC date and four hex characters, again while the
// id is already on a decision line.
func (r *recorder) newDecisionID(lines []logLine) (string, error) {
	taken := map[string]bool{}
	for _, l := range lines {
		if l.Kind == kindDecision {
			taken[l.ID] = true
		}
	}
	date := r.env.Now().UTC().Format(time.DateOnly)
	suffix := make([]byte, 2)
	for range maxIDDraws {
		if _, err := io.ReadFull(r.env.Rand, suffix); err != nil {
			return "", err
		}
		if id := date + "-" + hex.EncodeToString(suffix); !taken[id] {
			return id, nil
		}
	}
	return "", fmt.Errorf("no free decision id for %s after %d draws", date, maxIDDraws)
}

// readLogLines reads every line of the log at path. A missing file is an empty
// log; a line that is not a version 1 record fails the read with its number.
func readLogLines(path string) ([]logLine, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var lines []logLine
	reader := bufio.NewReader(io.LimitReader(file, maxEvalFileBytes+1))
	read := 0
	for n := 1; ; n++ {
		raw, err := reader.ReadBytes('\n')
		read += len(raw)
		if read > maxEvalFileBytes {
			return nil, fmt.Errorf("the log is larger than %d bytes", maxEvalFileBytes)
		}
		if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 {
			var line logLine
			if jerr := json.Unmarshal(trimmed, &line); jerr != nil {
				return nil, fmt.Errorf("line %d is not a JSON record: %w", n, jerr)
			}
			if line.V != logFormatVersion || line.ID == "" {
				return nil, fmt.Errorf("line %d is not a version %d record with an id", n, logFormatVersion)
			}
			lines = append(lines, line)
		}
		if errors.Is(err, io.EOF) {
			return lines, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// guardWorkTree refuses a log inside a git working tree unless the caller
// passed --yes. It looks from the nearest directory on the path that exists,
// so a directory that has not been created yet cannot hide the work tree.
func guardWorkTree(ctx context.Context, path string) error {
	if !insideWorkTree(path) {
		return nil
	}
	subject := "the decision log " + path + " is inside a git working tree, where it could be committed"
	outcome, err := ax.Confirm(ctx, subject)
	switch {
	case outcome == ax.ConfirmationApproved:
		return nil
	case err != nil:
		return err
	default:
		return newValidationError(ctx, "confirmation required: "+subject+"; pass --yes to write it there", "yes")
	}
}

func insideWorkTree(path string) bool {
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return false
	}
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
