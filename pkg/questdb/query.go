package questdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// QueryClient calls QuestDB's HTTP /exec endpoint. The official Go
// client only does ILP writes; reads — daily roll-ups, ad-hoc
// SELECTs — go through /exec by hand. This client wraps that with
// auth, timeout, and JSON decoding.
//
// QuestDB does NOT accept parameter binding on /exec; callers must
// quote literals into the SQL themselves. Use time.Time.UTC() +
// the documented "2006-01-02T15:04:05.000000Z" format for timestamps.
type QueryClient struct {
	baseURL string
	cli     *http.Client
	token   string
	user    string
	pass    string
}

// NewQueryClient builds a QueryClient from cfg. The HTTPBaseURL is
// used directly if set; otherwise http(s)://Address is synthesized
// from Address + TLS.
func NewQueryClient(cfg Config) *QueryClient {
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &QueryClient{
		baseURL: cfg.httpBaseURL(),
		cli:     &http.Client{Timeout: timeout},
		token:   cfg.Token,
		user:    cfg.BasicAuthUser,
		pass:    cfg.BasicAuthPassword,
	}
}

// ExecResponse is QuestDB's /exec JSON shape (subset). Dataset
// rows are positional — order matches the SELECT — and numeric
// fields arrive as JSON numbers (float64) or strings for large
// LONGs. The caller is responsible for type-asserting each cell.
type ExecResponse struct {
	Query   string       `json:"query"`
	Columns []ExecColumn `json:"columns"`
	Dataset [][]any      `json:"dataset"`
	Count   int          `json:"count"`
	Error   string       `json:"error"`
}

// ExecColumn names a column in the SELECT result.
type ExecColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Exec runs an arbitrary SQL statement against /exec and returns
// the decoded response. Returns an error if the HTTP status is
// non-2xx, the body fails to decode, or QuestDB reports a
// query-level error (resp.Error populated).
func (q *QueryClient) Exec(ctx context.Context, sql string) (*ExecResponse, error) {
	u, err := url.Parse(q.baseURL)
	if err != nil {
		return nil, fmt.Errorf("questdb: invalid base url: %w", err)
	}
	u.Path = "/exec"
	qs := u.Query()
	qs.Set("query", sql)
	u.RawQuery = qs.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if q.token != "" {
		req.Header.Set("Authorization", "Bearer "+q.token)
	} else if q.user != "" {
		req.SetBasicAuth(q.user, q.pass)
	}

	httpResp, err := q.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("questdb: http: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("questdb: read body: %w", err)
	}
	if httpResp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("questdb: http %d: %s",
			httpResp.StatusCode, truncate(string(body), 500))
	}

	var resp ExecResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("questdb: decode: %w (body=%s)",
			err, truncate(string(body), 500))
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("questdb: query error: %s", resp.Error)
	}
	return &resp, nil
}

// Ping runs `SELECT 1` to verify connectivity and auth. Useful in
// startup health checks so misconfiguration fails fast.
func (q *QueryClient) Ping(ctx context.Context) error {
	_, err := q.Exec(ctx, "SELECT 1")
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
