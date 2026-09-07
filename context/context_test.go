// SPDX-FileCopyrightText: 2026 Forsway Scandinavia AB
// SPDX-License-Identifier: Apache-2.0

package context

import "testing"

// The EE subscription id is taken with Add(1), which increments before it returns, so
// init() has to leave the generator at 0 for the first subscription to be numbered "1"
// as it was when the counter was a plain int read before being incremented. A stored 1
// shifts every id by one, which is invisible to a test that only checks the ids are
// distinct.
func TestInitNumbersEeSubscriptionsFromOne(t *testing.T) {
	udrSelf := UDR_Self()

	initial := udrSelf.EeSubscriptionIDGenerator.Load()
	t.Cleanup(func() { udrSelf.EeSubscriptionIDGenerator.Store(initial) })

	if initial != 0 {
		t.Errorf("init left the EE subscription id generator at %d, want 0", initial)
	}
	if first := udrSelf.EeSubscriptionIDGenerator.Add(1); first != 1 {
		t.Errorf("first EE subscription would be numbered %d, want 1", first)
	}
}
