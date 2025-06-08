package vivid

import (
	"fmt"
	"reflect"
)

func newHookInfo(rawType, hookType hookType, hook Hook) *hookInfo {
	if !isRegisteredHookType(hookType) {
		panic(fmt.Errorf("hook type %s is not registered", rawType.String()))
	}

	info := &hookInfo{
		hook:     hook,
		hookType: rawType,
		hookVof:  reflect.ValueOf(hook),
	}

	for i := range rawType.NumMethod() {
		method := rawType.Method(i)
		if method.Name == getHookTypeMethodName(hookType) {
			info.funcVof = method.Func
			argNum := method.Type.NumIn() - 1
			info.argTypes = make([]reflect.Type, argNum)
			for i := range method.Type.NumIn() {
				if i == 0 {
					continue
				}
				info.argTypes[i-1] = method.Type.In(i - 1)
			}
			break
		}
	}

	return info
}

type hookInfo struct {
	hook     Hook
	hookType hookType
	hookVof  reflect.Value
	funcVof  reflect.Value
	argTypes []reflect.Type
}

func (h *hookInfo) trigger(args []any) {
	var argVof = make([]reflect.Value, len(args)+1)
	argVof[0] = h.hookVof
	for i, arg := range args {
		argVof[i+1] = reflect.ValueOf(arg)
	}

	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("hook %s panicked: %v", h.hookType, r))
		}
	}()

	h.funcVof.Call(argVof)
}
