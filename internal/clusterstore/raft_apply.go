package clusterstore

import "errors"

// replayRaftStore 先恢复压缩快照，再顺序消费已提交日志；未提交后缀不会进入状态机。
// 比较失败和容量不足是已经执行的业务结果，格式错误则保留已应用前缀并停止。
func replayRaftStore(node *RaftNode, store *Store, applied func(RaftEntry, Result)) (*Store, error) {
	for {
		entries, err := node.Committed(store.Index())
		if errors.Is(err, ErrRaftCompacted) {
			snapshot, err := node.Snapshot()
			if err != nil {
				return store, err
			}
			restored, err := Restore(snapshot.Data)
			if err != nil {
				return store, err
			}
			if restored.Index() != snapshot.Index {
				return store, ErrInvalidSnapshot
			}
			store = restored
			continue
		}
		if err != nil || len(entries) == 0 {
			return store, err
		}
		for _, entry := range entries {
			command, err := DecodeCommand(entry.Data)
			if err != nil {
				return store, err
			}
			result, err := store.Apply(entry.Index, command)
			if err != nil {
				return store, err
			}
			if applied != nil {
				applied(entry, result)
			}
		}
	}
}
