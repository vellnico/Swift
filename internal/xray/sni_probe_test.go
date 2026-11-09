//go:build probe

package xray

import (
	"context"
	"testing"
)

// go test -tags probe -run TestProbeCandidates -v ./internal/xray
// checks the built-in Reality targets from the current network.
func TestProbeCandidates(t *testing.T) {
	ok := 0
	for _, h := range SNICandidates {
		err := TLSProber{}.Probe(context.Background(), h)
		t.Logf("%-24s %v", h, err)
		if err == nil {
			ok++
		}
	}
	if ok == 0 {
		t.Fatal("no candidate passed")
	}
}
