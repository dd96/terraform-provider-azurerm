// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package providerjson

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// SourceValidationInfo contains validation information extracted from source code
type SourceValidationInfo struct {
	Pattern         string   `json:"pattern,omitempty"`          // Regex pattern
	MinLength       *int     `json:"minLength,omitempty"`        // Minimum length constraint
	MaxLength       *int     `json:"maxLength,omitempty"`        // Maximum length constraint
	MinValue        *int     `json:"minValue,omitempty"`         // Minimum numeric value
	MaxValue        *int     `json:"maxValue,omitempty"`         // Maximum numeric value
	AllowedChars    string   `json:"allowedChars,omitempty"`     // Description of allowed characters
	Format          string   `json:"format,omitempty"`           // Format description (e.g., "email", "time", "CIDR")
	ErrorMessage    string   `json:"errorMessage,omitempty"`     // Example error message
	Requirements    []string `json:"requirements,omitempty"`     // List of requirements
	BlockedValues   []string `json:"blockedValues,omitempty"`    // Values that are not allowed
	CaseSensitive   *bool    `json:"caseSensitive,omitempty"`    // Whether validation is case-sensitive
}

// parseValidatorSource attempts to extract validation information from a function's source code
func parseValidatorSource(validateFunc interface{}) *SourceValidationInfo {
	if validateFunc == nil {
		return nil
	}

	// Get function pointer and file location
	funcPtr := runtime.FuncForPC(reflect.ValueOf(validateFunc).Pointer())
	if funcPtr == nil {
		return nil
	}

	funcName := funcPtr.Name()
	file, line := funcPtr.FileLine(funcPtr.Entry())
	if file == "" {
		return nil
	}

	// Special handling for vendor validators
	if strings.Contains(file, "/vendor/") {
		return parseVendorValidator(funcName, file, line)
	}

	// Parse the source file
	info, err := parseSourceFile(file, line)
	if err != nil {
		return nil
	}

	return info
}

// parseVendorValidator handles validators from vendor packages
func parseVendorValidator(funcName, file string, line int) *SourceValidationInfo {
	info := &SourceValidationInfo{}

	// Handle ID validators (ValidateXXXID pattern)
	if strings.Contains(funcName, "Validate") && strings.HasSuffix(funcName, "ID") {
		// Extract resource type from function name
		// e.g., "github.com/.../commonids.ValidateSubnetID" -> "SubnetID"
		parts := strings.Split(funcName, ".")
		if len(parts) > 0 {
			validatorName := parts[len(parts)-1]
			resourceType := strings.TrimPrefix(validatorName, "Validate")

			info.Format = "Azure Resource ID"
			info.Requirements = []string{
				"must be a valid " + resourceType,
				"format validated by Azure SDK parser",
			}

			// Try to extract ID format from source if possible
			if idFormat := extractIDFormatFromSource(file, line); idFormat != "" {
				info.Pattern = idFormat
			}
		}
		return info
	}

	// Handle specific well-known validators
	switch {
	case strings.Contains(funcName, "resourcegroups.ValidateName"):
		info.Pattern = "^[-\\w._()]+$"
		info.MaxLength = intPtr(90)
		info.AllowedChars = "alphanumeric, dash, underscore, period, parentheses"
		info.Requirements = []string{"cannot end with period"}
		info.ErrorMessage = "resource group name can be up to 90 characters and can contain alphanumerics, dash, underscore, period, and parentheses"

	case strings.Contains(funcName, "tags.Validate"):
		info.Requirements = []string{
			"maximum 50 tags allowed",
			"tag keys max 512 characters",
			"tag values max 256 characters",
		}
		info.ErrorMessage = "tag validation enforces Azure tag limits"

	case strings.Contains(funcName, "tags.ValidateHasLowerCaseKeys"):
		info.Requirements = []string{
			"all tag keys must be lowercase",
		}
		caseSensitive := true
		info.CaseSensitive = &caseSensitive

	case strings.Contains(funcName, "location.EnhancedValidate"):
		info.Requirements = []string{
			"must be a valid Azure location",
			"normalized to lowercase for comparison",
		}
		info.ErrorMessage = "location must be a valid Azure region (e.g., eastus, westeurope)"

	default:
		// For other vendor validators, try parsing the source file
		if parsed, err := parseSourceFile(file, line); err == nil {
			return parsed
		}
	}

	return info
}

