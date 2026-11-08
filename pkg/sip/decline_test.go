// Copyright 2024 LiveKit, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sip

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeclineDigits(t *testing.T) {
	cases := map[string]string{
		"+4573708725":  "73708725",
		"004573708725": "73708725",
		"4573708725":   "73708725",
		"73708725":     "73708725",
		"45123456":     "45123456", // bare 8 digits that start with 45 stay as they are
		"":             "",
	}
	for in, want := range cases {
		if got := declineDigits(in); got != want {
			t.Errorf("declineDigits(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchesDeclineNumber(t *testing.T) {
	list := []string{"73708725", "+4573708737"}
	cases := []struct {
		name    string
		to      string
		numbers []string
		want    bool
	}{
		{"listed number", "73708725", list, true},
		{"listed number with country code", "+4573708737", list, true},
		{"listed number, other prefix form", "004573708725", list, true},
		{"number not on the list", "73708759", list, false},
		{"empty list matches nothing", "73708725", nil, false},
		{"empty called number", "", list, false},
		{"wildcard matches any number", "73708759", []string{"*"}, true},
		{"wildcard with country code", "+4573708759", []string{"*"}, true},
		{"wildcard with spaces", "73708759", []string{" * "}, true},
		{"wildcard next to numbers", "12345678", []string{"73708725", "*"}, true},
		{"wildcard, empty called number", "", []string{"*"}, false},
		{"wildcard, called number without digits", "anonymous", []string{"*"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := matchesDeclineNumber(c.to, c.numbers); got != c.want {
				t.Errorf("matchesDeclineNumber(%q, %v) = %v, want %v", c.to, c.numbers, got, c.want)
			}
		})
	}
}

func TestShouldDeclineInbound(t *testing.T) {
	serve := func(status int, body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("from") == "" || r.URL.Query().Get("to") == "" {
				t.Errorf("missing from/to in %s", r.URL.RawQuery)
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
	}
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"reject", http.StatusOK, `{"reject": true}`, true},
		{"allow", http.StatusOK, `{"reject": false}`, false},
		{"server error fails open", http.StatusInternalServerError, `{"reject": true}`, false},
		{"bad body fails open", http.StatusOK, `not json`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := serve(c.status, c.body)
			defer srv.Close()
			if got := shouldDeclineInbound(context.Background(), srv.URL+"/api/should-decline", "22978097", "73708759", "trunk"); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
	if shouldDeclineInbound(context.Background(), "", "1", "2", "t") {
		t.Error("empty URL must fail open")
	}
}
