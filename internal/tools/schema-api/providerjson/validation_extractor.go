// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package providerjson

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// extractValidationParameters tries to extract actual parameters from validation functions
// by testing them with sample values to understand their behavior
func extractValidationParameters(fn interface{}, validationType string) []interface{} {
	if fn == nil {
		return nil
	}

	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return nil
	}

	t := v.Type()

	// Check if this is a SchemaValidateFunc: func(interface{}, string) ([]string, []error)
	if t.NumIn() != 2 || t.NumOut() != 2 {
		return nil
	}

	switch validationType {
	case "StringInSlice", "StringNotInSlice":
		return extractStringInSliceParams(fn, validationType)
	case "IntBetween":
		return extractIntBetweenParams(fn)
	case "IntAtLeast":
		return extractIntAtLeastParam(fn)
	case "IntAtMost":
		return extractIntAtMostParam(fn)
	case "IntDivisibleBy":
		return extractIntDivisibleByParam(fn)
	case "FloatBetween":
		return extractFloatBetweenParams(fn)
	case "FloatAtLeast":
		return extractFloatAtLeastParam(fn)
	case "FloatAtMost":
		return extractFloatAtMostParam(fn)
	case "StringLenBetween":
		return extractStringLenBetweenParams(fn)
	case "StringMatch", "StringDoesNotMatch":
		return extractStringMatchParams(fn)
	case "IntInSlice", "IntNotInSlice":
		return extractIntInSliceParams(fn)
	case "IsURLWithScheme":
		return extractURLWithSchemeParams(fn)
	}

	return nil
}

