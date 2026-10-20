package main

import (
	"strings"
	"testing"
)

func TestReachableGate(t *testing.T) {
	for _, tc := range []struct {
		report string
		fail   bool
	}{{`{"config":{}}`, false}, {`{"config":{}}{"finding":{"osv":"test","trace":[{"module":"m"}]}}`, false}, {`{"config":{}}{"finding":{"osv":"test","trace":[{"function":"Call"}]}}`, true}, {`{"config":{}}{"finding":`, true}, {``, true}} {
		if err := check(strings.NewReader(tc.report)); (err != nil) != tc.fail {
			t.Fatal(tc, err)
		}
	}
}
