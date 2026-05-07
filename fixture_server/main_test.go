package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseNodePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		path  string
		ok    bool
		level int
		id    int
	}{
		{name: "valid", path: "/node/2/15", ok: true, level: 2, id: 15},
		{name: "invalid parts", path: "/node/2", ok: false},
		{name: "invalid prefix", path: "/x/2/1", ok: false},
		{name: "invalid level", path: "/node/a/1", ok: false},
		{name: "invalid id", path: "/node/1/b", ok: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			level, id, ok := parseNodePath(tc.path)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v got %v", tc.ok, ok)
			}
			if tc.ok {
				if level != tc.level || id != tc.id {
					t.Fatalf("expected (%d,%d), got (%d,%d)", tc.level, tc.id, level, id)
				}
			}
		})
	}
}

func TestPageHandlerRootAndNode(t *testing.T) {
	t.Parallel()

	cfg := config{Depth: 2, Branching: 3, Latency: 0}
	h := pageHandler(cfg, &serverState{})

	rootReq := httptest.NewRequest(http.MethodGet, "http://fixture/", nil)
	rootRec := httptest.NewRecorder()
	h(rootRec, rootReq)

	if rootRec.Code != http.StatusOK {
		t.Fatalf("expected root status 200, got %d", rootRec.Code)
	}

	rootBody := rootRec.Body.String()
	if !strings.Contains(rootBody, "/node/1/0") {
		t.Fatalf("expected child link on root page")
	}

	nodeReq := httptest.NewRequest(http.MethodGet, "http://fixture/node/1/4", nil)
	nodeRec := httptest.NewRecorder()
	h(nodeRec, nodeReq)

	if nodeRec.Code != http.StatusOK {
		t.Fatalf("expected node status 200, got %d", nodeRec.Code)
	}

	nodeBody := nodeRec.Body.String()
	if !strings.Contains(nodeBody, "node-1-4") {
		t.Fatalf("expected node identity in body")
	}
}

func TestPageHandlerDepthLimitAndLatency(t *testing.T) {
	t.Parallel()

	cfg := config{Depth: 1, Branching: 2, Latency: 20 * time.Millisecond}
	h := pageHandler(cfg, &serverState{})

	start := time.Now()
	req := httptest.NewRequest(http.MethodGet, "http://fixture/node/2/1", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for depth overflow, got %d", rec.Code)
	}

	if time.Since(start) < 20*time.Millisecond {
		t.Fatalf("expected latency delay to be applied")
	}
}

func TestStatusInjection429EveryN(t *testing.T) {
	t.Parallel()

	cfg := config{Depth: 1, Branching: 2, Latency: 0, StatusMode: "429_every_n", StatusN: 3}
	h := pageHandler(cfg, &serverState{})

	for i := 1; i <= 6; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://fixture/", nil)
		rec := httptest.NewRecorder()
		h(rec, req)

		if i%3 == 0 {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d expected 429 got %d", i, rec.Code)
			}
		} else {
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d expected 200 got %d", i, rec.Code)
			}
		}
	}
}

func TestStatusInjection500EveryN(t *testing.T) {
	t.Parallel()

	cfg := config{Depth: 1, Branching: 2, Latency: 0, StatusMode: "500_every_n", StatusN: 2}
	h := pageHandler(cfg, &serverState{})

	first := httptest.NewRecorder()
	h(first, httptest.NewRequest(http.MethodGet, "http://fixture/", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first request expected 200 got %d", first.Code)
	}

	second := httptest.NewRecorder()
	h(second, httptest.NewRequest(http.MethodGet, "http://fixture/", nil))
	if second.Code != http.StatusInternalServerError {
		t.Fatalf("second request expected 500 got %d", second.Code)
	}
}

func TestShouldInjectStatus(t *testing.T) {
	t.Parallel()

	if shouldInjectStatus(config{StatusMode: "none", StatusN: 5}, 10) {
		t.Fatalf("none mode should never inject")
	}

	if !shouldInjectStatus(config{StatusMode: "429_every_n", StatusN: 5}, 10) {
		t.Fatalf("expected injection at multiple of n")
	}

	if shouldInjectStatus(config{StatusMode: "429_every_n", StatusN: 5}, 11) {
		t.Fatalf("did not expect injection for non-multiple")
	}
}

func TestRobotsAndHealthHandlers(t *testing.T) {
	t.Parallel()

	robotsReq := httptest.NewRequest(http.MethodGet, "http://fixture/robots.txt", nil)
	robotsRec := httptest.NewRecorder()
	robotsHandler(robotsRec, robotsReq)

	if robotsRec.Code != http.StatusOK {
		t.Fatalf("expected robots status 200, got %d", robotsRec.Code)
	}

	if !strings.Contains(robotsRec.Body.String(), "Allow: /") {
		t.Fatalf("expected robots allow rule")
	}

	healthReq := httptest.NewRequest(http.MethodGet, "http://fixture/healthz", nil)
	healthRec := httptest.NewRecorder()
	healthzHandler(healthRec, healthReq)

	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected health status 200, got %d", healthRec.Code)
	}

	body, _ := io.ReadAll(healthRec.Body)
	if string(body) != "ok\n" {
		t.Fatalf("unexpected health body: %s", string(body))
	}
}
