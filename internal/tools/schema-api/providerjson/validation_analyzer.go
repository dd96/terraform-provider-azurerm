// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package providerjson

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// extractValidation attempts to extract validation information from ValidateFunc or ValidateDiagFunc
func extractValidation(validateFunc interface{}) *ValidationInfoJSON {
	if validateFunc == nil {
		return nil
	}

	// Get function name and try to analyze it
	funcName := getFunctionName(validateFunc)
	if funcName == "" {
		return nil
	}

	validation := &ValidationInfoJSON{}

	// Try to parse common validation patterns
	switch {
	// String validations
	case strings.Contains(funcName, "StringIsNotEmpty"):
		validation.Type = "StringIsNotEmpty"
		validation.Message = "string must not be empty"

	case strings.Contains(funcName, "StringIsNotWhiteSpace"):
		validation.Type = "StringIsNotWhiteSpace"
		validation.Message = "string must not be empty or whitespace"

	case strings.Contains(funcName, "StringIsEmpty"):
		validation.Type = "StringIsEmpty"
		validation.Message = "string must be empty"

	case strings.Contains(funcName, "StringIsWhiteSpace"):
		validation.Type = "StringIsWhiteSpace"
		validation.Message = "string must be whitespace only"

	case strings.Contains(funcName, "StringInSlice"):
		validation.Type = "StringInSlice"
		if params := extractValidationParameters(validateFunc, "StringInSlice"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "StringNotInSlice"):
		validation.Type = "StringNotInSlice"
		if params := extractValidationParameters(validateFunc, "StringNotInSlice"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "StringLenBetween"):
		validation.Type = "StringLenBetween"
		if params := extractValidationParameters(validateFunc, "StringLenBetween"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "StringMatch"):
		validation.Type = "StringMatch"
		if params := extractValidationParameters(validateFunc, "StringMatch"); params != nil {
			validation.Parameters = params
			if len(params) > 0 {
				if msg, ok := params[0].(string); ok {
					// Check if this looks like a regex pattern (not a plain description)
					if looksLikeRegex(msg) {
						validation.Pattern = msg
					} else {
						validation.Message = msg
					}
				}
			}
		}

	case strings.Contains(funcName, "StringDoesNotMatch"):
		validation.Type = "StringDoesNotMatch"
		if params := extractValidationParameters(validateFunc, "StringDoesNotMatch"); params != nil {
			validation.Parameters = params
			if len(params) > 0 {
				if msg, ok := params[0].(string); ok {
					if looksLikeRegex(msg) {
						validation.Pattern = msg
					} else {
						validation.Message = msg
					}
				}
			}
		}

	case strings.Contains(funcName, "StringDoesNotContainAny"):
		validation.Type = "StringDoesNotContainAny"
		if params := extractValidationParameters(validateFunc, "StringDoesNotContainAny"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "StringIsBase64"):
		validation.Type = "StringIsBase64"
		validation.Message = "string must be valid base64"

	case strings.Contains(funcName, "StringIsJSON"):
		validation.Type = "StringIsJSON"
		validation.Message = "string must be valid JSON"

	case strings.Contains(funcName, "StringIsValidRegExp"):
		validation.Type = "StringIsValidRegExp"
		validation.Message = "string must be a valid regular expression"

	// Integer validations
	case strings.Contains(funcName, "IntBetween"):
		validation.Type = "IntBetween"
		if params := extractValidationParameters(validateFunc, "IntBetween"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "IntAtLeast"):
		validation.Type = "IntAtLeast"
		if params := extractValidationParameters(validateFunc, "IntAtLeast"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "IntAtMost"):
		validation.Type = "IntAtMost"
		if params := extractValidationParameters(validateFunc, "IntAtMost"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "IntInSlice"):
		validation.Type = "IntInSlice"
		if params := extractValidationParameters(validateFunc, "IntInSlice"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "IntNotInSlice"):
		validation.Type = "IntNotInSlice"
		if params := extractValidationParameters(validateFunc, "IntNotInSlice"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "IntDivisibleBy"):
		validation.Type = "IntDivisibleBy"
		if params := extractValidationParameters(validateFunc, "IntDivisibleBy"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "IntPositive"):
		validation.Type = "IntPositive"
		validation.Message = "integer must be positive"

	// Float validations
	case strings.Contains(funcName, "FloatBetween"):
		validation.Type = "FloatBetween"
		if params := extractValidationParameters(validateFunc, "FloatBetween"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "FloatAtLeast"):
		validation.Type = "FloatAtLeast"
		if params := extractValidationParameters(validateFunc, "FloatAtLeast"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "FloatAtMost"):
		validation.Type = "FloatAtMost"
		if params := extractValidationParameters(validateFunc, "FloatAtMost"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "FloatInSlice"):
		validation.Type = "FloatInSlice"
		if params := extractValidationParameters(validateFunc, "FloatInSlice"); params != nil {
			validation.Parameters = params
		}

	// Network validations
	case strings.Contains(funcName, "IsIPv4Address"):
		validation.Type = "IsIPv4Address"
		validation.Message = "must be a valid IPv4 address"

	case strings.Contains(funcName, "IsIPv6Address"):
		validation.Type = "IsIPv6Address"
		validation.Message = "must be a valid IPv6 address"

	case strings.Contains(funcName, "IsIPAddress"):
		validation.Type = "IsIPAddress"
		validation.Message = "must be a valid IP address"

	case strings.Contains(funcName, "IsIPv4Range"):
		validation.Type = "IsIPv4Range"
		validation.Message = "must be a valid IPv4 range"

	case strings.Contains(funcName, "IsCIDR"):
		validation.Type = "IsCIDR"
		validation.Message = "must be a valid CIDR notation"

	case strings.Contains(funcName, "IsCIDRNetwork"):
		validation.Type = "IsCIDRNetwork"
		if params := extractValidationParameters(validateFunc, "IsCIDRNetwork"); params != nil {
			validation.Parameters = params
		} else if params := extractClosureParameters(validateFunc); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "IsMACAddress"):
		validation.Type = "IsMACAddress"
		validation.Message = "must be a valid MAC address"

	case strings.Contains(funcName, "IsPortNumber"):
		validation.Type = "IsPortNumber"
		validation.Message = "must be a valid port number (1-65535)"

	case strings.Contains(funcName, "IsPortNumberOrZero"):
		validation.Type = "IsPortNumberOrZero"
		validation.Message = "must be a valid port number (0-65535)"

	// URL validations
	case strings.Contains(funcName, "IsURLWithHTTPS"):
		validation.Type = "IsURLWithHTTPS"
		validation.Message = "must be a valid HTTPS URL"

	case strings.Contains(funcName, "IsURLWithHTTPorHTTPS"):
		validation.Type = "IsURLWithHTTPorHTTPS"
		validation.Message = "must be a valid HTTP or HTTPS URL"

	case strings.Contains(funcName, "IsURLWithScheme"):
		validation.Type = "IsURLWithScheme"
		if params := extractValidationParameters(validateFunc, "IsURLWithScheme"); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "IsURLWithPath"):
		validation.Type = "IsURLWithPath"
		validation.Message = "must be a valid URL with a path"

	// UUID and Time validations
	case strings.Contains(funcName, "IsUUID"):
		validation.Type = "IsUUID"
		validation.Message = "must be a valid UUID"

	case strings.Contains(funcName, "IsRFC3339Time"):
		validation.Type = "IsRFC3339Time"
		validation.Message = "must be a valid RFC3339 timestamp"

	case strings.Contains(funcName, "IsDayOfTheWeek"):
		validation.Type = "IsDayOfTheWeek"
		validation.Message = "must be a valid day of the week"

	case strings.Contains(funcName, "IsMonth"):
		validation.Type = "IsMonth"
		validation.Message = "must be a valid month name"

	// Composite validations
	case strings.Contains(funcName, "All"):
		validation.Type = "Composite"
		validation.Composite = "all"
		validation.Message = "all validators must pass"

	case strings.Contains(funcName, "Any"):
		validation.Type = "Composite"
		validation.Composite = "any"
		validation.Message = "at least one validator must pass"

	case strings.Contains(funcName, "None"):
		validation.Type = "Composite"
		validation.Composite = "none"
		validation.Message = "none of the validators should pass"

	case strings.Contains(funcName, "NoZeroValues"):
		validation.Type = "NoZeroValues"
		validation.Message = "must not be a zero value"

	// Map validations
	case strings.Contains(funcName, "MapKeyLenBetween"):
		validation.Type = "MapKeyLenBetween"
		if params := extractClosureParameters(validateFunc); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "MapValueLenBetween"):
		validation.Type = "MapValueLenBetween"
		if params := extractClosureParameters(validateFunc); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "MapKeyMatch"):
		validation.Type = "MapKeyMatch"
		if params := extractClosureParameters(validateFunc); params != nil {
			validation.Parameters = params
		}

	case strings.Contains(funcName, "MapValueMatch"):
		validation.Type = "MapValueMatch"
		if params := extractClosureParameters(validateFunc); params != nil {
			validation.Parameters = params
		}

	// List validations
	case strings.Contains(funcName, "ListOfUniqueStrings"):
		validation.Type = "ListOfUniqueStrings"
		validation.Message = "list must contain unique strings"

	// Custom/Service-specific validations
	default:
		validation.Type = "Custom"
		// Use a cleaned-up function name (drop module path prefix)
		validation.Message = cleanFuncName(funcName)

		// Attempt to parse source code for detailed validation information
		if sourceInfo := parseValidatorSource(validateFunc); sourceInfo != nil {
			if sourceInfo.Pattern != "" {
				validation.Pattern = sourceInfo.Pattern
			}

			constraints := make(map[string]interface{})
			if sourceInfo.MinLength != nil {
				constraints["minLength"] = *sourceInfo.MinLength
			}
			if sourceInfo.MaxLength != nil {
				constraints["maxLength"] = *sourceInfo.MaxLength
			}
			if sourceInfo.MinValue != nil {
				constraints["minValue"] = *sourceInfo.MinValue
			}
			if sourceInfo.MaxValue != nil {
				constraints["maxValue"] = *sourceInfo.MaxValue
			}
			if sourceInfo.AllowedChars != "" {
				constraints["allowedChars"] = sourceInfo.AllowedChars
			}
			if sourceInfo.Format != "" {
				constraints["format"] = sourceInfo.Format
			}
			if len(sourceInfo.Requirements) > 0 {
				constraints["requirements"] = sourceInfo.Requirements
			}
			if len(sourceInfo.BlockedValues) > 0 {
				constraints["blockedValues"] = sourceInfo.BlockedValues
			}
			if sourceInfo.CaseSensitive != nil {
				constraints["caseSensitive"] = *sourceInfo.CaseSensitive
			}

			if len(constraints) > 0 {
				validation.Parameters = []interface{}{constraints}
			}

			if sourceInfo.ErrorMessage != "" {
				validation.Message = sourceInfo.ErrorMessage
			}
		}
	}

	// Generate a Terraform validation hint
	validation.TerraformHint = generateTerraformHint(validation)

	return validation
}

