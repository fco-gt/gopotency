package idempotency

import "testing"

func TestResponse_ToCachedResponse(t *testing.T) {
	resp := &Response{
		StatusCode: 201,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"X-Test":       {"one", "two"},
		},
		Body:        []byte(`{"ok":true}`),
		ContentType: "application/json",
	}

	cached := resp.ToCachedResponse()

	if cached.StatusCode != resp.StatusCode {
		t.Fatalf("expected status %d, got %d", resp.StatusCode, cached.StatusCode)
	}
	if string(cached.Body) != string(resp.Body) {
		t.Fatalf("expected body %q, got %q", string(resp.Body), string(cached.Body))
	}
	if cached.ContentType != resp.ContentType {
		t.Fatalf("expected content type %q, got %q", resp.ContentType, cached.ContentType)
	}
	if len(cached.Headers["X-Test"]) != 2 {
		t.Fatalf("expected 2 X-Test headers, got %d", len(cached.Headers["X-Test"]))
	}
}

