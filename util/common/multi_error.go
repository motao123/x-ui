package common

import (
	"strings"
)

type multiError []error

func (e multiError) Error() string {
	var r strings.Builder
	r.WriteString("multierr: ")
	for _, err := range e {
		r.WriteString(err.Error())
		r.WriteString(" | ")
	}
	return r.String()
}

func (e multiError) PublicMessage() string {
	var r strings.Builder
	for i, err := range e {
		if publicErr, ok := err.(interface{ PublicMessage() string }); ok {
			r.WriteString(publicErr.PublicMessage())
		} else {
			r.WriteString(err.Error())
		}
		if i < len(e)-1 {
			r.WriteString("; ")
		}
	}
	return r.String()
}

func Combine(maybeError ...error) error {
	var errs multiError
	for _, err := range maybeError {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