// cleanFuncName strips the Go module path prefix and returns a clean name like "validate.StorageAccountName"
func cleanFuncName(funcName string) string {
	// funcName looks like: "github.com/hashicorp/terraform-provider-azurerm/internal/services/storage/validate.StorageAccountName"
	// We want: "validate.StorageAccountName"
	lastSlash := strings.LastIndex(funcName, "/")
	if lastSlash != -1 {
		return funcName[lastSlash+1:]
	}
	return funcName
}

// looksLikeRegex returns true if the string appears to be a regex pattern rather than a human description
func looksLikeRegex(s string) bool {
	// Regex patterns typically contain anchors, character classes, quantifiers
	for _, ch := range []string{"^", "$", "[", "\\", "(", ")", "+", "*", "?", "{", "}"} {
		if strings.Contains(s, ch) {
			return true
		}
	}
	return false
}

// generateTerraformHint produces a suggested Terraform validation condition for the given validation
func generateTerraformHint(v *ValidationInfoJSON) string {
	if v == nil {
		return ""
	}

	switch v.Type {
	case "StringIsNotEmpty":
		return `length(var.value) > 0`

	case "StringIsNotWhiteSpace":
		return `length(trimspace(var.value)) > 0`

	case "StringIsEmpty":
		return `length(var.value) == 0`

	case "StringIsBase64":
		return `can(base64decode(var.value))`

	case "StringIsJSON":
		return `can(jsondecode(var.value))`

	case "StringIsValidRegExp":
		return `can(regex(var.value, ""))`

	case "StringInSlice":
		if len(v.Parameters) > 0 {
			vals := formatStringSlice(v.Parameters)
			return fmt.Sprintf(`contains(%s, var.value)`, vals)
		}
		return `contains(["..."], var.value)`

	case "StringNotInSlice":
		if len(v.Parameters) > 0 {
			vals := formatStringSlice(v.Parameters)
			return fmt.Sprintf(`!contains(%s, var.value)`, vals)
		}
		return `!contains(["..."], var.value)`

	case "StringLenBetween":
		if len(v.Parameters) == 2 {
			min := formatNumber(v.Parameters[0])
			max := formatNumber(v.Parameters[1])
			return fmt.Sprintf(`length(var.value) >= %s && length(var.value) <= %s`, min, max)
		}
		return `length(var.value) >= min && length(var.value) <= max`

	case "StringMatch":
		if v.Pattern != "" {
			return fmt.Sprintf(`can(regex(%q, var.value))`, v.Pattern)
		}
		return `can(regex("...", var.value))`

	case "StringDoesNotMatch":
		if v.Pattern != "" {
			return fmt.Sprintf(`!can(regex(%q, var.value))`, v.Pattern)
		}
		return `!can(regex("...", var.value))`

	case "IntBetween":
		if len(v.Parameters) == 2 {
			min := formatNumber(v.Parameters[0])
			max := formatNumber(v.Parameters[1])
			return fmt.Sprintf(`var.value >= %s && var.value <= %s`, min, max)
		}
		return `var.value >= min && var.value <= max`

	case "IntAtLeast":
		if len(v.Parameters) == 1 {
			return fmt.Sprintf(`var.value >= %s`, formatNumber(v.Parameters[0]))
		}
		return `var.value >= min`

	case "IntAtMost":
		if len(v.Parameters) == 1 {
			return fmt.Sprintf(`var.value <= %s`, formatNumber(v.Parameters[0]))
		}
		return `var.value <= max`

	case "IntDivisibleBy":
		if len(v.Parameters) == 1 {
			return fmt.Sprintf(`var.value %% %s == 0`, formatNumber(v.Parameters[0]))
		}
		return `var.value % divisor == 0`

	case "IntPositive":
		return `var.value > 0`

	case "IntInSlice":
		if len(v.Parameters) > 0 {
			vals := formatIntSlice(v.Parameters)
			return fmt.Sprintf(`contains(%s, var.value)`, vals)
		}
		return `contains([...], var.value)`

	case "IntNotInSlice":
		if len(v.Parameters) > 0 {
			vals := formatIntSlice(v.Parameters)
			return fmt.Sprintf(`!contains(%s, var.value)`, vals)
		}
		return `!contains([...], var.value)`

	case "FloatBetween":
		if len(v.Parameters) == 2 {
			min := formatNumber(v.Parameters[0])
			max := formatNumber(v.Parameters[1])
			return fmt.Sprintf(`var.value >= %s && var.value <= %s`, min, max)
		}
		return `var.value >= min && var.value <= max`

	case "FloatAtLeast":
		if len(v.Parameters) == 1 {
			return fmt.Sprintf(`var.value >= %s`, formatNumber(v.Parameters[0]))
		}
		return `var.value >= min`

	case "FloatAtMost":
		if len(v.Parameters) == 1 {
			return fmt.Sprintf(`var.value <= %s`, formatNumber(v.Parameters[0]))
		}
		return `var.value <= max`

	case "IsIPv4Address":
		return `can(cidrnetmask("${var.value}/32"))`

	case "IsIPv6Address":
		return `can(regex("^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$", var.value))`

	case "IsIPAddress":
		return `can(cidrnetmask("${var.value}/32")) || can(regex("^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$", var.value))`

	case "IsCIDR":
		return `can(cidrhost(var.value, 0))`

	case "IsCIDRNetwork":
		if len(v.Parameters) == 2 {
			min := formatNumber(v.Parameters[0])
			max := formatNumber(v.Parameters[1])
			return fmt.Sprintf(`can(cidrhost(var.value, 0)) && tonumber(split("/", var.value)[1]) >= %s && tonumber(split("/", var.value)[1]) <= %s`, min, max)
		}
		return `can(cidrhost(var.value, 0))`

	case "IsMACAddress":
		return `can(regex("^([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}$", var.value))`

	case "IsPortNumber":
		return `tonumber(var.value) >= 1 && tonumber(var.value) <= 65535`

	case "IsPortNumberOrZero":
		return `tonumber(var.value) >= 0 && tonumber(var.value) <= 65535`

	case "IsURLWithHTTPS":
		return `can(regex("^https://", var.value))`

	case "IsURLWithHTTPorHTTPS":
		return `can(regex("^https?://", var.value))`

	case "IsURLWithScheme":
		if len(v.Parameters) > 0 {
			vals := formatStringSlice(v.Parameters)
			return fmt.Sprintf(`can(regex("^(" + join("|", %s) + ")://", var.value))`, vals)
		}
		return `can(regex("^(scheme)://", var.value))`

	case "IsURLWithPath":
		return `can(regex("^https?://.+/.+", var.value))`

	case "IsUUID":
		return `can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", var.value))`

	case "IsRFC3339Time":
		return `can(formatdate("", var.value))`

	case "NoZeroValues":
		return `var.value != null && var.value != "" && var.value != 0 && var.value != false`

	case "Custom":
		if v.Pattern != "" {
			return fmt.Sprintf(`can(regex(%q, var.value))`, v.Pattern)
		}
		// Try to build a hint from constraints in parameters
		if len(v.Parameters) > 0 {
			if constraints, ok := v.Parameters[0].(map[string]interface{}); ok {
				return hintFromConstraints(constraints)
			}
		}
	}

	return ""
}