// extractIDFormatFromSource attempts to extract ID format template from source
func extractIDFormatFromSource(filePath string, targetLine int) string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return ""
	}

	var idFormat string

	// Look for ID() method specifically - it contains the format string
	ast.Inspect(node, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			// Only look at ID() method
			if funcDecl.Name.Name == "ID" {
				// Look for variable declarations with string literals (fmtString := "...")
				ast.Inspect(funcDecl.Body, func(n2 ast.Node) bool {
					// Look for assignment statements
					if assign, ok := n2.(*ast.AssignStmt); ok {
						for i, rhs := range assign.Rhs {
							if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
								// Check if this is assigned to a variable named fmtString or similar
								if i < len(assign.Lhs) {
									if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
										if strings.Contains(strings.ToLower(ident.Name), "fmt") ||
										   strings.Contains(strings.ToLower(ident.Name), "format") ||
										   strings.Contains(strings.ToLower(ident.Name), "string") {
											idFormat = strings.Trim(lit.Value, "`\"")
											return false
										}
									}
								}
							}
						}
					}

					// Also look for direct fmt.Sprintf calls
					if call, ok := n2.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" && sel.Sel.Name == "Sprintf" {
								// Extract format string (first argument)
								if len(call.Args) > 0 {
									if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
										idFormat = strings.Trim(lit.Value, "`\"")
										return false
									}
									// Format string might be a variable reference
									if _, ok := call.Args[0].(*ast.Ident); ok && idFormat == "" {
										// We already extracted it from the assignment above
										return false
									}
								}
							}
						}
					}
					return true
				})
			}
		}
		return idFormat == ""
	})

	return idFormat
}

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

// parseSourceFile parses a Go source file and extracts validation information
func parseSourceFile(filePath string, targetLine int) (*SourceValidationInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	info := &SourceValidationInfo{}

	// Find the function containing the target line
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Check if this function contains our target line
			if fset.Position(x.Pos()).Line <= targetLine && fset.Position(x.End()).Line >= targetLine {
				extractFromFunction(x, info, fset)
				return false
			}
		}
		return true
	})

	return info, nil
}

// extractFromFunction extracts validation information from a function declaration
func extractFromFunction(funcDecl *ast.FuncDecl, info *SourceValidationInfo, fset *token.FileSet) {
	// Walk through the function body
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			// Look for regexp.MustCompile or regexp.Compile calls
			if isRegexpCompile(x) {
				if pattern := extractRegexPattern(x); pattern != "" {
					info.Pattern = pattern
					analyzeRegexPattern(pattern, info)
				}
			}

			// Look for strings.Contains, HasPrefix, HasSuffix calls
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "strings" {
					extractStringChecks(sel.Sel.Name, x, info)
				}
			}

		case *ast.BinaryExpr:
			// Look for comparison operations (len checks, numeric comparisons)
			extractComparisons(x, info)

		case *ast.BasicLit:
			// Look for string literals that might be error messages
			if x.Kind == token.STRING {
				msg := strings.Trim(x.Value, "`\"")
				if isErrorMessage(msg) && info.ErrorMessage == "" {
					info.ErrorMessage = msg
				}
			}
		}
		return true
	})
}

// isRegexpCompile checks if a call expression is regexp.MustCompile or regexp.Compile
func isRegexpCompile(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name == "regexp" && (sel.Sel.Name == "MustCompile" || sel.Sel.Name == "Compile")
		}
	}
	return false
}

// extractRegexPattern extracts the pattern string from a regexp.MustCompile call
func extractRegexPattern(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}

	if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
		// Remove quotes and backticks
		pattern := lit.Value
		if len(pattern) >= 2 {
			if pattern[0] == '`' {
				return pattern[1 : len(pattern)-1]
			}
			if pattern[0] == '"' {
				// Handle escaped characters
				unquoted, err := strconv.Unquote(pattern)
				if err == nil {
					return unquoted
				}
			}
		}
	}

	return ""
}

