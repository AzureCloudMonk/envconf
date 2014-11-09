/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package envconf loads configuration options from environment variables.
package envconf

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var durType = reflect.TypeOf(time.Duration(0))

// Load returns an Environment populated with values from the user environment.
func Load() Environment {
	return New(os.Environ())
}

// New returns an Environment populated with configuration options from
// environ. Variable names are case-insensitive.
func New(environ []string) Environment {
	env := make(Environment)
	for _, value := range environ {
		if v := strings.SplitN(value, "=", 2); len(v) == 2 {
			env[v[0]] = v[1]
		}
	}
	return env
}

// An Environment holds encoded environment variables.
type Environment map[string]string

// Get returns the value of an environment variable, performing a
// case-insensitive search if an exact match is not found.
func (env Environment) Get(name string) (value string, ok bool) {
	if value, ok = env[name]; ok {
		return strings.TrimSpace(value), true
	}
	for key, value := range env {
		if strings.EqualFold(key, name) {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

// Decode decodes configuration options specified via prefixed env keys into
// the value pointed to by v. If v contains nested structs, Decode will prepend
// sep to each field name.
func (env Environment) Decode(prefix, sep string, v interface{}) error {
	return env.decode(prefix, sep, v, nil)
}

// DecodeStrict returns an error if env contains prefixed variables that do not
// correspond to either field names in v, or keys in ignoreEnv.
func (env Environment) DecodeStrict(prefix, sep string, v interface{}, ignoreEnv map[string]interface{}) error {
	var fields []string
	if err := env.decode(prefix, sep, v, &fields); err != nil {
		return err
	}
getEnv:
	for key := range env {
		if !hasPrefixFold(key, prefix) {
			continue
		}
		if _, ok := ignoreEnv[key]; ok {
			continue
		}
		for _, field := range fields {
			if strings.EqualFold(key, field) {
				continue getEnv
			}
		}
		for field := range ignoreEnv {
			if strings.EqualFold(key, field) {
				continue getEnv
			}
		}
		return fmt.Errorf("Unrecognized environment variable '%s'", key)
	}
	return nil
}

// decode decodes env into v.
func (env Environment) decode(prefix, sep string, v interface{}, fields *[]string) error {
	value := reflect.ValueOf(v)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return fmt.Errorf("Non-pointer type '%s'", value.Type())
	}
	field, ok := indirect(value)
	if !ok {
		return nil
	}
	return env.decodeField(prefix, sep, field, fields)
}

// decodeField decodes an environment variable into a struct field. Literals
// are decoded directly into the value; structs are decoded recursively.
func (env Environment) decodeField(name, sep string, value reflect.Value, fields *[]string) error {
	typ := value.Type()
	if typ.Kind() != reflect.Struct {
		source, ok := env.Get(name)
		if !ok {
			return nil
		}
		if err := decodeLiteral(source, value); err != nil {
			return nil
		}
		if fields != nil {
			*fields = append(*fields, name)
		}
		return nil
	}
	for i := 0; i < typ.NumField(); i++ {
		fieldTyp := typ.Field(i)
		tag := fieldTyp.Tag.Get("env")
		if tag == "-" {
			continue
		}
		field, ok := indirect(value.Field(i))
		if !ok {
			continue
		}
		if len(tag) > 0 || field.Kind() != reflect.Struct || !fieldTyp.Anonymous {
			if len(tag) == 0 {
				tag = fieldTyp.Name
			}
			tag = name + sep + tag
		} else {
			tag = name
		}
		if err := env.decodeField(tag, sep, field, fields); err != nil {
			return err
		}
	}
	return nil
}

// decodeLiteral decodes a source string into a value. Only integers, floats,
// Booleans, slices, and strings are supported.
func decodeLiteral(source string, value reflect.Value) (err error) {
	if !value.CanSet() {
		return nil
	}
	typ := value.Type()
	kind := typ.Kind()
	if kind >= reflect.Int && kind <= reflect.Int64 {
		var result int64
		if typ == durType {
			duration, err := time.ParseDuration(source)
			if err == nil {
				result = int64(duration)
			}
		} else {
			result, err = strconv.ParseInt(source, 0, value.Type().Bits())
		}
		if err != nil {
			return err
		}
		value.SetInt(result)
		return nil
	}
	if kind >= reflect.Uint && kind <= reflect.Uint64 {
		result, err := strconv.ParseUint(source, 0, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetUint(result)
		return nil
	}
	if kind >= reflect.Float32 && kind <= reflect.Float64 {
		result, err := strconv.ParseFloat(source, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetFloat(result)
		return nil
	}
	switch kind {
	case reflect.Bool:
		result, err := strconv.ParseBool(source)
		if err != nil {
			return err
		}
		value.SetBool(result)
		return nil

	case reflect.Slice:
		return decodeSlice(source, value)

	case reflect.String:
		value.SetString(source)
		return nil
	}
	return fmt.Errorf("Unsupported type %s", kind)
}

// splitList splits a comma-separated list into a slice of strings, accounting
// for escape characters.
func splitList(source string) (results []string) {
	var (
		isEscaped, hasEscape bool
		lastIndex, index     int
	)
	for ; index < len(source); index++ {
		if isEscaped {
			isEscaped = false
			continue
		}
		switch source[index] {
		case '\\':
			isEscaped = true
			hasEscape = true

		case ',':
			result := source[lastIndex:index]
			if hasEscape {
				result = strings.Map(removeEscape, result)
				hasEscape = false
			}
			results = append(results, result)
			lastIndex = index + 1
		}
	}
	if lastIndex < index {
		result := source[lastIndex:]
		if hasEscape {
			result = strings.Map(removeEscape, result)
		}
		results = append(results, result)
	}
	return results
}

// decodeSlice decodes a comma-separated list of values into a slice.
// Slices are decoded recursively.
func decodeSlice(source string, value reflect.Value) error {
	sources := splitList(source)
	value.SetLen(0)
	for _, source := range sources {
		element, ok := indirect(reflect.New(typ.Elem()))
		if !ok {
			continue
		}
		if err := decodeLiteral(source, element); err != nil {
			return err
		}
		value.Set(reflect.Append(value, element))
	}
	return nil
}

// indirect returns the value pointed to by a pointer, allocating zero values
// for nil pointers.
func indirect(value reflect.Value) (reflect.Value, bool) {
	for value.Kind() == reflect.Ptr {
		if value.IsNil() {
			if value.CanSet() {
				value.Set(reflect.New(value.Type().Elem()))
			} else {
				return value, false
			}
		}
		value = reflect.Indirect(value)
	}
	return value, true
}

// hasPrefixFold is a case-insensitive version of strings.HasPrefix.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// removeEscape is used by splitList to remove escape characters.
func removeEscape(r rune) rune {
	if r == '\\' {
		return -1
	}
	return r
}
