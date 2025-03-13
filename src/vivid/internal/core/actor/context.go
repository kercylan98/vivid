package actor

import "github.com/kercylan98/go-log/log"

type Context interface {
	LoggerProvider() log.Provider
	MetadataContext() MetadataContext
	RelationContext() RelationContext
	GenerateContext() GenerateContext
	ProcessContext() ProcessContext
	MessageContext() MessageContext
}
