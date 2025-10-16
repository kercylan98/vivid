package runtime

import (
	"fmt"
	"strings"
	"sync/atomic"
)

func NewProcessID(address, path string) ProcessID {
	return ProcessID{
		address: address,
		path:    path,
	}
}

type ProcessID struct {
	address string
	path    string
	cache   atomic.Pointer[Process]
}

func (p *ProcessID) Get() (Process, bool) {
	if cache := p.cache.Load(); cache != nil {
		return *cache, true
	}
	return nil, false
}

func (p *ProcessID) Cache(value Process) {
	p.cache.Store(&value)
}

func (p *ProcessID) Address() string {
	return p.address
}

func (p *ProcessID) Path() string {
	return p.path
}

func (p *ProcessID) String() string {
	return fmt.Sprintf("%s/%s", p.address, p.path)
}

func (p *ProcessID) Equal(other *ProcessID) bool {
	return p.address == other.address && p.path == other.path
}

func (p *ProcessID) Separate() ProcessID {
	return NewProcessID(p.address, p.path)
}

func (p *ProcessID) Branch(path string) ProcessID {
	var sb strings.Builder
	sb.WriteString(p.path)
	sb.WriteString("/")
	sb.WriteString(path)
	return NewProcessID(p.address, sb.String())
}
