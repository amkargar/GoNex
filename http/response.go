package http

import (
	"errors"
	apperrors "github.com/amkargar/gonex/errors"
	"github.com/gin-gonic/gin"
)

type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}
type ErrorResponse struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

func renderError(c *gin.Context, err error, allowed []int) {
	status := 500
	response := ErrorResponse{Code: "internal", Message: "internal server error"}
	var domain *apperrors.Error
	if errors.As(err, &domain) {
		switch domain.Code {
		case apperrors.NotFound:
			status = 404
		case apperrors.Conflict:
			status = 409
		case apperrors.Invalid:
			status = 400
		case apperrors.Unavailable:
			status = 503
		case apperrors.Unauthorized:
			status = 401
		case apperrors.Forbidden:
			status = 403
		}
		declared := status == 400 || status == 500
		for _, s := range allowed {
			if s == status {
				declared = true
			}
		}
		if declared && status != 500 {
			response = ErrorResponse{Code: string(domain.Code), Message: domain.Message}
		} else {
			status = 500
		}
	}
	c.JSON(status, response)
}