// callValidator calls a validator function with a test value and returns errors
func callValidator(fn interface{}, testValue interface{}) []error {
	v := reflect.ValueOf(fn)

	defer func() {
		recover() // Ignore panics
	}()

	results := v.Call([]reflect.Value{
		reflect.ValueOf(testValue),
		reflect.ValueOf("field"),
	})

	if len(results) != 2 {
		return nil
	}

	errorsVal := results[1]
	if errorsVal.IsNil() || errorsVal.Len() == 0 {
		return nil
	}

	errs := make([]error, 0, errorsVal.Len())
	for i := 0; i < errorsVal.Len(); i++ {
		if e := errorsVal.Index(i); !e.IsNil() {
			if err, ok := e.Interface().(error); ok {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// parseQuotedSlice parses a Go %q-formatted string slice like: ["Asia Pacific" "Australia" "Europe"]
// which is what fmt.Sprintf("%q", []string{...}) produces.
func parseQuotedSlice(s string) []string {
	// Strip surrounding brackets
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		s = s[1 : len(s)-1]
	}
	s = strings.TrimSpace(s)

	var values []string
	for len(s) > 0 {
		s = strings.TrimSpace(s)
		if len(s) == 0 {
			break
		}
		if s[0] == '"' {
			// Find closing quote (handle escaped quotes)
			i := 1
			for i < len(s) {
				if s[i] == '\\' {
					i += 2
					continue
				}
				if s[i] == '"' {
					break
				}
				i++
			}
			if i < len(s) {
				values = append(values, s[1:i])
				s = s[i+1:]
			} else {
				break
			}
		} else {
			// Unquoted token (fallback for non-%q format)
			end := strings.IndexAny(s, " \t")
			if end == -1 {
				values = append(values, s)
				break
			}
			values = append(values, s[:end])
			s = s[end:]
		}
	}
	return values
}

// extractStringInSliceParams extracts the allowed/blocked values from a StringInSlice or StringNotInSlice validator
func extractStringInSliceParams(fn interface{}, validationType string) []interface{} {
	var errs []error

	if validationType == "StringInSlice" {
		// Use a value that definitely won't be in any reasonable slice
		errs = callValidator(fn, "__INVALID_TEST_VALUE_XYZ__")
	} else {
		// StringNotInSlice: we need a value that IS in the blocked list to get error.
		// Try calling with empty string which is often blocked.
		errs = callValidator(fn, "")
		if len(errs) == 0 {
			// Try other common values
			errs = callValidator(fn, " ")
		}
		if len(errs) == 0 {
			return nil
		}
	}

	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()

	// SDK format uses %q on the slice: `expected field to be one of ["val1" "val2"], got ...`
	// or for StringNotInSlice: `expected field to not be any of ["val1" "val2"], got ...`
	startIdx := strings.Index(msg, "[")
	endIdx := strings.LastIndex(msg, "]")

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil
	}

	bracketContent := msg[startIdx : endIdx+1]
	values := parseQuotedSlice(bracketContent)
	if len(values) == 0 {
		return nil
	}

	result := make([]interface{}, len(values))
	for i, v := range values {
		result[i] = v
	}
	return result
}

// extractIntBetweenParams extracts min and max from IntBetween validator
func extractIntBetweenParams(fn interface{}) []interface{} {
	errs := callValidator(fn, -999999999)
	if len(errs) == 0 {
		errs = callValidator(fn, 999999999)
	}
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()
	// "expected field to be in the range (1 - 100), got 999999999"
	re := regexp.MustCompile(`\((-?\d+)\s*-\s*(-?\d+)\)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 3 {
		var minVal, maxVal int
		fmt.Sscanf(matches[1], "%d", &minVal)
		fmt.Sscanf(matches[2], "%d", &maxVal)
		return []interface{}{minVal, maxVal}
	}

	return nil
}

// extractIntAtLeastParam extracts minimum value
func extractIntAtLeastParam(fn interface{}) []interface{} {
	errs := callValidator(fn, -999999999)
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()
	// "expected field to be at least (1), got -999999999"
	re := regexp.MustCompile(`at least \((-?\d+)\)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 2 {
		var minVal int
		fmt.Sscanf(matches[1], "%d", &minVal)
		return []interface{}{minVal}
	}

	return nil
}

// extractIntAtMostParam extracts maximum value
func extractIntAtMostParam(fn interface{}) []interface{} {
	errs := callValidator(fn, 999999999)
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()
	// "expected field to be at most (100), got 999999999"
	re := regexp.MustCompile(`at most \((-?\d+)\)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 2 {
		var maxVal int
		fmt.Sscanf(matches[1], "%d", &maxVal)
		return []interface{}{maxVal}
	}

	return nil
}

// extractIntDivisibleByParam extracts the divisor from an IntDivisibleBy validator
func extractIntDivisibleByParam(fn interface{}) []interface{} {
	// Test with 1 which is not divisible by most values > 1
	errs := callValidator(fn, 1)
	if len(errs) == 0 {
		// 1 is divisible by 1, try 3
		errs = callValidator(fn, 3)
	}
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()
	// "expected field to be divisible by 4, got: 1"
	re := regexp.MustCompile(`divisible by (\d+)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 2 {
		var divisor int
		fmt.Sscanf(matches[1], "%d", &divisor)
		return []interface{}{divisor}
	}

	return nil
}

// extractFloatBetweenParams extracts min and max for float validators
func extractFloatBetweenParams(fn interface{}) []interface{} {
	errs := callValidator(fn, -9999999.99)
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()
	// "expected field to be in the range (0.000000 - 1.000000)"
	re := regexp.MustCompile(`\((-?[0-9.]+)\s*-\s*(-?[0-9.]+)\)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 3 {
		var minVal, maxVal float64
		fmt.Sscanf(matches[1], "%f", &minVal)
		fmt.Sscanf(matches[2], "%f", &maxVal)
		return []interface{}{minVal, maxVal}
	}

	return nil
}

// extractFloatAtLeastParam extracts the minimum value from a FloatAtLeast validator
func extractFloatAtLeastParam(fn interface{}) []interface{} {
	errs := callValidator(fn, -9999999.99)
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()
	// "expected field to be at least (0.000000), got -9999999.990000"
	re := regexp.MustCompile(`at least \((-?[0-9.]+)\)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 2 {
		var minVal float64
		fmt.Sscanf(matches[1], "%f", &minVal)
		return []interface{}{minVal}
	}

	return nil
}

// extractFloatAtMostParam extracts the maximum value from a FloatAtMost validator
func extractFloatAtMostParam(fn interface{}) []interface{} {
	errs := callValidator(fn, 9999999.99)
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()
	// "expected field to be at most (100.000000), got 9999999.990000"
	re := regexp.MustCompile(`at most \((-?[0-9.]+)\)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 2 {
		var maxVal float64
		fmt.Sscanf(matches[1], "%f", &maxVal)
		return []interface{}{maxVal}
	}

	return nil
}

// extractStringLenBetweenParams extracts min and max length
func extractStringLenBetweenParams(fn interface{}) []interface{} {
	errs := callValidator(fn, "")
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()
	// "expected length of field to be in the range (3 - 24), got "
	re := regexp.MustCompile(`\((\d+)\s*-\s*(\d+)\)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 3 {
		var minVal, maxVal int
		fmt.Sscanf(matches[1], "%d", &minVal)
		fmt.Sscanf(matches[2], "%d", &maxVal)
		return []interface{}{minVal, maxVal}
	}

	return nil
}

// extractStringMatchParams extracts the regex pattern (and optional message) from a StringMatch validator.
// Returns [pattern] if a regex pattern can be extracted, or [errorMessage] otherwise.
func extractStringMatchParams(fn interface{}) []interface{} {
	errs := callValidator(fn, "")
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()

	// Try to extract regex pattern from: `expected value of "field" to match regular expression "PATTERN", got ""`
	re := regexp.MustCompile(`match regular expression "(.+)", got`)
	if matches := re.FindStringSubmatch(msg); len(matches) == 2 {
		return []interface{}{matches[1]}
	}

	// Custom message format: `invalid value for field (DESCRIPTION)`
	re2 := regexp.MustCompile(`\((.+)\)$`)
	if matches := re2.FindStringSubmatch(msg); len(matches) == 2 {
		return []interface{}{matches[1]}
	}

	// Return the raw error message as fallback
	return []interface{}{msg}
}

// extractIntInSliceParams extracts allowed integer values
func extractIntInSliceParams(fn interface{}) []interface{} {
	errs := callValidator(fn, -999999)
	if len(errs) == 0 {
		return nil
	}

	msg := errs[0].Error()

	// Parse error message with integer values
	startIdx := strings.Index(msg, "[")
	endIdx := strings.LastIndex(msg, "]")

	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		valueStr := msg[startIdx+1 : endIdx]
		values := strings.Fields(valueStr)

		result := make([]interface{}, 0, len(values))
		for _, v := range values {
			var intVal int
			if _, err := fmt.Sscanf(v, "%d", &intVal); err == nil {
				result = append(result, intVal)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	return nil
}

// extractURLWithSchemeParams extracts allowed URL schemes from an IsURLWithScheme validator
func extractURLWithSchemeParams(fn interface{}) []interface{} {
	// Call with a URL that has an invalid scheme to get the scheme error
	errs := callValidator(fn, "invalid-scheme://example.com/path")
	if len(errs) == 0 {
		return nil
	}

	// Find the error about schemes (not about empty URL or missing host)
	for _, err := range errs {
		msg := err.Error()
		// Format: `expected "field" to have a url with schema of: "http,https", got ...`
		re := regexp.MustCompile(`schema of: "([^"]+)"`)
		if matches := re.FindStringSubmatch(msg); len(matches) == 2 {
			schemes := strings.Split(matches[1], ",")
			result := make([]interface{}, len(schemes))
			for i, s := range schemes {
				result[i] = strings.TrimSpace(s)
			}
			return result
		}
	}

	return nil
}