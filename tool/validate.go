package tool

import (
	"fmt"
	"math"
	"strings"

	"regexp"
)

// Drone API request paths are assembled by drone-go with plain string
// formatting (for example "/api/repos/%s/%s"), so any value that reaches it
// unvalidated can reshape the request path or inject query parameters.
// The helpers in this file restrict every caller supplied identifier to a
// conservative character set before it is forwarded to the Drone client.
const (
	maxSegmentLength = 100
	maxBranchLength  = 200
	maxCommitLength  = 200
	maxParams        = 50
	maxParamLength   = 1024
	maxNumber        = 1_000_000_000
)

var (
	// segmentRe matches one URL path segment. It must start with an
	// alphanumeric character, which rejects "", ".", ".." and any value
	// containing "/", "\", "?", "#", "%", "&", "=" or control characters.
	segmentRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

	// branchRe allows git branch and ref names such as "feature/x".
	// Each "/" separated segment is validated separately below.
	branchRe = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

// firstErr returns the first non-nil error, which keeps Validate
// implementations free of repetitive if blocks.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// ValidateSegment rejects values that could alter the Drone API request path.
func ValidateSegment(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(value) > maxSegmentLength {
		return fmt.Errorf("%s must not exceed %d characters", field, maxSegmentLength)
	}
	if !segmentRe.MatchString(value) {
		return fmt.Errorf("%s may only contain letters, digits, dots, underscores and hyphens, and must start with a letter or digit", field)
	}
	return nil
}

// ValidateOptionalSegment validates a value that is allowed to be absent.
func ValidateOptionalSegment(field, value string) error {
	if value == "" {
		return nil
	}
	return ValidateSegment(field, value)
}

// ValidateBranch validates a git branch name, which may contain slashes.
func ValidateBranch(field, value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxBranchLength {
		return fmt.Errorf("%s must not exceed %d characters", field, maxBranchLength)
	}
	if !branchRe.MatchString(value) {
		return fmt.Errorf("%s contains invalid characters", field)
	}
	if strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "//") {
		return fmt.Errorf("%s contains an empty path segment", field)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("%s contains a relative path segment", field)
		}
	}
	return nil
}

// ValidateNumber converts a JSON number argument into a bounded positive int.
// The MCP schema advertises these fields as numbers, so the whole-number and
// range checks happen here.
func ValidateNumber(field string, value float64) (int, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, fmt.Errorf("%s must be a whole number", field)
	}
	if value < 1 || value > maxNumber {
		return 0, fmt.Errorf("%s must be between 1 and %d", field, maxNumber)
	}
	return int(value), nil
}

// ValidateParams bounds user supplied build parameters. The values are
// url-encoded by drone-go, so this is a size guard rather than an escaping
// guard.
func ValidateParams(field string, params map[string]string) error {
	if len(params) > maxParams {
		return fmt.Errorf("%s must not contain more than %d entries", field, maxParams)
	}
	for key, value := range params {
		if key == "" {
			return fmt.Errorf("%s contains an empty key", field)
		}
		if len(key) > maxParamLength || len(value) > maxParamLength {
			return fmt.Errorf("%s keys and values must not exceed %d characters", field, maxParamLength)
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must not contain newlines", field)
		}
	}
	return nil
}

// ValidateCommit validates an optional commit SHA, tag or ref name.
func ValidateCommit(field, value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxCommitLength {
		return fmt.Errorf("%s must not exceed %d characters", field, maxCommitLength)
	}
	if !branchRe.MatchString(value) {
		return fmt.Errorf("%s contains invalid characters", field)
	}
	return nil
}

// Additional size limits for values that travel in request bodies rather than
// in the request path.
const (
	maxSecretValueLength = 64 << 10
	maxTemplateLength    = 512 << 10
	maxEmailLength       = 320
	maxTokenLength       = 4096
	maxExprLength        = 100
)

// ValidateText validates a required free form value. These values are sent in
// a JSON body, so this is a size and hygiene guard rather than an escaping
// guard.
func ValidateText(field, value string, maxLength int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return ValidateOptionalText(field, value, maxLength)
}

// ValidateOptionalText validates a free form value that may be absent.
// Newlines are allowed: template bodies and certificate style secret values
// are legitimately multi-line, and these values never reach a request line,
// a header or a log message.
func ValidateOptionalText(field, value string, maxLength int) error {
	if value == "" {
		return nil
	}
	if len(value) > maxLength {
		return fmt.Errorf("%s must not exceed %d characters", field, maxLength)
	}
	return nil
}
