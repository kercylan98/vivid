package cluster

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/utils"
)

const defaultClusterAskTimeout = 5 * time.Second

var _ vivid.ClusterContext = (*Context)(nil)

func NewClusterContext(system vivid.ActorSystem, options vivid.ClusterOptions, actorRefParser ActorRefParser, nodeRef vivid.ActorRef) *Context {
	return &Context{
		options:        options,
		system:         system,
		nodeRef:        nodeRef,
		actorRefParser: actorRefParser,
	}
}

type Context struct {
	options        vivid.ClusterOptions
	system         vivid.ActorSystem
	nodeRef        vivid.ActorRef
	actorRefParser ActorRefParser
}

func (c *Context) Name() string {
	return c.options.ClusterName
}

func (c *Context) GetMembers() ([]vivid.ClusterMemberInfo, error) {
	if c == nil {
		return nil, vivid.ErrorClusterDisabled
	}
	request := &publicMessageAsGetNodesQuery{
		ClusterName: c.Name(),
	}
	result, err := c.system.Ask(c.nodeRef, request, defaultClusterAskTimeout).Result()
	if err != nil {
		return nil, err
	}
	if errResult, ok := result.(error); ok && errResult != nil {
		return nil, errResult
	}
	members := result.(*publicMessageAsGetNodesResult).Members
	return members, nil
}

func (c *Context) getClusterState() (*publicMessageAsGetClusterStateResult, error) {
	if c == nil {
		return nil, vivid.ErrorClusterDisabled
	}
	result, err := c.system.Ask(c.nodeRef, &publicMessageAsGetClusterState{}, defaultClusterAskTimeout).Result()
	if err != nil {
		return nil, err
	}
	if errResult, ok := result.(error); ok && errResult != nil {
		return nil, errResult
	}
	return result.(*publicMessageAsGetClusterStateResult), nil
}

func (c *Context) Leader() (vivid.ActorRef, error) {
	if c == nil {
		return nil, vivid.ErrorClusterDisabled
	}
	state, err := c.getClusterState()
	if err != nil {
		return nil, err
	}
	if state.LeaderAddress == "" {
		return nil, nil
	}
	ref, err := c.actorRefParser(state.LeaderAddress, c.nodeRef.GetPath())
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func (c *Context) IsLeader() (bool, error) {
	if c == nil {
		return false, vivid.ErrorClusterDisabled
	}
	state, err := c.getClusterState()
	if err != nil {
		return false, err
	}
	if state.LeaderAddress == "" {
		return false, nil
	}
	selfAddr := c.nodeRef.GetAddress()
	if n, ok := utils.NormalizeAddress(selfAddr); ok {
		selfAddr = n
	}
	return state.LeaderAddress == selfAddr, nil
}

func (c *Context) InQuorum() (bool, error) {
	if c == nil {
		return false, vivid.ErrorClusterDisabled
	}
	state, err := c.getClusterState()
	if err != nil {
		return false, err
	}
	return state.InQuorum, nil
}

func (c *Context) SetNodeVersion(version string) {
	if c == nil {
		return
	}
	c.system.Tell(c.nodeRef, &publicMessageAsSetNodeVersion{version: version})
}

func (c *Context) UpdateMembers(addresses []string) {
	if c == nil || len(addresses) == 0 {
		return
	}
	c.system.Tell(c.nodeRef, &publicMessageAsMembersUpdated{nodes: addresses})
}

func (c *Context) Leave() {
	if c == nil {
		return
	}
	c.system.Tell(c.nodeRef, &publicMessageAsInitiateLeave{})
}
