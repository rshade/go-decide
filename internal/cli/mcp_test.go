package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type mcpFake struct {
	hits   atomic.Int32
	status atomic.Int32
	answer atomic.Pointer[string]
	paths  sync.Map
}

func (f *mcpFake) set(body string) { f.answer.Store(&body) }

func (f *mcpFake) fail(status int, body string) {
	f.status.Store(int32(status))
	f.set(body)
}

type mcpServer struct {
	session *sdk.ClientSession
	fake    *mcpFake
	stderr  *lockedBuffer
	done    chan int
	cancel  context.CancelFunc
}

func (s *mcpServer) stop(t *testing.T) int {
	t.Helper()
	_ = s.session.Close()
	s.cancel()
	select {
	case code := <-s.done:
		return code
	case <-time.After(5 * time.Second):
		t.Fatal("mcp-server did not stop within 5s of cancellation")
		return -1
	}
}

func startMCP(t *testing.T, extra ...string) *mcpServer {
	t.Helper()
	fake := &mcpFake{}
	fake.set(choiceAnswer("ship", 0.95))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.hits.Add(1)
		fake.paths.Store(r.URL.Path, true)
		body := *fake.answer.Load()
		if code := int(fake.status.Load()); code != 0 {
			w.WriteHeader(code)
		} else if strings.Contains(r.URL.Path, "/ai/run/") {
			body = clefEnvelope(body)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TYPESAFE_API_KEY", "jev-secret-key")
	t.Setenv("TYPESAFE_BASE_URL", srv.URL)
	t.Setenv("TYPESAFE_LOG_LEVEL", "")
	t.Setenv("CLOUDFLARE_AUTH_TOKEN", "cf-secret-token")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", clefAccount)
	t.Setenv("CLOUDFLARE_BASE_URL", srv.URL)
	t.Setenv("CLOUDFLARE_LOG_LEVEL", "")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	stderr := &lockedBuffer{}
	done := make(chan int, 1)
	args := append([]string{"mcp-server", "--transport", "http", "--addr", addr}, extra...)
	go func() {
		done <- Run(ctx, args, Env{
			Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: stderr,
			Getenv: func(string) string { return "" }, Version: goldenVersion,
		})
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("mcp-server did not listen on %s; stderr: %s", addr, stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "v0.0.0-test"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: "http://" + addr, DisableStandaloneSSE: true}, nil)
	if err != nil {
		cancel()
		t.Fatalf("connecting to mcp-server: %v", err)
	}
	s := &mcpServer{session: session, fake: fake, stderr: stderr, done: done, cancel: cancel}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
	return s
}

func (s *mcpServer) call(t *testing.T, tool string, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	res, err := s.session.CallTool(context.Background(), &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("calling %s: %v", tool, err)
	}
	return res
}

func resultText(t *testing.T, res *sdk.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*sdk.TextContent); ok {
			sb.WriteString(text.Text)
		}
	}
	return sb.String()
}

func askArgs() map[string]any {
	return map[string]any{
		"state":        "all checks passed",
		"instructions": "Ship it?",
		"option":       []string{"ship=release it", "hold=wait"},
	}
}

func TestMCPListsAskAndScoreButNotEval(t *testing.T) {
	s := startMCP(t)
	res, err := s.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	if !names["go-decide-ask"] || !names["go-decide-score"] {
		t.Errorf("tools = %v, want go-decide-ask and go-decide-score", names)
	}
	for _, forbidden := range []string{"go-decide-eval", "eval", "mcp-server", "go-decide-mcp-server", "__schema"} {
		if names[forbidden] {
			t.Errorf("tool %q is listed, want it absent", forbidden)
		}
	}
	if _, err := s.session.CallTool(context.Background(), &sdk.CallToolParams{Name: "go-decide-eval"}); err == nil {
		t.Error("calling eval succeeded, want an unknown tool failure")
	}
}

