package server

import "github.com/gin-gonic/gin"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func abortAPIError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, apiError{Code: code, Message: message})
}
