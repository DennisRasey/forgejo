// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package optional

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"strconv"

	"code.forgejo.org/xorm/xorm/schemas"
)

type Option[T any] []T

func None[T any]() Option[T] {
	return nil
}

func Some[T any](v T) Option[T] {
	return Option[T]{v}
}

func FromPtr[T any](v *T) Option[T] {
	if v == nil {
		return None[T]()
	}
	return Some(*v)
}

func FromNonDefault[T comparable](v T) Option[T] {
	var zero T
	if v == zero {
		return None[T]()
	}
	return Some(v)
}

func (o Option[T]) Has() bool {
	return o != nil
}

func (o Option[T]) Get() (has bool, value T) {
	if o != nil {
		has = true
		value = o[0]
	}
	return has, value
}

func (o Option[T]) ValueOrZeroValue() T {
	var zeroValue T
	return o.ValueOrDefault(zeroValue)
}

func (o Option[T]) ValueOrDefault(v T) T {
	if o.Has() {
		return o[0]
	}
	return v
}

// ParseBool get the corresponding optional.Option[bool] of a string using strconv.ParseBool
func ParseBool(s string) Option[bool] {
	v, e := strconv.ParseBool(s)
	if e != nil {
		return None[bool]()
	}
	return Some(v)
}

// Option[T] can be used in an xorm bean as a field type for a nullable column. Multiple interfaces must be implemented
// for this to work correctly and won't be checked at compile-time of the bean struct, so they're asserted here in case
// the interface definitions change:
var (
	_ sql.Scanner              = (*Option[bool])(nil) // read data from DB
	_ driver.Valuer            = None[bool]()         // write data to DB
	_ schemas.SQLTypeDelegator = None[bool]()         // represent column field type correctly
)

// Convert database data into an Option[T]. sql.Null[T] has all the necessary logic to perform Value(), so it is used as
// an implementation.
func (o *Option[T]) Scan(value any) error {
	var n sql.Null[T]
	if err := n.Scan(value); err != nil {
		return err
	}
	if n.Valid {
		*o = Some(n.V)
	} else {
		*o = None[T]()
	}
	return nil
}

// Convert Option[T] into the necessary database data to represent it. sql.Null[T] has all the necessary logic to
// perform Value(), so it is used as an implementation.
func (o Option[T]) Value() (driver.Value, error) {
	var n sql.Null[T]
	if o.Has() {
		n.V = o[0]
		n.Valid = true
	} else {
		n.Valid = false
	}
	return n.Value()
}

// Make xorm use whatever SQLType is appropriate for T to represent Option[T] in the database table
func (o Option[T]) DelegateSQLType() reflect.Type {
	return reflect.TypeFor[T]()
}
