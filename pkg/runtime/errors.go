package runtime

import "errors"

type Error error

var (
	ErrProcessNotFound      = Error(errors.New("process not found"))      // 进程未找到
	ErrProcessAlreadyExists = Error(errors.New("process already exists")) // 进程已存在
	ErrBadProcess           = Error(errors.New("bad process"))            // 进程无效
)
