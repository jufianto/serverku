package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier struct {
	webhookURL string
	client     *http.Client
}

// New creates a new Slack Notifier.
func New(webhookURL string) *Notifier {
	return &Notifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 5 * time.Second, // 5s timeout to prevent blocking
		},
	}
}

type payload struct {
	Text string `json:"text"`
}

func (n *Notifier) send(ctx context.Context, msg string) error {
	if n.webhookURL == "" {
		return nil
	}

	p := payload{Text: msg}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack api returned status: %s", resp.Status)
	}

	return nil
}

func (n *Notifier) SendUp(ctx context.Context, project string, ip string) error {
	return n.send(ctx, fmt.Sprintf("✅ *serverku*: Project `%s` is up and running at `%s`", project, ip))
}

func (n *Notifier) SendDown(ctx context.Context, project string) error {
	return n.send(ctx, fmt.Sprintf("🛑 *serverku*: Project `%s` has been shut down", project))
}

func (n *Notifier) SendError(ctx context.Context, project string, err error) error {
	return n.send(ctx, fmt.Sprintf("❌ *serverku*: Project `%s` encountered an error:\n```%s```", project, err.Error()))
}