// formatStringSlice formats a []interface{} of strings as a Terraform list literal
func formatStringSlice(params []interface{}) string {
	parts := make([]string, 0, len(params))
	for _, p := range params {
		parts = append(parts, fmt.Sprintf("%q", fmt.Sprint(p)))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// formatIntSlice formats a []interface{} of ints as a Terraform list literal
func formatIntSlice(params []interface{}) string {
	parts := make([]string, 0, len(params))
	for _, p := range params {
		parts = append(parts, formatNumber(p))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// formatNumber formats a numeric interface{} value for Terraform expressions
func formatNumber(v interface{}) string {
	switch n := v.(type) {
	case int:
		return fmt.Sprintf("%d", n)
	case int64:
		return fmt.Sprintf("%d", n)
	case float64:
		// Remove trailing zeros for cleanliness
		s := fmt.Sprintf("%g", n)
		return s
	case float32:
		s := fmt.Sprintf("%g", n)
		return s
	default:
		return fmt.Sprint(v)
	}
}

// hintFromConstraints builds a partial Terraform hint from source-extracted constraints
func hintFromConstraints(c map[string]interface{}) string {
	var parts []string

	if minLen, ok := c["minLength"]; ok {
		if maxLen, ok2 := c["maxLength"]; ok2 {
			parts = append(parts, fmt.Sprintf("length(var.value) >= %v && length(var.value) <= %v", minLen, maxLen))
		} else {
			parts = append(parts, fmt.Sprintf("length(var.value) >= %v", minLen))
		}
	} else if maxLen, ok := c["maxLength"]; ok {
		parts = append(parts, fmt.Sprintf("length(var.value) <= %v", maxLen))
	}

	if len(parts) > 0 {
		return strings.Join(parts, " && ")
	}
	return ""
}

// getFunctionName returns the name of a function
func getFunctionName(i interface{}) string {
	if i == nil {
		return ""
	}
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// extractClosureParameters attempts to extract parameters from a closure by test-calling it
func extractClosureParameters(fn interface{}) []interface{} {
	if fn == nil {
		return nil
	}

	funcName := getFunctionName(fn)

	// For IsCIDRNetwork, try to extract min/max prefix bits from error message
	if strings.Contains(funcName, "IsCIDRNetwork") {
		return extractCIDRNetworkParams(fn)
	}

	// For other closures, return nil (we cannot reliably extract parameters)
	return nil
}

// extractCIDRNetworkParams extracts the min/max prefix bits from an IsCIDRNetwork validator
func extractCIDRNetworkParams(fn interface{}) []interface{} {
	// Call with a /0 CIDR which is likely outside the valid range for most uses
	errs := callValidator(fn, "0.0.0.0/0")
	if len(errs) == 0 {
		// Try /33 which is always invalid
		errs = callValidator(fn, "0.0.0.0/33")
	}
	if len(errs) == 0 {
		return nil
	}

	for _, err := range errs {
		msg := err.Error()
		// Try to find prefix range in error message
		// Common format: "expected prefix between X and Y"
		var minBits, maxBits int
		if n, _ := fmt.Sscanf(msg, "expected prefix between %d and %d", &minBits, &maxBits); n == 2 {
			return []interface{}{minBits, maxBits}
		}
		// Try another format
		if n, _ := fmt.Sscanf(msg, "network's CIDR bits must be between %d and %d", &minBits, &maxBits); n == 2 {
			return []interface{}{minBits, maxBits}
		}
	}

	return nil
}