// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package timeutil

import (
	"testing"
	"time"

	"forgejo.org/modules/optional"

	"github.com/stretchr/testify/assert"
)

func TestTimeStamp_AsOptionalTime(t *testing.T) {
	tests := []struct {
		name      string
		timestamp TimeStamp
		expected  optional.Option[time.Time]
	}{
		{
			name:      "Positive timestamp",
			timestamp: TimeStamp(1),
			expected:  optional.Some(time.Date(1970, 1, 1, 0, 0, 1, 0, time.UTC).Local()),
		},
		{
			name:      "Zero timestamp",
			timestamp: TimeStamp(0),
			expected:  optional.None[time.Time](),
		},
		{
			name:      "Negative timestamp",
			timestamp: TimeStamp(-1),
			expected:  optional.Some[time.Time](time.Date(1969, 12, 31, 23, 59, 59, 0, time.UTC).Local()),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.timestamp.AsOptionalTime())
		})
	}
}
