package message

import (
	"github.com/pingcap-incubator/tinykv/kv/raftstore/snap"
	"github.com/pingcap-incubator/tinykv/proto/pkg/metapb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/raft_cmdpb"
)

type MsgType int64

const (
	// just a placeholder
	//只是一个占位符
	MsgTypeNull MsgType = 0
	// message to start the ticker of peer
	//消息以启动对等节点的自动收报机
	MsgTypeStart MsgType = 1
	// message of base tick to drive the ticker
	用于驱动自动收报机的基本滴答消息
	MsgTypeTick MsgType = 2
	// message wraps a raft message that should be forwarded to Raft module
	// the raft message is from peer on other store
	//message 包装了一个应该转发到 Raft 模块的 raft 消息，该 raft 消息来自其他商店的对等点
	MsgTypeRaftMessage MsgType = 3
	// message warps a raft command that maybe a read/write request or admin request
	// the raft command should be proposed to Raft module
	//消息扭曲了一个可能是读写请求或管理员请求的 raft 命令，应该向 Raft 模块提出 raft 命令
	MsgTypeRaftCmd MsgType = 4
	// message to trigger split region
	//触发分割区域的消息
	// it first asks Scheduler for allocating new split region's ids, then schedules a
	// MsyTypeRaftCmd with split admin command
	//它首先要求调度程序分配新的分割区域的 id，然后调度一个带有拆分管理命令的 MsyTypeRaftCmd
	MsgTypeSplitRegion MsgType = 5
	// message to update region approximate size
	// it is sent by split checker
	//更新区域近似大小的消息
	//它由拆分检查器发送
	MsgTypeRegionApproximateSize MsgType = 6
	// message to trigger gc generated snapshots
	//触发 gc 生成快照的消息
	MsgTypeGcSnap MsgType = 7

	// message wraps a raft message to the peer not existing on the Store.
	// It is due to region split or add peer conf change
	//	message 将 raft 消息包装到 Store 上不存在的对等方。
	//  是因为region split或者添加peer conf改变
	MsgTypeStoreRaftMessage MsgType = 101
	// message of store base tick to drive the store ticker, including store heartbeat
	// store base tick 的消息，用于驱动 store ticker，包括 store heartbeat
	MsgTypeStoreTick MsgType = 106
	// message to start the ticker of store
	MsgTypeStoreStart MsgType = 107
)

type Msg struct {
	Type     MsgType
	RegionID uint64
	Data     interface{}
}

func NewMsg(tp MsgType, data interface{}) Msg {
	return Msg{Type: tp, Data: data}
}

func NewPeerMsg(tp MsgType, regionID uint64, data interface{}) Msg {
	return Msg{Type: tp, RegionID: regionID, Data: data}
}

type MsgGCSnap struct {
	Snaps []snap.SnapKeyWithSending
}

type MsgRaftCmd struct {
	Request  *raft_cmdpb.RaftCmdRequest
	Callback *Callback
}

type MsgSplitRegion struct {
	RegionEpoch *metapb.RegionEpoch
	SplitKey    []byte
	Callback    *Callback
}
