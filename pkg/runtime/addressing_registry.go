package runtime

// AddressingRegistry 地址注册表是所有可服务单元的地址注册表，它记录了所有可服务单元的地址信息。
type AddressingRegistry interface {
	Register(id *ProcessID, process Process) (Process, error)

	Unregister(id *ProcessID) error

	Find(id *ProcessID) (Process, error)
}