// analyzeRegexPattern extracts validation constraints from a regex pattern
func analyzeRegexPattern(pattern string, info *SourceValidationInfo) {
	// Extract length constraints from quantifiers like {min,max}
	lengthRe := regexp.MustCompile(`\{(\d+),(\d+)\}`)
	if matches := lengthRe.FindStringSubmatch(pattern); len(matches) == 3 {
		if min, err := strconv.Atoi(matches[1]); err == nil {
			info.MinLength = &min
		}
		if max, err := strconv.Atoi(matches[2]); err == nil {
			info.MaxLength = &max
		}
	}

	// Extract minimum-only length {min,}
	minLengthRe := regexp.MustCompile(`\{(\d+),\}`)
	if matches := minLengthRe.FindStringSubmatch(pattern); len(matches) == 2 {
		if min, err := strconv.Atoi(matches[1]); err == nil {
			info.MinLength = &min
		}
	}

	// Extract exact length {n}
	exactLengthRe := regexp.MustCompile(`\{(\d+)\}`)
	if matches := exactLengthRe.FindStringSubmatch(pattern); len(matches) == 2 {
		if length, err := strconv.Atoi(matches[1]); err == nil {
			info.MinLength = &length
			info.MaxLength = &length
		}
	}

	// Analyze character classes to determine allowed characters
	info.AllowedChars = describeCharacterSet(pattern)

	// Detect common formats
	info.Format = detectFormat(pattern)

	// Extract anchor requirements (must start/end with specific characters)
	extractAnchorRequirements(pattern, info)
}

// describeCharacterSet analyzes regex pattern to describe allowed characters
func describeCharacterSet(pattern string) string {
	var descriptions []string

	if strings.Contains(pattern, "[a-z]") {
		descriptions = append(descriptions, "lowercase letters")
	}
	if strings.Contains(pattern, "[A-Z]") {
		descriptions = append(descriptions, "uppercase letters")
	}
	if strings.Contains(pattern, "[0-9]") || strings.Contains(pattern, "\\d") {
		descriptions = append(descriptions, "digits")
	}
	if strings.Contains(pattern, "-") && !strings.Contains(pattern, "\\-") {
		descriptions = append(descriptions, "hyphens")
	}
	if strings.Contains(pattern, "_") {
		descriptions = append(descriptions, "underscores")
	}
	if strings.Contains(pattern, "\\.") {
		descriptions = append(descriptions, "dots")
	}

	// Check for alphanumeric patterns
	if strings.Contains(pattern, "[a-zA-Z0-9]") || strings.Contains(pattern, "[0-9a-zA-Z]") {
		return "alphanumeric characters"
	}
	if strings.Contains(pattern, "[a-z0-9]") {
		return "lowercase alphanumeric characters"
	}
	if strings.Contains(pattern, "[A-Z0-9]") {
		return "uppercase alphanumeric characters"
	}

	if len(descriptions) > 0 {
		return strings.Join(descriptions, ", ")
	}

	return ""
}

// detectFormat identifies common format patterns
func detectFormat(pattern string) string {
	patterns := map[string]string{
		`\w+([-+.']\w+)*@\w+`:                    "email",
		`([0-9]{1,3}\.){3}[0-9]{1,3}`:            "IPv4",
		`([0-9a-fA-F]{1,4}:){7}`:                 "IPv6",
		`/([0-9]|[1-2][0-9]|3[0-2])`:             "CIDR",
		`([0-9]{2}):([0-5][0-9]):([0-5][0-9])`:   "time (HH:MM:SS)",
		`([0-5][0-9])$`:                          "time (HH:MM)", // HH:MM format
		`[0-9]{4}-[0-9]{2}-[0-9]{2}`:             "date (YYYY-MM-DD)",
		`^P[0-9]+[YMWD]`:                          "ISO8601 duration",
		`[0-9]+[smh]$`:                            "duration with unit (s/m/h)",
		`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}`:          "UUID",
		`^https?://`:                              "URL",
	}

	for patternFragment, formatName := range patterns {
		if strings.Contains(pattern, patternFragment) {
			return formatName
		}
	}

	return ""
}

// extractAnchorRequirements extracts start/end requirements from anchored patterns
func extractAnchorRequirements(pattern string, info *SourceValidationInfo) {
	// Pattern starts with specific character requirement
	if strings.HasPrefix(pattern, "^[a-z]") {
		info.Requirements = append(info.Requirements, "must start with lowercase letter")
	} else if strings.HasPrefix(pattern, "^[A-Z]") {
		info.Requirements = append(info.Requirements, "must start with uppercase letter")
	} else if strings.HasPrefix(pattern, "^[a-zA-Z]") {
		info.Requirements = append(info.Requirements, "must start with letter")
	} else if strings.HasPrefix(pattern, "^[0-9]") {
		info.Requirements = append(info.Requirements, "must start with digit")
	}

	// Pattern ends with specific character requirement
	if strings.HasSuffix(pattern, "[a-z]$") {
		info.Requirements = append(info.Requirements, "must end with lowercase letter")
	} else if strings.HasSuffix(pattern, "[A-Z]$") {
		info.Requirements = append(info.Requirements, "must end with uppercase letter")
	} else if strings.HasSuffix(pattern, "[a-zA-Z]$") {
		info.Requirements = append(info.Requirements, "must end with letter")
	} else if strings.HasSuffix(pattern, "[0-9]$") {
		info.Requirements = append(info.Requirements, "must end with digit")
	}
}

