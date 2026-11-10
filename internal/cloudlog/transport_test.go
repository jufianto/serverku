package cloudlog

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTransportLogsActionsWithoutSecrets(t *testing.T) {
	SetDebug(true)
	t.Cleanup(func() { SetDebug(false) })
	for _, tc := range []struct {
		name   string
		status int
		fail   bool
	}{
		{name: "success", status: 201},
		{name: "API rejection", status: 422},
		{name: "transport failure", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			oldWriter := log.Writer()
			log.SetOutput(&output)
			t.Cleanup(func() { log.SetOutput(oldWriter) })
			req, err := http.NewRequest(http.MethodPost, "https://api.example.test/v2/droplets?token=query-secret", strings.NewReader("private-body-secret"))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer auth-secret")
			expectedErr := errors.New("https://api.example.test/?token=error-secret")
			expected := &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader("response-secret"))}
			called := false
			transport := &Transport{Provider: "digitalocean", Base: roundTripFunc(func(got *http.Request) (*http.Response, error) {
				called = true
				if got != req || got.Header.Get("Authorization") != "Bearer auth-secret" || got.URL.RawQuery != "token=query-secret" {
					t.Error("request was changed")
				}
				if tc.fail {
					return nil, expectedErr
				}
				return expected, nil
			})}
			resp, gotErr := transport.RoundTrip(req)
			if !called {
				t.Fatal("request not forwarded")
			}
			if tc.fail {
				if gotErr != expectedErr || resp != nil {
					t.Fatalf("result changed: %v, %v", resp, gotErr)
				}
			} else {
				if resp != expected || gotErr != nil {
					t.Fatalf("result changed: %v, %v", resp, gotErr)
				}
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if string(body) != "response-secret" {
					t.Error("response body was consumed")
				}
			}
			logs := output.String()
			if !strings.Contains(logs, "[digitalocean API] POST /v2/droplets: sending") {
				t.Errorf("missing action: %s", logs)
			}
			result := "HTTP " + strconv.Itoa(tc.status)
			if tc.fail {
				result = "failed"
			}
			if !strings.Contains(logs, result) {
				t.Errorf("missing result: %s", logs)
			}
			for _, secret := range []string{"query-secret", "private-body-secret", "auth-secret", "response-secret", "error-secret"} {
				if strings.Contains(logs, secret) {
					t.Errorf("logs exposed %s", secret)
				}
			}
		})
	}
}

func TestTransportQuietModeStillReturnsFailures(t *testing.T) {
	SetDebug(false)
	var logs bytes.Buffer
	oldWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(oldWriter) })
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.test/v2/droplets", nil)
	for _, fail := range []bool{false, true} {
		expected := &http.Response{StatusCode: 403}
		expectedErr := errors.New("request failed")
		transport := &Transport{Provider: "gcp", Base: roundTripFunc(func(*http.Request) (*http.Response, error) {
			if fail {
				return nil, expectedErr
			}
			return expected, nil
		})}
		response, err := transport.RoundTrip(req)
		if fail {
			if err != expectedErr || response != nil {
				t.Fatal("transport failure hidden")
			}
		} else {
			if response != expected || err != nil {
				t.Fatal("HTTP failure changed")
			}
		}
	}
	if logs.Len() != 0 {
		t.Fatalf("quiet mode printed API logs: %s", logs.String())
	}
}
