// Copyright 2010 The Go Authors. All rights reserved.
// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause
//
// Original source code: https://cs.opensource.google/go/go/+/refs/tags/go1.25.4:src/time/format.go;l=1621

package duration

import (
	"errors"
	"strconv"
	"time"
)

// leadingInt consumes the leading [0-9]* from s.
func leadingInt(s string) (x uint64, rem string, ok bool) {
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c-'0' > 9 {
			break
		}
		if x > 1<<63/10 {
			// overflow
			return 0, rem, false
		}
		x = x*10 + uint64(c) - '0'
		if x > 1<<63 {
			// overflow
			return 0, rem, false
		}
	}
	return x, s[i:], true
}

// leadingFraction consumes the leading [0-9]* from s.
// It is used only for fractions, so does not return an error on overflow,
// it just stops accumulating precision.
func leadingFraction(s string) (x uint64, scale float64, rem string) {
	i := 0
	scale = 1
	overflow := false
	for ; i < len(s); i++ {
		c := s[i]
		if c-'0' > 9 {
			break
		}
		if overflow {
			continue
		}
		if x > (1<<63-1)/10 {
			// It's possible for overflow to give a positive number, so take care.
			overflow = true
			continue
		}
		y := x*10 + uint64(c) - '0'
		if y > 1<<63 {
			overflow = true
			continue
		}
		x = y
		scale *= 10
	}
	return x, scale, s[i:]
}

var unitMap = map[string]uint64{
	"ns": uint64(time.Nanosecond),
	"us": uint64(time.Microsecond),
	"µs": uint64(time.Microsecond), // U+00B5 = micro symbol
	"μs": uint64(time.Microsecond), // U+03BC = Greek letter mu
	"ms": uint64(time.Millisecond),
	"s":  uint64(time.Second),
	"m":  uint64(time.Minute),
	"h":  uint64(time.Hour),

	// Added, take a common definition.
	"d": uint64(time.Hour * 24),
	"y": uint64(time.Hour * 24 * 365),
}

// Parse parses a duration string.
// A duration string is a possibly signed sequence of
// decimal numbers, each with optional fraction and a unit suffix,
// such as "300ms", "-1.5h" or "2h45m".
// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h", "d", "y".
func Parse(s string) (time.Duration, error) {
	// [-+]?([0-9]*(\.[0-9]*)?[a-z]+)+
	orig := s
	var d uint64
	neg := false

	// Consume [-+]?
	if s != "" {
		c := s[0]
		if c == '-' || c == '+' {
			neg = c == '-'
			s = s[1:]
		}
	}
	// Special case: if all that is left is "0", this is zero.
	if s == "0" {
		return 0, nil
	}
	if s == "" {
		return 0, errors.New("time: invalid duration " + strconv.QuoteToASCII(orig))
	}
	for s != "" {
		var (
			v, f  uint64      // integers before, after decimal point
			scale float64 = 1 // value = v + f/scale
		)

		var ok bool

		// The next character must be [0-9.]
		if s[0] != '.' && s[0]-'0' > 9 {
			return 0, errors.New("time: invalid duration " + strconv.QuoteToASCII(orig))
		}
		// Consume [0-9]*
		pl := len(s)
		v, s, ok = leadingInt(s)
		if !ok {
			return 0, errors.New("time: invalid duration " + strconv.QuoteToASCII(orig))
		}
		pre := pl != len(s) // whether we consumed anything before a period

		// Consume (\.[0-9]*)?
		post := false
		if s != "" && s[0] == '.' {
			s = s[1:]
			pl := len(s)
			f, scale, s = leadingFraction(s)
			post = pl != len(s)
		}
		if !pre && !post {
			// no digits (e.g. ".s" or "-.s")
			return 0, errors.New("time: invalid duration " + strconv.QuoteToASCII(orig))
		}

		// Consume unit.
		i := 0
		for ; i < len(s); i++ {
			c := s[i]
			if c == '.' || c-'0' <= 9 {
				break
			}
		}
		if i == 0 {
			return 0, errors.New("time: missing unit in duration " + strconv.QuoteToASCII(orig))
		}
		u := s[:i]
		s = s[i:]
		unit, ok := unitMap[u]
		if !ok {
			return 0, errors.New("time: unknown unit " + strconv.QuoteToASCII(u) + " in duration " + strconv.QuoteToASCII(orig))
		}
		if v > 1<<63/unit {
			// overflow
			return 0, errors.New("time: invalid duration " + strconv.QuoteToASCII(orig))
		}
		v *= unit
		if f > 0 {
			// float64 is needed to be nanosecond accurate for fractions of hours.
			// v >= 0 && (f*unit/scale) <= 3.6e+12 (ns/h, h is the largest unit)
			v += uint64(float64(f) * (float64(unit) / scale))
			if v > 1<<63 {
				// overflow
				return 0, errors.New("time: invalid duration " + strconv.QuoteToASCII(orig))
			}
		}
		d += v
		if d > 1<<63 {
			return 0, errors.New("time: invalid duration " + strconv.QuoteToASCII(orig))
		}
	}
	if neg {
		return -time.Duration(d), nil
	}
	if d > 1<<63-1 {
		return 0, errors.New("time: invalid duration " + strconv.QuoteToASCII(orig))
	}
	return time.Duration(d), nil
}
