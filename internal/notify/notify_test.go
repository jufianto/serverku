package notify

import (
	"context"
	"errors"
	"testing"
)

type mockNotifier struct {
	upErr    error
	downErr  error
	errorErr error

	upCalled    bool
	downCalled  bool
	errorCalled bool
}

func (m *mockNotifier) SendUp(ctx context.Context, project string, ip string) error {
	m.upCalled = true
	return m.upErr
}

func (m *mockNotifier) SendDown(ctx context.Context, project string) error {
	m.downCalled = true
	return m.downErr
}

func (m *mockNotifier) SendError(ctx context.Context, project string, err error) error {
	m.errorCalled = true
	return m.errorErr
}

func TestMultiNotifier_Success(t *testing.T) {
	mock1 := &mockNotifier{}
	mock2 := &mockNotifier{}

	multi := NewMultiNotifier(mock1, mock2)

	err := multi.SendUp(context.Background(), "test", "1.2.3.4")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mock1.upCalled || !mock2.upCalled {
		t.Error("expected both mocks to be called for SendUp")
	}

	err = multi.SendDown(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mock1.downCalled || !mock2.downCalled {
		t.Error("expected both mocks to be called for SendDown")
	}

	err = multi.SendError(context.Background(), "test", errors.New("boom"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mock1.errorCalled || !mock2.errorCalled {
		t.Error("expected both mocks to be called for SendError")
	}
}

func TestMultiNotifier_Error(t *testing.T) {
	expectedErr := errors.New("notification failed")
	mock1 := &mockNotifier{upErr: expectedErr}
	mock2 := &mockNotifier{}

	multi := NewMultiNotifier(mock1, mock2)

	err := multi.SendUp(context.Background(), "test", "1.2.3.4")
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	// Should stop on first error
	if !mock1.upCalled {
		t.Error("expected mock1 to be called")
	}
	if mock2.upCalled {
		t.Error("expected mock2 NOT to be called after mock1 failed")
	}
}
