package sugar

// WaitAllChannel 永久阻塞地等待所有通道完成，直到所有通道都完成。
func WaitAllChannel[T any](channels ...chan T) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		for _, channel := range channels {
			<-channel
		}
		close(done)
	}()
	return done
}
