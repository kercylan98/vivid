package vivid

var (
	_ ActorContext = (*actorContextImpl)(nil)
)

type ActorContext interface {
	actorContextBasic
	actorContextChildren
	actorContextMailboxMessageHandler
	actorContextProcess
}
