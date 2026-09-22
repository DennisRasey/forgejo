// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"forgejo.org/modules/optional"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionScheduleSpec_NewActionScheduleSpec(t *testing.T) {
	tests := []struct {
		name        string
		refTime     time.Time
		cronPattern string
		timeZone    string
		want        string
		wantErr     assert.ErrorAssertionFunc
	}{
		{
			name:        "without timezone",
			refTime:     time.Date(2026, 4, 6, 11, 56, 0, 0, time.UTC),
			cronPattern: "58 14 * * *",
			want:        "2026-04-06T14:58:00Z",
			wantErr:     assert.NoError,
		},
		{
			name:        "with separate timezone",
			refTime:     time.Date(2026, 4, 6, 11, 56, 0, 0, time.UTC),
			cronPattern: "58 14 * * *",
			timeZone:    "Europe/Tallinn", // +03 (EEST)
			want:        "2026-04-06T11:58:00Z",
			wantErr:     assert.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, err := NewActionScheduleSpec(test.cronPattern, optional.FromNonDefault(test.timeZone), test.refTime)
			test.wantErr(t, err)

			if err == nil {
				assert.Equal(t, test.want, s.Next.AsTime().UTC().Format(time.RFC3339))
			}
		})
	}
}

func TestActionScheduleSpec_Parse(t *testing.T) {
	// Mock the local timezone is not UTC
	local := time.Local
	tz, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	defer func() {
		time.Local = local
	}()
	time.Local = tz

	isError := func(t assert.TestingT, err error, log ...any) bool {
		return assert.ErrorIs(t, err, ErrPersistentScheduling, log...)
	}

	tests := []struct {
		name     string
		refTime  time.Time
		spec     string
		timeZone string
		want     string
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:    "regular",
			refTime: time.Date(2024, 7, 31, 15, 47, 55, 0, time.Local),
			spec:    "0 10 * * *",
			want:    "2024-07-31T10:00:00Z",
			wantErr: assert.NoError,
		},
		{
			name:    "invalid",
			refTime: time.Date(2024, 7, 31, 15, 47, 55, 0, time.Local),
			spec:    "0 10 * *",
			want:    "",
			wantErr: isError,
		},
		{
			name:    "with TZ in cron schedule",
			refTime: time.Date(2024, 7, 31, 15, 47, 55, 0, time.Local),
			spec:    "TZ=America/New_York 0 10 * * *",
			want:    "2024-07-31T14:00:00Z",
			wantErr: assert.NoError,
		},
		{
			name:    "with CRON_TZ in cron schedule",
			refTime: time.Date(2024, 7, 31, 15, 47, 55, 0, time.Local),
			spec:    "CRON_TZ=America/New_York 0 10 * * *",
			want:    "2024-07-31T14:00:00Z",
			wantErr: assert.NoError,
		},
		{
			name:     "with separate time zone",
			refTime:  time.Date(2024, 7, 31, 15, 47, 55, 0, time.Local),
			spec:     "0 10 * * *",
			timeZone: "America/New_York",
			want:     "2024-07-31T14:00:00Z",
			wantErr:  assert.NoError,
		},
		{
			name:     "separate time zone takes precedence over inlined time zone",
			refTime:  time.Date(2024, 7, 31, 15, 47, 55, 0, time.Local),
			spec:     "CRON_TZ=Europe/Berlin 0 10 * * *",
			timeZone: "America/New_York",
			want:     "2024-07-31T14:00:00Z",
			wantErr:  assert.NoError,
		},
		{
			name:    "time zone irrelevant",
			refTime: time.Date(2024, 7, 31, 15, 47, 55, 0, time.Local),
			spec:    "@every 5m",
			want:    "2024-07-31T07:52:55Z",
			wantErr: assert.NoError,
		},
		{
			name:    "time zone irrelevant",
			refTime: time.Date(2024, 7, 31, 15, 47, 55, 0, time.Local),
			spec:    "TZ=Europe/Canada 0 10 * * *", // invalid TZ
			want:    "",
			wantErr: isError,
		},
		{
			// The various cron implementations handle the DST jump forwards differently. The most popular approaches
			// are (a) scheduling all jobs at 3 o'clock that were supposed to run between 2 and 3 o'clock, or (b)
			// skipping the execution on that day because any time between 2 and 3 o'clock never happened. Forgejo uses
			// option B because the code it inherited already did that and was exposed to users.
			name:     "skips execution during DST jump forwards",
			refTime:  time.Date(2025, 3, 30, 0, 55, 0, 0, time.UTC), // 01:55 local time
			spec:     "10 2 * * *",                                  // The clock jumps at 2 o'clock to 3 o'clock.
			timeZone: "Europe/Berlin",
			want:     "2025-03-31T00:10:00Z",
			wantErr:  assert.NoError,
		},
		{
			name:     "executes a first time before DST jump backwards",
			refTime:  time.Date(2025, 10, 26, 0, 5, 0, 0, time.UTC), // 02:05 local time
			spec:     "10 2 * * *",                                  // The clock jumps at 3 o'clock to 2 o'clock.
			timeZone: "Europe/Berlin",
			want:     "2025-10-26T00:10:00Z",
			wantErr:  assert.NoError,
		},
		{
			name:     "executes a second time after DST jump backwards",
			refTime:  time.Date(2025, 10, 26, 1, 5, 0, 0, time.UTC), // 02:05 local time
			spec:     "10 2 * * *",                                  // The clock jumps at 3 o'clock to 2 o'clock.
			timeZone: "Europe/Berlin",
			want:     "2025-10-26T01:10:00Z",
			wantErr:  assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ActionScheduleSpec{
				Spec:     tt.spec,
				TimeZone: optional.FromNonDefault(tt.timeZone),
			}
			got, err := s.Parse()
			tt.wantErr(t, err)

			if err == nil {
				assert.Equal(t, tt.want, got.Next(tt.refTime).UTC().Format(time.RFC3339))
			}
		})
	}
}
