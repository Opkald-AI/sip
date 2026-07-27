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
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// declineDigits reduces a phone number to its significant digits for matching:
// keeps digits only and strips the Danish country code when the length shows it
// is prefixed (0045 + 8, or 45 + 8), so "+4548795304", "004548795304",
// "4548795304" and "48795304" all normalize to the same value. A bare 8-digit
// number that happens to start with "45" is left untouched.
func declineDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteByte(byte(r))
		}
	}
	x := b.String()
	if len(x) == 12 && strings.HasPrefix(x, "0045") {
		return x[4:]
	}
	if len(x) == 10 && strings.HasPrefix(x, "45") {
		return x[2:]
	}
	return x
}

// matchesDeclineNumber reports whether the called (dialed) number is one of the
// numbers the decline check is scoped to. An empty list matches nothing, so the
// check stays off for every clinic until it is explicitly scoped to a number.
func matchesDeclineNumber(to string, numbers []string) bool {
	if len(numbers) == 0 || to == "" {
		return false
	}
	td := declineDigits(to)
	if td == "" {
		return false
	}
	for _, n := range numbers {
		if declineDigits(n) == td {
			return true
		}
	}
	return false
}

// declineCheckTimeout bounds the pre-answer decline check so a slow/unreachable
// endpoint can never stall the INVITE path. On timeout we fail open (answer).
const declineCheckTimeout = 1500 * time.Millisecond

type declineCheckResult struct {
	Reject bool `json:"reject"`
}

// shouldDeclineInbound asks an external endpoint whether an inbound call should
// be rejected with a busy tone before it is answered. It runs after the trunk
// is resolved (so the endpoint knows which clinic/number was called) and before
// the call is accepted, so a positive result yields a real 486 Busy Here.
//
// It is deliberately FAIL-OPEN: any misconfiguration, timeout, non-200 status,
// or unparsable body returns false so a check problem never rejects a
// legitimate caller. The endpoint contract is a GET with query params
// from / to / trunk, returning 200 and JSON {"reject": <bool>}.
func shouldDeclineInbound(ctx context.Context, checkURL, from, to, trunkID string) bool {
	if checkURL == "" {
		return false
	}
	u, err := url.Parse(checkURL)
	if err != nil {
		return false
	}
	q := u.Query()
	q.Set("from", from)
	q.Set("to", to)
	q.Set("trunk", trunkID)
	u.RawQuery = q.Encode()

	cctx, cancel := context.WithTimeout(ctx, declineCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var out declineCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	return out.Reject
}
