// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raft

import (
	"errors"

	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

// ErrStepLocalMsg is returned when try to step a local raft message
//尝试步进本地 raft 消息时返回 ErrStepLocalMsg
var ErrStepLocalMsg = errors.New("raft: cannot step raft local message")

// ErrStepPeerNotFound is returned when try to step a response message
// but there is no peer found in raft.Prs for that node.
//ErrStepPeerNotFound 尝试步进响应消息但在 raft.Prs 中没有找到该节点的对等点时返回 ErrStepPeerNotFound
var ErrStepPeerNotFound = errors.New("raft: cannot step as peer not found")

// SoftState provides state that is volatile and does not need to be persisted to the WAL.
//SoftState 提供易变的状态，不需要持久化到 WAL
type SoftState struct {
	Lead      uint64
	RaftState StateType
}

// Ready encapsulates the entries and messages that are ready to read,
// be saved to stable storage, committed or sent to other peers.
// All fields in Ready are read-only.
//Ready 封装了准备读取、保存到稳定存储、提交或发送到其他对等点的条目和消息。 Ready 中的所有字段都是只读的
type Ready struct {
	// The current volatile state of a Node.
	// SoftState will be nil if there is no update.
	// It is not required to consume or store SoftState.
	// 节点的当前 volatile 状态。如果没有更新，SoftState 将为 nil。不需要消耗或存储 SoftState。
	*SoftState
	//	Lead      uint64
	//	RaftState StateType

	// The current state of a Node to be saved to stable storage BEFORE
	// Messages are sent.
	// HardState will be equal to empty state if there is no update.
	//在发送消息之前要保存到稳定存储的节点的当前状态。如果没有更新，HardState 将等于空状态。
	pb.HardState
	//	Term                 uint64   `protobuf:"varint,1,opt,name=term,proto3" json:"term,omitempty"`
	//	Vote                 uint64   `protobuf:"varint,2,opt,name=vote,proto3" json:"vote,omitempty"`
	//	Commit               uint64   `protobuf:"varint,3,opt,name=commit,proto3" json:"commit,omitempty"`
	//	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	//	XXX_unrecognized     []byte   `json:"-"`
	//	XXX_sizecache        int32    `json:"-"`

	// Entries specifies entries to be saved to stable storage BEFORE
	// Messages are sent.
	//条目指定要在发送消息之前保存到稳定存储的条目。
	Entries []pb.Entry

	// Snapshot specifies the snapshot to be saved to stable storage.
	//Snapshot 指定要保存到稳定存储的快照
	Snapshot pb.Snapshot

	// CommittedEntries specifies entries to be committed to a
	// store/state-machine. These have previously been committed to stable
	// store.
	//CommittedEntries 指定要提交到存储/状态机的条目。 这些以前一直致力于稳定存储
	CommittedEntries []pb.Entry

	// Messages specifies outbound messages to be sent AFTER Entries are
	// committed to stable storage.
	// If it contains a MessageType_MsgSnapshot message, the application MUST report back to raft
	// when the snapshot has been received or has failed by calling ReportSnapshot.
	//Messages 指定在条目提交到稳定存储后要发送的出站消息。 如果它包含 MessageType_MsgSnapshot 消息，则应用程序必须在收到快照或通过调用 ReportSnapshot 失败时向 raft 报告。
	Messages []pb.Message
}

// RawNode is a wrapper of Raft.
// RawNode 是 Raft 的包装器。
type RawNode struct {
	Raft *Raft
	// Your Data Here (2A).
	preSoft SoftState
	preHard pb.HardState
}

// NewRawNode returns a new RawNode given configuration and a list of raft peers.
//NewRawNode 返回一个新的RawNode给定配置和一个 raft peers 列表
func NewRawNode(config *Config) (*RawNode, error) {
	// Your Code Here (2A).
	r := newRaft(config)
	//if r == nil || r.RaftLog == nil {
	//	return nil,nil
	//}
	rn := &RawNode{
		Raft: r,
		preSoft: SoftState{
			Lead:      r.Lead,
			RaftState: r.State,
		},
		preHard: pb.HardState{
			Term:                 r.Term,
			Vote:                 r.Vote,
			Commit:               r.RaftLog.committed,
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil,
			XXX_sizecache:        0,
		},
	}
	return rn, nil
}

// Tick advances the internal logical clock by a single tick.
//Tick 将内部逻辑时钟提前一个滴答
func (rn *RawNode) Tick() {
	rn.Raft.tick()
}

// Campaign causes this RawNode to transition to candidate state.
//Campaign 导致此RawNode转换为候选状态
func (rn *RawNode) Campaign() error {
	return rn.Raft.Step(pb.Message{
		MsgType: pb.MessageType_MsgHup,
	})
}

// Propose proposes data be appended to the raft log.
//Propose 建议将数据附加到 raft 日志中。
func (rn *RawNode) Propose(data []byte) error {
	ent := pb.Entry{Data: data}
	return rn.Raft.Step(pb.Message{
		MsgType: pb.MessageType_MsgPropose,
		From:    rn.Raft.id,
		Entries: []*pb.Entry{&ent}})
}

// ProposeConfChange proposes a config change.
//ProposeConfChange 建议更改配置。
func (rn *RawNode) ProposeConfChange(cc pb.ConfChange) error {
	data, err := cc.Marshal()
	if err != nil {
		return err
	}
	ent := pb.Entry{EntryType: pb.EntryType_EntryConfChange, Data: data}
	return rn.Raft.Step(pb.Message{
		MsgType: pb.MessageType_MsgPropose,
		Entries: []*pb.Entry{&ent},
	})
}

// ApplyConfChange applies a config change to the local node.
//ApplyConfChange 将配置更改应用于本地节点。
func (rn *RawNode) ApplyConfChange(cc pb.ConfChange) *pb.ConfState {
	if cc.NodeId == None {
		return &pb.ConfState{Nodes: nodes(rn.Raft)}
	}
	switch cc.ChangeType {
	case pb.ConfChangeType_AddNode:
		rn.Raft.addNode(cc.NodeId)
	case pb.ConfChangeType_RemoveNode:
		rn.Raft.removeNode(cc.NodeId)
	default:
		panic("unexpected conf type")
	}
	return &pb.ConfState{Nodes: nodes(rn.Raft)}
}

// Step advances the state machine using the given message.
//Step 使用给定的消息推进状态机
func (rn *RawNode) Step(m pb.Message) error {
	// ignore unexpected local messages receiving over network
	if IsLocalMsg(m.MsgType) {
		return ErrStepLocalMsg
	}
	if pr := rn.Raft.Prs[m.From]; pr != nil || !IsResponseMsg(m.MsgType) {
		return rn.Raft.Step(m)
	}
	return ErrStepPeerNotFound
}

// Ready returns the current point-in-time state of this RawNode.
//Ready返回此RawNode的当前时间点状态
func (rn *RawNode) Ready() Ready {
	r := rn.Raft
	g := r.RaftLog
	soft := rn.getSoft()
	hard := pb.HardState{
		Term:                 0,
		Vote:                 0,
		Commit:               0,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
	hh := rn.getHard()
	if hh != nil {
		hard = *hh
	}
	ready := Ready{
		SoftState:        soft,
		HardState:        hard,
		Entries:          g.unstableEntries(),
		Snapshot:         pb.Snapshot{},
		CommittedEntries: g.nextEnts(),
		Messages:         r.msgs,
	}
	// Your Code Here (2A).
	return ready
}

// HasReady called when RawNode user need to check if any Ready pending.
//当 RawNode 用户需要检查是否有任何就绪挂起时调用 HasReady。
func (rn *RawNode) HasReady() bool {
	// Your Code Here (2A).

	return false
}
func (rn *RawNode) getSoft() *SoftState {
	r := rn.Raft
	nowSoft := &SoftState{
		Lead:      r.Lead,
		RaftState: r.State,
	}
	if rn.preSoft.Lead == nowSoft.Lead && rn.preSoft.RaftState == nowSoft.RaftState {
		return nil
	} else {
		//rn.preSoft = *nowSoft
		return nowSoft
	}

}
func (rn *RawNode) getHard() *pb.HardState {
	r := rn.Raft
	nowHard := &pb.HardState{
		Term:                 r.Term,
		Vote:                 r.Vote,
		Commit:               r.RaftLog.committed,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
	if rn.preHard.Term == nowHard.Term && rn.preHard.Vote == nowHard.Vote && rn.preHard.Commit == nowHard.Commit {
		return nil
	} else {
		//rn.preHard = *nowHard
		return nowHard
	}
}

// Advance notifies the RawNode that the application has applied and saved progress in the
// last Ready results.
//Advance 通知RawNode应用程序已应用并保存了上次Ready结果中的进度
func (rn *RawNode) Advance(rd Ready) {
	//	ready := Ready{
	//		SoftState:        soft,
	//		HardState:        *hard,
	//		Entries:          g.entries,
	//		Snapshot:         pb.Snapshot{},
	//		CommittedEntries: g.nextEnts(),
	//		Messages:         r.msgs,
	//	}
	r := rn.Raft
	g := r.RaftLog
	for _, v := range rd.Messages {
		rn.Step(v)
	}
	g.stabled = g.LastIndex()
	g.applied = g.committed

	nowHard := rn.getHard()
	nowSoft := rn.getSoft()
	if nowHard != nil {
		rn.preHard = *nowHard
	}
	if nowSoft != nil {
		rn.preSoft = *nowSoft
	}

	//Your Code Here (2A).
}

// GetProgress return the Progress of this node and its peers, if this
// node is leader.
//如果此节点是领导者，则 GetProgress 返回此节点及其对等节点的进度。
func (rn *RawNode) GetProgress() map[uint64]Progress {
	prs := make(map[uint64]Progress)
	if rn.Raft.State == StateLeader {
		for id, p := range rn.Raft.Prs {
			prs[id] = *p
		}
	}
	return prs
}

// TransferLeader tries to transfer leadership to the given transferee.
//TransferLeader 尝试将领导权转移给给定的受让人
func (rn *RawNode) TransferLeader(transferee uint64) {
	_ = rn.Raft.Step(pb.Message{MsgType: pb.MessageType_MsgTransferLeader, From: transferee})
}
