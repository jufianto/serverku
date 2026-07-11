package provisioner

import (
	"strings"
	"testing"
)

func TestHeartbeatScript(t *testing.T) {
	script := heartbeatScript(HeartbeatOpts{
		Hours:         6,
		ProjectName:   "demo",
		BotToken:      "123:abc",
		ChatID:        "42",
		HourlyRateUSD: 0.0067,
		RateIsLive:    true,
	})

	for _, want := range []string{
		"api.telegram.org/bot123:abc/sendMessage",
		`chat_id="42"`,
		"serverku down demo",
		"OnActiveSec=6h",
		"OnUnitActiveSec=6h",
		"chmod 0700 /usr/local/bin/serverku-heartbeat",
		"systemctl enable --now serverku-heartbeat.timer",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q", want)
		}
	}

	// A live rate must not carry est. markers.
	if strings.Contains(script, "est.") {
		t.Errorf("live-rate script must not contain est. markers:\n%s", script)
	}
	if !strings.Contains(script, "$0.0067/hr") {
		t.Errorf("script missing live rate:\n%s", script)
	}
}

func TestHeartbeatScriptEstimateMarked(t *testing.T) {
	script := heartbeatScript(HeartbeatOpts{
		Hours:         12,
		ProjectName:   "demo",
		BotToken:      "123:abc",
		ChatID:        "42",
		HourlyRateUSD: 0.0335,
		RateIsLive:    false,
	})

	// Offline table rates must be visibly marked as estimates.
	if !strings.Contains(script, "est.") {
		t.Errorf("table-rate script must contain est. markers:\n%s", script)
	}
}

func TestHeartbeatScriptNoRateOmitsCost(t *testing.T) {
	script := heartbeatScript(HeartbeatOpts{
		Hours:       6,
		ProjectName: "demo",
		BotToken:    "123:abc",
		ChatID:      "42",
	})

	if !strings.Contains(script, `COST=""`) {
		t.Errorf("no-rate script should set empty COST:\n%s", script)
	}
	if strings.Contains(script, "awk") {
		t.Errorf("no-rate script should not compute cost:\n%s", script)
	}
}
