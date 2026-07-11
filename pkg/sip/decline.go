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
	"time"
)

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
