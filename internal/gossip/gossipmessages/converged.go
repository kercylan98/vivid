package gossipmessages

var (
	_Converged = &Converged{}
)

func NewConverged() *Converged {
	return _Converged
}

// Converged 表示集群收敛消息
type Converged struct{}
