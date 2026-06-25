package confirm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BackendStore is the production Store (ADR-057 R1): it persists confirmations
// through supkube-backend's /api/v1/mcp/confirmations API, which stores them as
// cross-replica K8s Secrets. The mcp-server thus stays a pure backend client
// (no K8s deps). Bearer = the server→backend service token.
type BackendStore struct {
	BaseURL string
	Token   string
	HC      *http.Client
}

func NewBackendStore(baseURL, token string) *BackendStore {
	return &BackendStore{BaseURL: baseURL, Token: token, HC: &http.Client{Timeout: 10 * time.Second}}
}

func (b *BackendStore) Put(ctx context.Context, snap Snapshot) (string, error) {
	body, _ := json.Marshal(map[string]any{"skill": snap.Skill, "args": snap.Args, "dryRun": snap.DryRun})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL+"/api/v1/mcp/confirmations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	b.auth(req)
	resp, err := b.HC.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("confirm store put %d: %s", resp.StatusCode, string(rb))
	}
	var out struct {
		ConfirmID string `json:"confirmId"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return "", err
	}
	return out.ConfirmID, nil
}

func (b *BackendStore) Get(ctx context.Context, id string) (Snapshot, bool, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, b.BaseURL+"/api/v1/mcp/confirmations/"+id, nil)
	b.auth(req)
	resp, err := b.HC.Do(req)
	if err != nil {
		return Snapshot{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Snapshot{}, false, nil // missing or expired
	}
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return Snapshot{}, false, fmt.Errorf("confirm store get %d: %s", resp.StatusCode, string(rb))
	}
	var snap Snapshot
	if err := json.Unmarshal(rb, &snap); err != nil {
		return Snapshot{}, false, err
	}
	return snap, true, nil
}

func (b *BackendStore) Delete(ctx context.Context, id string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, b.BaseURL+"/api/v1/mcp/confirmations/"+id, nil)
	b.auth(req)
	resp, err := b.HC.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		rb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("confirm store delete %d: %s", resp.StatusCode, string(rb))
	}
	return nil
}

func (b *BackendStore) auth(req *http.Request) {
	if b.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.Token)
	}
}
