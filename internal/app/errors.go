package app

import (
	"fmt"
	"strings"
)

// ApplicationConfigErrorTag identifies which configuration field an error relates to.
type ApplicationConfigErrorTag int

const (
	ACEOrgPlayerIDPepper  ApplicationConfigErrorTag = iota
	ACEOrgPlayerIDHash
	ACEEndpoint
	ACEEnvironment
	ACEUsePlayersDB
	ACEServiceCredentials
)

// ApplicationConfigError represents a single config validation error for a specific field.
type ApplicationConfigError struct {
	Tag ApplicationConfigErrorTag
	Err error
}

// Error returns a formatted error string: "field-name: message".
func (e *ApplicationConfigError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorPrefix(), e.Err.Error())
}

// ErrorPrefix returns the human-readable field name for this error's tag.
func (e *ApplicationConfigError) ErrorPrefix() string {
	switch e.Tag {
	case ACEOrgPlayerIDPepper:
		return "org-playerID-pepper"
	case ACEOrgPlayerIDHash:
		return "org-playerID-hash"
	case ACEEndpoint:
		return "endpoint"
	case ACEEnvironment:
		return "environment"
	case ACEUsePlayersDB:
		return "use-players-db"
	case ACEServiceCredentials:
		return "service-credentials"
	default:
		return "unknown"
	}
}

// Unwrap returns the underlying error.
func (e *ApplicationConfigError) Unwrap() error {
	return e.Err
}

// ApplicationConfigErrors collects multiple ApplicationConfigError values.
type ApplicationConfigErrors struct {
	Errors []*ApplicationConfigError
}

// HasErrors returns true if there are any errors.
func (e *ApplicationConfigErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// Add appends an error. If err is an *ApplicationConfigError it is added directly;
// otherwise it is wrapped with a generic tag.
func (e *ApplicationConfigErrors) Add(err error) {
	if ace, ok := err.(*ApplicationConfigError); ok {
		e.Errors = append(e.Errors, ace)
	} else {
		e.Errors = append(e.Errors, &ApplicationConfigError{Err: err})
	}
}

// Error returns a newline-separated list of all error messages.
func (e *ApplicationConfigErrors) Error() string {
	var parts []string
	for _, err := range e.Errors {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "\n")
}

// Filter methods — return errors matching the given tag.

func (e *ApplicationConfigErrors) filterByTag(tag ApplicationConfigErrorTag) []*ApplicationConfigError {
	var result []*ApplicationConfigError
	for _, err := range e.Errors {
		if err.Tag == tag {
			result = append(result, err)
		}
	}
	return result
}

func (e *ApplicationConfigErrors) GetPlayerIDPepperError() []*ApplicationConfigError {
	return e.filterByTag(ACEOrgPlayerIDPepper)
}

func (e *ApplicationConfigErrors) GetPlayerIDHashError() []*ApplicationConfigError {
	return e.filterByTag(ACEOrgPlayerIDHash)
}

func (e *ApplicationConfigErrors) GetEndpointError() []*ApplicationConfigError {
	return e.filterByTag(ACEEndpoint)
}

func (e *ApplicationConfigErrors) GetEnvironmentError() []*ApplicationConfigError {
	return e.filterByTag(ACEEnvironment)
}

func (e *ApplicationConfigErrors) GetUsePlayersDBError() []*ApplicationConfigError {
	return e.filterByTag(ACEUsePlayersDB)
}

func (e *ApplicationConfigErrors) GetServiceCredentialsError() []*ApplicationConfigError {
	return e.filterByTag(ACEServiceCredentials)
}
