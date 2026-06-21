package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// defaultBaseURL is the Telegram Bot API root. It is overridable per-instance
// so tests can point the notifier at an httptest server.
const defaultBaseURL = "https://api.telegram.org"

type Notifier struct {
	botToken string
	chatID   string
	baseURL  string
	client   *http.Client
}

// New creates a new Telegram Notifier.
func New(botToken, chatID string) *Notifier {
	return &Notifier{
		botToken: botToken,
		chatID:   chatID,
		baseURL:  defaultBaseURL,
		client: &http.Client{
			Timeout: 5 * time.Second, // 5s timeout to prevent blocking
		},
	}
}

type payload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (n *Notifier) send(ctx context.Context, msg string) error {
	if n.botToken == "" || n.chatID == "" {
		return nil
	}

	p := payload{
		ChatID:    n.chatID,
		Text:      msg,
		ParseMode: "Markdown",
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", n.baseURL, n.botToken)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
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
		return fmt.Errorf("telegram api returned status: %s", resp.Status)
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
