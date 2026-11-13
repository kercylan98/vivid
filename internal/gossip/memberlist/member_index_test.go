package memberlist_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func indexMember(t *testing.T, id string) *endpoint.Information {
	t.Helper()
	ref, err := actor.NewRef("127.0.0.1:10001", "/@gossip")
	require.NoError(t, err)
	info := endpoint.NewInformation(ref, time.Unix(1, 0))
	info.InstanceID, info.Status = id, endpoint.StatusUp
	return info
}

func TestMemberList_IndexOwnership(t *testing.T) {
	members := memberlist.New(nil)
	first, second := indexMember(t, "first"), indexMember(t, "second")
	require.True(t, members.Upsert(first))
	require.True(t, members.Upsert(second))
	first.InstanceID, first.Status = "changed-input", endpoint.StatusRemoved
	assert.Nil(t, members.Get(first.ID()), "写入后修改调用方身份不能改变索引")
	assert.Equal(t, endpoint.StatusUp, members.Get("first").Status)
	copy := members.Get("first")
	copy.InstanceID, copy.Status = "changed-result", endpoint.StatusRemoved
	assert.Nil(t, members.Get(copy.ID()))
	assert.Equal(t, endpoint.StatusUp, members.Get("first").Status)
	list := members.List()
	list[0], list[1] = list[1], list[0]
	list[0].InstanceID = "changed-list"
	assert.Equal(t, "first", members.At(0).InstanceID)
	assert.Equal(t, "second", members.At(1).InstanceID)
	visits := 0
	for _, member := range members.All() {
		member.InstanceID = "changed-iteration"
		visits++
		break
	}
	assert.Equal(t, 1, visits, "迭代器支持提前退出")
	assert.Equal(t, "first", members.At(0).InstanceID)
	copy = members.Get("first")
	copy.Status = endpoint.StatusLeaving
	assert.False(t, members.Upsert(copy))
	assert.Equal(t, endpoint.StatusLeaving, members.At(0).Status)
	assert.Equal(t, "second", members.At(1).InstanceID, "覆盖不能改变首次接收顺序")
	assert.False(t, members.Upsert(nil))
	assert.Equal(t, 2, members.Len())
}

func TestMemberList_IndexClone(t *testing.T) {
	members := memberlist.New(nil)
	members.Upsert(indexMember(t, "first"))
	members.Upsert(indexMember(t, "second"))
	clone := members.Clone()
	assert.NotSame(t, members.Get("first").ActorRef, clone.Get("first").ActorRef)
	changed := clone.Get("first")
	changed.Status = endpoint.StatusRemoved
	assert.False(t, clone.Upsert(changed), "克隆首次写入须建立自己的索引")
	assert.Equal(t, endpoint.StatusUp, members.Get("first").Status)
	assert.Equal(t, endpoint.StatusRemoved, clone.Get("first").Status)
	clone.Upsert(indexMember(t, "third"))
	assert.Nil(t, members.Get("third"))
	assert.Equal(t, "first", clone.At(0).InstanceID)
}

func TestMemberList_IndexDecode(t *testing.T) {
	first, second := indexMember(t, "first"), indexMember(t, "second")
	duplicate := first.Clone()
	duplicate.Status = endpoint.StatusRemoved
	// 保留收到的重复项，索引不能悄悄折叠它们或将首条改成末条。
	wire := serialization.NewWriter().Write([]*endpoint.Information{first, second, duplicate})
	require.NoError(t, wire.Err())
	members := memberlist.New(nil)
	members.Upsert(indexMember(t, "old"))
	require.NoError(t, members.Decode(serialization.NewReader(wire.Bytes()), members))
	assert.Nil(t, members.Get("old"), "重新解码不能保留旧索引")
	assert.Equal(t, 3, members.Len())
	assert.Equal(t, endpoint.StatusUp, members.Get("first").Status)
	encoded := serialization.NewWriter()
	require.NoError(t, members.Encode(encoded, members))
	assert.Equal(t, wire.Bytes(), encoded.Bytes(), "索引与记录存储不改变成员字段编码和顺序")
	changed := first.Clone()
	changed.Status = endpoint.StatusLeaving
	assert.False(t, members.Upsert(changed))
	assert.Equal(t, endpoint.StatusLeaving, members.Get("first").Status)
	assert.Equal(t, endpoint.StatusRemoved, members.At(2).Status, "重复列表覆盖沿用首项语义")
	require.Error(t, members.Decode(serialization.NewReader(wire.Bytes()[:len(wire.Bytes())-1]), members))
	assert.Equal(t, endpoint.StatusLeaving, members.Get("first").Status, "失败解码不能使记录和索引脱节")
	assert.Equal(t, 3, members.Len())
	for _, limits := range []serialization.DecodeLimits{{MaxElements: 2}, {MaxAllocationBytes: 1}} {
		err := members.Decode(serialization.NewReader(wire.Bytes(), limits), members)
		require.ErrorIs(t, err, serialization.ErrDecodeLimit)
		assert.Equal(t, endpoint.StatusLeaving, members.Get("first").Status, "预算不足不能修改已有记录和索引")
		assert.Equal(t, 3, members.Len())
	}
	empty := serialization.NewWriter().Write([]*endpoint.Information{})
	require.NoError(t, members.Decode(serialization.NewReader(empty.Bytes()), members))
	assert.Zero(t, members.Len())
	assert.Nil(t, members.Get("first"))
	assert.True(t, members.Upsert(first))
	assert.Equal(t, first, members.Get(first.ID()))
}
