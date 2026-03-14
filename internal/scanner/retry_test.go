package scanner

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sequentialMockClient struct {
	responses []*http.Response
	errors    []error
	callCount int
}

func (m *sequentialMockClient) Do(_ *http.Request) (*http.Response, error) {
	idx := m.callCount
	m.callCount++
	if idx >= len(m.responses) {
		return nil, fmt.Errorf("unexpected call %d", idx)
	}
	return m.responses[idx], m.errors[idx]
}

func TestRetryOn500(t *testing.T) {
	mock := &sequentialMockClient{
		responses: []*http.Response{
			{StatusCode: 500, Body: io.NopCloser(strings.NewReader(""))},
			{StatusCode: 500, Body: io.NopCloser(strings.NewReader(""))},
			{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))},
		},
		errors: []error{nil, nil, nil},
	}

	client := NewRetryClient(mock, 3, 1*time.Millisecond)
	req, _ := http.NewRequest("GET", "http://test.local", nil)
	resp, err := client.Do(req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, mock.callCount)
}

func TestNoRetryOn400(t *testing.T) {
	mock := &sequentialMockClient{
		responses: []*http.Response{
			{StatusCode: 400, Body: io.NopCloser(strings.NewReader("bad request"))},
		},
		errors: []error{nil},
	}

	client := NewRetryClient(mock, 3, 1*time.Millisecond)
	req, _ := http.NewRequest("GET", "http://test.local", nil)
	resp, err := client.Do(req)

	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, 1, mock.callCount)
}

func TestRetrySuccessAfterTwo(t *testing.T) {
	mock := &sequentialMockClient{
		responses: []*http.Response{
			nil,
			{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))},
		},
		errors: []error{
			fmt.Errorf("connection refused"),
			nil,
		},
	}

	client := NewRetryClient(mock, 3, 1*time.Millisecond)
	req, _ := http.NewRequest("GET", "http://test.local", nil)
	resp, err := client.Do(req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, mock.callCount)
}

func TestRetryMaxAttemptsExhausted(t *testing.T) {
	mock := &sequentialMockClient{
		responses: []*http.Response{
			{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))},
			{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))},
			{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))},
		},
		errors: []error{nil, nil, nil},
	}

	client := NewRetryClient(mock, 3, 1*time.Millisecond)
	req, _ := http.NewRequest("GET", "http://test.local", nil)
	resp, err := client.Do(req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "max retries (3) exhausted")
	assert.Equal(t, 3, mock.callCount)
}

func TestRetryOn429(t *testing.T) {
	mock := &sequentialMockClient{
		responses: []*http.Response{
			{StatusCode: 429, Body: io.NopCloser(strings.NewReader(""))},
			{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))},
		},
		errors: []error{nil, nil},
	}

	client := NewRetryClient(mock, 3, 1*time.Millisecond)
	req, _ := http.NewRequest("GET", "http://test.local", nil)
	resp, err := client.Do(req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, mock.callCount)
}
