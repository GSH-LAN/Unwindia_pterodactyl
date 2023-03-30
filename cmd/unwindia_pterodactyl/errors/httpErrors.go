package errors

import "github.com/meysamhadeli/problem-details"

type BadParamError struct {
	problem.ProblemDetailErr
	Description    string `json:"description,omitempty"`
	AdditionalInfo string `json:"additionalInfo,omitempty"`
}
