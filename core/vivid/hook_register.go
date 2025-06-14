package vivid

import (
    "fmt"
    "reflect"
)

func newHookRegister(hooks []Hook) *hookRegister {
    r := &hookRegister{}

    if len(hooks) > 0 {
        r.hooks = make(map[hookType][]*hookInfo)
    }
    for _, hook := range hooks {
        tof := reflect.TypeOf(hook)
        // 查找匹配的 Hook 类型
        suc := false
        for h := range hookTypes {
            // 一个 Hook 实现可能同时满足多个 Hook 类型
            if tof.AssignableTo(h) {
                r.hooks[h] = append(r.hooks[h], newHookInfo(tof, h, hook))
                suc = true
            }
        }
        if !suc {
            panic(fmt.Errorf("vivid: hook type %s not found", tof.String()))
        }
    }

    return r
}

type hookRegister struct {
    hooks map[hookType][]*hookInfo
}

func (r *hookRegister) hasHook(hookType hookType) bool {
    if r == nil {
        return false
    }
    _, ok := r.hooks[hookType]
    return ok
}

func (r *hookRegister) trigger(hookType hookType, args ...any) {
    if r == nil {
        return
    }

    hooks, exist := r.hooks[hookType]
    if !exist {
        return
    }
    for _, hook := range hooks {
        hook.trigger(args)
    }
}
