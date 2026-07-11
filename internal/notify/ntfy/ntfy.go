// Package ntfy sends notifications via ntfy (https://ntfy.sh) topics.
// ntfy is account-less publish/subscribe push: publishing is a plain HTTP
// POST to <server>/<topic>, and the topic name is the only capability.
package ntfy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Notifier struct {
	server string // base URL without trailing slash, e.g. https://ntfy.sh
	topic  string
	client *http.Client
}

// New creates a new ntfy Notifier publishing to server/topic.
func New(server, topic string) *Notifier {
	return &Notifier{
		server: strings.TrimRight(server, "/"),
		topic:  topic,
		client: &http.Client{
			Timeout: 5 * time.Second, // do not block lifecycle commands
		},
	}
}

// send publishes a message to the topic. Title, priority, and tags ride in
// headers per the ntfy publish API.
func (n *Notifier) send(ctx context.Context, title, msg, priority, tags string) error {
	if n.topic == "" {
		return nil
	}

	url := fmt.Sprintf("%s/%s", n.server, n.topic)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(msg))
	if err != nil {
		return err
	}
	req.Header.Set("Title", title)
	req.Header.Set("Priority", priority)
	req.Header.Set("Tags", tags)

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned status: %s", resp.Status)
	}
	return nil
}

func (n *Notifier) SendUp(ctx context.Context, project string, ip string) error {
	return n.send(ctx, "serverku: "+project+" is up",
		fmt.Sprintf("Project %s is up and running at %s", project, ip),
		"default", "white_check_mark")
}

func (n *Notifier) SendDown(ctx context.Context, project string) error {
	return n.send(ctx, "serverku: "+project+" is down",
		fmt.Sprintf("Project %s has been shut down", project),
		"default", "octagonal_sign")
}

func (n *Notifier) SendError(ctx context.Context, project string, err error) error {
	return n.send(ctx, "serverku: "+project+" error",
		fmt.Sprintf("Project %s encountered an error: %s", project, err.Error()),
		"high", "x")
}

// SendTest publishes a test message so users can verify their subscription
// (used by `serverku ntfy --test`).
func (n *Notifier) SendTest(ctx context.Context, project string) error {
	return n.send(ctx, "serverku: test notification",
		fmt.Sprintf("Subscription for project %s works. You will receive lifecycle and heartbeat notifications here.", project),
		"default", "tada")
}