// extractStringChecks extracts validation requirements from strings package calls
func extractStringChecks(methodName string, call *ast.CallExpr, info *SourceValidationInfo) {
	switch methodName {
	case "Contains":
		if len(call.Args) >= 2 {
			if lit, ok := call.Args[1].(*ast.BasicLit); ok {
				substring := strings.Trim(lit.Value, "`\"")
				info.Requirements = append(info.Requirements, "must not contain '"+substring+"'")
			}
		}
	case "HasPrefix":
		if len(call.Args) >= 2 {
			if lit, ok := call.Args[1].(*ast.BasicLit); ok {
				prefix := strings.Trim(lit.Value, "`\"")
				info.Requirements = append(info.Requirements, "must not start with '"+prefix+"'")
			}
		}
	case "HasSuffix":
		if len(call.Args) >= 2 {
			if lit, ok := call.Args[1].(*ast.BasicLit); ok {
				suffix := strings.Trim(lit.Value, "`\"")
				info.Requirements = append(info.Requirements, "must not end with '"+suffix+"'")
			}
		}
	}
}

// extractComparisons extracts numeric and length comparisons
func extractComparisons(expr *ast.BinaryExpr, info *SourceValidationInfo) {
	// Check if this is a len() comparison
	if call, ok := expr.X.(*ast.CallExpr); ok {
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "len" {
			// This is a len(x) comparison - ONLY set length values
			// Note: These comparisons are typically in error conditions
			// e.g., "if len(v) < 4 { error }" means minLength is 4
			// e.g., "if len(v) > 42 { error }" means maxLength is 42
			if lit, ok := expr.Y.(*ast.BasicLit); ok && lit.Kind == token.INT {
				value, _ := strconv.Atoi(lit.Value)
				switch expr.Op {
				case token.LSS: // len(v) < N -> error, so valid is >= N (minLength = N)
					if info.MinLength == nil {
						info.MinLength = &value
					}
				case token.LEQ: // len(v) <= N -> error, so valid is > N (minLength = N+1)
					min := value + 1
					if info.MinLength == nil {
						info.MinLength = &min
					}
				case token.GTR: // len(v) > N -> error, so valid is <= N (maxLength = N)
					if info.MaxLength == nil {
						info.MaxLength = &value
					}
				case token.GEQ: // len(v) >= N -> error, so valid is < N (maxLength = N-1)
					max := value - 1
					if info.MaxLength == nil {
						info.MaxLength = &max
					}
				}
			}
			return // Don't process as numeric comparison
		}
	}

	// Check for direct numeric comparisons (only if not a len() comparison)
	// This would be for things like: if value < 100 or if port > 1024
	// Don't use this section - it conflicts with len() comparisons above
	// Most numeric validators use IntBetween/IntAtLeast which are already handled
}

// isErrorMessage determines if a string looks like an error message
func isErrorMessage(s string) bool {
	lowerS := strings.ToLower(s)
	return strings.Contains(lowerS, "expected") ||
		strings.Contains(lowerS, "must") ||
		strings.Contains(lowerS, "invalid") ||
		strings.Contains(lowerS, "error") ||
		strings.Contains(lowerS, "cannot") ||
		strings.Contains(lowerS, "should")
}

// getValidatorSourcePath attempts to find the source file path for a validator function
func getValidatorSourcePath(funcName string) string {
	// Extract package path from function name
	// Example: "github.com/hashicorp/terraform-provider-azurerm/internal/services/storage/validate.StorageAccountName"

	lastSlash := strings.LastIndex(funcName, "/")
	if lastSlash == -1 {
		return ""
	}

	lastDot := strings.LastIndex(funcName, ".")
	if lastDot == -1 || lastDot < lastSlash {
		return ""
	}

	packagePath := funcName[:lastDot]

	// Convert package path to file path
	// Remove module prefix and convert to relative path
	if strings.HasPrefix(packagePath, "github.com/hashicorp/terraform-provider-azurerm/") {
		relPath := strings.TrimPrefix(packagePath, "github.com/hashicorp/terraform-provider-azurerm/")
		return filepath.Join(relPath)
	}

	return ""
}
