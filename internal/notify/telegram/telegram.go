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

func (n *Notifier) SendTest(ctx context.Context, project string) error {
	return n.send(ctx, fmt.Sprintf("👋 *serverku*: test notification for project `%s`. You will receive lifecycle and heartbeat updates here.", project))
}

// BotInfo describes a Telegram bot, as returned by getMe.
type BotInfo struct {
	Username string
}

// GetMe verifies a bot token against the Telegram API and returns the bot's
// username. Used by `serverku notify setup` to check the token before asking
// the user to message the bot.
func GetMe(ctx context.Context, botToken string) (BotInfo, error) {
	return getMe(ctx, defaultBaseURL, botToken)
}

func getMe(ctx context.Context, baseURL, botToken string) (BotInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/bot%s/getMe", baseURL, botToken), nil)
	if err != nil {
		return BotInfo{}, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return BotInfo{}, err
	}
	defer resp.Body.Close()

	var body struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return BotInfo{}, err
	}
	if !body.OK || body.Result.Username == "" {
		return BotInfo{}, fmt.Errorf("telegram rejected the bot token (status %s)", resp.Status)
	}
	return BotInfo{Username: body.Result.Username}, nil
}

// Chat describes the sender of a message to the bot, as discovered from
// getUpdates.
type Chat struct {
	ID   string
	Name string
}

// DiscoverChatID polls getUpdates until someone messages the bot, then
// returns that chat's ID. This lets `serverku notify setup` find the chat_id
// automatically: the user just sends the bot any message. It blocks until a
// message arrives or ctx is done.
func DiscoverChatID(ctx context.Context, botToken string) (Chat, error) {
	return discoverChatID(ctx, defaultBaseURL, botToken)
}

func discoverChatID(ctx context.Context, baseURL, botToken string) (Chat, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	for {
		if err := ctx.Err(); err != nil {
			return Chat{}, fmt.Errorf("no message received: %w", err)
		}

		// Short long-poll so ctx cancellation is respected promptly.
		url := fmt.Sprintf("%s/bot%s/getUpdates?timeout=10", baseURL, botToken)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return Chat{}, err
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return Chat{}, fmt.Errorf("no message received: %w", ctx.Err())
			}
			return Chat{}, err
		}

		var body struct {
			OK     bool `json:"ok"`
			Result []struct {
				Message struct {
					Chat struct {
						ID        int64  `json:"id"`
						FirstName string `json:"first_name"`
						Username  string `json:"username"`
						Title     string `json:"title"`
					} `json:"chat"`
				} `json:"message"`
			} `json:"result"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			return Chat{}, err
		}
		if !body.OK {
			return Chat{}, fmt.Errorf("telegram getUpdates failed (status %s)", resp.Status)
		}

		// Use the most recent update that carries a chat.
		for i := len(body.Result) - 1; i >= 0; i-- {
			c := body.Result[i].Message.Chat
			if c.ID == 0 {
				continue
			}
			name := c.FirstName
			if name == "" {
				name = c.Username
			}
			if name == "" {
				name = c.Title
			}
			return Chat{ID: fmt.Sprintf("%d", c.ID), Name: name}, nil
		}
	}
}