func TestMCPOutcomesAreResultsNotErrors(t *testing.T) {
	for _, tt := range []struct {
		outcome    string
		confidence float64
	}{
		{"decided", 0.95},
		{"uncertain", 0.7},
		{"escalate", 0.3},
	} {
		t.Run(tt.outcome, func(t *testing.T) {
			s := startMCP(t)
			s.fake.set(choiceAnswer("ship", tt.confidence))
			res := s.call(t, "go-decide-ask", askArgs())
			if res.IsError {
				t.Fatalf("%s outcome is flagged as an error: %s", tt.outcome, resultText(t, res))
			}
			var env envelope[AskOutput]
			if err := json.Unmarshal([]byte(resultText(t, res)), &env); err != nil {
				t.Fatalf("decoding %q: %v", resultText(t, res), err)
			}
			if env.Data.Outcome != tt.outcome {
				t.Errorf("outcome = %q, want %q", env.Data.Outcome, tt.outcome)
			}
			if code := s.stop(t); code != 0 {
				t.Errorf("server exit code = %d after a %s outcome, want 0", code, tt.outcome)
			}
		})
	}
}

func TestMCPValidationFailureIsAnErrorAndSendsNothing(t *testing.T) {
	s := startMCP(t)
	args := askArgs()
	args["state"] = ""
	res := s.call(t, "go-decide-ask", args)
	if !res.IsError {
		t.Fatalf("empty state was not flagged as an error: %s", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), `"error_code":"validation_error"`) {
		t.Errorf("error envelope %q does not carry the validation_error code", resultText(t, res))
	}
	if hits := s.fake.hits.Load(); hits != 0 {
		t.Errorf("%d requests reached the backend, want 0", hits)
	}
}

func TestMCPDryRunSendsNothing(t *testing.T) {
	s := startMCP(t)
	args := askArgs()
	args["dry-run"] = true
	res := s.call(t, "go-decide-ask", args)
	if res.IsError || !strings.Contains(resultText(t, res), `"dry_run":true`) {
		t.Fatalf("dry run result = %q (error %v)", resultText(t, res), res.IsError)
	}
	if hits := s.fake.hits.Load(); hits != 0 {
		t.Errorf("%d requests reached the backend on a dry run, want 0", hits)
	}
}

func TestMCPBackendIsAnInput(t *testing.T) {
	s := startMCP(t)
	args := askArgs()
	args["backend"] = "clef"
	res := s.call(t, "go-decide-ask", args)
	if res.IsError || !strings.Contains(resultText(t, res), `"backend":"clef"`) {
		t.Fatalf("clef result = %q (error %v)", resultText(t, res), res.IsError)
	}
	if _, ok := s.fake.paths.Load("/v1/systemone"); ok {
		t.Error("a request reached the Jev path for a clef call")
	}
}

func TestMCPMissingClefCredentialsNeverFallBackToJev(t *testing.T) {
	s := startMCP(t)
	t.Setenv("CLOUDFLARE_AUTH_TOKEN", "")
	args := askArgs()
	args["backend"] = "clef"
	res := s.call(t, "go-decide-ask", args)
	if !res.IsError || !strings.Contains(resultText(t, res), "CLOUDFLARE_AUTH_TOKEN") {
		t.Fatalf("result = %q (error %v), want the clef configuration error", resultText(t, res), res.IsError)
	}
	if hits := s.fake.hits.Load(); hits != 0 {
		t.Errorf("%d requests reached a backend, want 0", hits)
	}
}

