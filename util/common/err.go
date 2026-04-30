package common

import (
	"fmt"
	"strings"
	"x-ui/logger"
)

// publicError 是一个实现了 entity.PublicError 接口的错误类型，
// 使得 safeErrorMessage 能将具体错误信息直接展示给用户。
type publicError struct {
	msg string
}

func (e *publicError) Error() string {
	return e.msg
}

func (e *publicError) PublicMessage() string {
	return e.msg
}

func NewErrorf(format string, a ...interface{}) error {
	msg := fmt.Sprintf(format, a...)
	return &publicError{msg: msg}
}

func NewError(a ...interface{}) error {
	msg := fmt.Sprintln(a...)
	msg = strings.TrimSuffix(msg, "\n")
	return &publicError{msg: msg}
}

func Recover(msg string) interface{} {
	panicErr := recover()
	if panicErr != nil {
		if msg != "" {
			logger.Error(msg, "panic:", panicErr)
		}
	}
	return panicErr
}