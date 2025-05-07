package persistence

type Repository interface {
	Save(persistenceId string, snapshot Snapshot, events []Event) error

	Load(persistenceId string) (Snapshot, []Event, error)
}