func TestMCPSecretsNeverAppearInResultsOrLogs(t *testing.T) {
	for _, tt := range []struct {
		name, backend, secret, body string
	}{
		{"jev", "jev", "jev-secret-key", `{"error":{"message":"invalid key jev-secret-key"}}`},
		{"clef", "clef", "cf-secret-token", `{"errors":[{"code":10000,"message":"auth failed cf-secret-token"}],"success":false,"result":null,"messages":[]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := startMCP(t)
			s.fake.fail(http.StatusUnauthorized, tt.body)
			args := askArgs()
			args["backend"] = tt.backend
			res := s.call(t, "go-decide-ask", args)
			text := resultText(t, res)
			if !res.IsError || !strings.Contains(text, "[REDACTED]") {
				t.Fatalf("result = %q (error %v), want an error that shows [REDACTED]", text, res.IsError)
			}
			if strings.Contains(text, tt.secret) || strings.Contains(s.stderr.String(), tt.secret) {
				t.Errorf("secret %q appears in a result or the server log", tt.secret)
			}
		})
	}
}

func TestMCPSpecFromStandardInputIsRefused(t *testing.T) {
	s := startMCP(t)
	done := make(chan *sdk.CallToolResult, 1)
	go func() {
		done <- s.call(t, "go-decide-ask", map[string]any{"spec": "-"})
	}()
	select {
	case res := <-done:
		if !res.IsError {
			t.Errorf("--spec - succeeded over MCP: %s", resultText(t, res))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("--spec - hung over MCP")
	}
	if hits := s.fake.hits.Load(); hits != 0 {
		t.Errorf("%d requests reached the backend, want 0", hits)
	}
}

func TestEvalStillRunsOnTheCommandLine(t *testing.T) {
	r := execute(t, "", []string{"eval", "--dry-run", "--decisions", "../../testdata/decisions.json", "--truth", "../../testdata/decisions_truth.json"}, http.StatusOK, "")
	if r.code != 0 {
		t.Fatalf("eval --dry-run exit %d: %s", r.code, r.stderr)
	}
}

func TestMCPServerRefusesAnUnsafeStart(t *testing.T) {
	t.Run("non-loopback bind", func(t *testing.T) {
		var stderr lockedBuffer
		code := Run(context.Background(), []string{"mcp-server", "--transport", "http", "--addr", "0.0.0.0:0"}, Env{
			Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &stderr,
			Getenv: func(string) string { return "" }, Version: goldenVersion,
		})
		if code != 2 {
			t.Errorf("exit = %d, want 2; stderr: %s", code, stderr.String())
		}
	})
}

func TestMCPServerReportsTheResolvedVersion(t *testing.T) {
	s := startMCP(t)
	init := s.session.InitializeResult()
	if init == nil || init.ServerInfo == nil || init.ServerInfo.Version != goldenVersion {
		t.Errorf("server info = %+v, want version %q", init, goldenVersion)
	}
}

func TestMCPOverlappingCallsDoNotRace(t *testing.T) {
	s := startMCP(t)
	s.fake.set(choiceAnswer("ship", 0.7))
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res := s.call(t, "go-decide-ask", askArgs()); res.IsError {
				t.Errorf("overlapping call flagged as an error: %s", resultText(t, res))
			}
		}()
	}
	wg.Wait()
	if code := s.stop(t); code != 0 {
		t.Errorf("server exit code = %d after overlapping uncertain outcomes, want 0", code)
	}
}

func TestMCPServesOverStandardInputAndOutputByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary; skipped with -short")
	}
	bin := filepath.Join(t.TempDir(), "go-decide")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v0.0.0-stdio-test", "-o", bin, "../../cmd/go-decide")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the binary: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "mcp-server")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(stdout)
	lines.Buffer(make([]byte, 1<<20), 1<<20)
	send := func(msg string) { _, _ = io.WriteString(stdin, msg+"\n") }
	next := func() string {
		got := make(chan string, 1)
		go func() {
			if lines.Scan() {
				got <- lines.Text()
			} else {
				got <- ""
			}
		}()
		select {
		case line := <-got:
			return line
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatal("no reply from mcp-server on standard output")
			return ""
		}
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"stdio-test","version":"0"}}}`)
	if reply := next(); !strings.Contains(reply, "v0.0.0-stdio-test") {
		t.Errorf("initialize reply %q does not carry the stamped version", reply)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	reply := next()
	if !strings.Contains(reply, "go-decide-ask") || !strings.Contains(reply, "go-decide-score") || strings.Contains(reply, "go-decide-eval") {
		t.Errorf("tools/list reply %q, want ask and score and no eval", reply)
	}
	_ = stdin.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("mcp-server exited with %v after its input closed, want 0", err)
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Error("mcp-server did not exit after its input closed")
	}
}
