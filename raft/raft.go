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
	"github.com/pingcap/log"
)

// None is a placeholder node ID used when there is no leader.
const None uint64 = 0

// StateType represents the role of a node in a cluster.

// StateType 节点在集群中的角色
type StateType uint64

const (
	StateFollower  StateType = iota
	StateCandidate           //候选人
	StateLeader
)

var stmap = [...]string{
	"StateFollower",
	"StateCandidate",
	"StateLeader",
}

func (st StateType) String() string {
	return stmap[uint64(st)]
}

// ErrProposalDropped is returned when the proposal is ignored by some cases,
// so that the proposer can be notified and fail fast.

// ErrProposalDropped 当提议被某些情况忽略时返回，以便通知提议者并快速失败。
var ErrProposalDropped = errors.New("raft proposal dropped")

// Config contains the parameters to start a raft.+

// config包含启动raft 的参数
type Config struct {
	// ID is the identity of the local raft. ID cannot be 0.
	ID uint64

	// peers contains the IDs of all nodes (including self) in the raft cluster. It
	// should only be set when starting a new raft cluster. Restarting raft from
	// previous configuration will panic if peers is set. peer is private and only
	// used for testing right now.

	// peers 包含 raft 集群中所有节点（包括 self）的 ID。 它应该只在启动一个新的 raft 集群时设置。 如果设置了 peers，则从以前的配置重新启动 raft 会发生恐慌。 peer 是私有的，现在仅用于测试。
	peers []uint64

	// ElectionTick is the number of Node.Tick invocations that must pass between
	// elections. That is, if a follower does not receive any message from the
	// leader of current term before ElectionTick has elapsed, it will become
	// candidate and start an election. ElectionTick must be greater than
	// HeartbeatTick. We suggest ElectionTick = 10 * HeartbeatTick to avoid
	// unnecessary leader switching.

	// ElectionTick 是必须在两次选举之间传递的 Node.Tick 调用次数。 也就是说，如果一个follower在ElectionTick结束之前没有收到当前任期leader的任何消息，它将成为候选人并开始选举。 ElectionTick 必须大于HeartbeatTick 。 我们建议 ElectionTick = 10 * HeartbeatTick以避免不必要的领导者切换
	ElectionTick int

	// HeartbeatTick is the number of Node.Tick invocations that must pass between
	// heartbeats. That is, a leader sends heartbeat messages to maintain its
	// leadership every HeartbeatTick ticks.

	// HeartbeatTick 是必须在心跳之间传递的 Node.Tick 调用数。 也就是说，领导者发送心跳消息以在每个 HeartbeatTick 滴答声中保持其领导地位。
	HeartbeatTick int

	// Storage is the storage for raft. raft generates entries and states to be
	// stored in storage. raft reads the persisted entries and states out of
	// Storage when it needs. raft reads out the previous state and configuration
	// out of storage when restarting.

	//Storage 是 raft 的存储。 raft 生成要存储在存储中的`条目`和`状态`。 raft 读取持久化的条目并在需要时从 Storage 中取出状态。 raft 在重新启动时从存储中读出之前的状态和配置。
	Storage Storage
	// Applied is the last applied index. It should only be set when restarting
	// raft. raft will not return entries to the application smaller or equal to
	// Applied. If Applied is unset when restarting, raft might return previous
	// applied entries. This is a very application dependent configuration.

	//Applied 是最后应用的索引。 它应该只在重新启动 raft 时设置。 raft 不会将条目返回到小于或等于 Applied 的应用程序。 如果重新启动时未设置 Applied ，则 raft 可能会返回以前应用的条目。 这是一个非常依赖于应用程序的配置。
	Applied uint64
}

func (c *Config) validate() error {
	if c.ID == None {
		return errors.New("cannot use none as id")
	}

	if c.HeartbeatTick <= 0 {
		return errors.New("heartbeat tick must be greater than 0")
	}

	if c.ElectionTick <= c.HeartbeatTick {
		return errors.New("election tick must be greater than heartbeat tick")
	}

	if c.Storage == nil {
		return errors.New("storage cannot be nil")
	}

	return nil
}

// Progress represents a follower’s progress in the view of the leader. Leader maintains
// progresses of all followers, and sends entries to the follower based on its progress.

// Progress 代表了一个追随者在领导者眼中的进步。 Leader 维护所有 follower 的进度，并根据进度将 entry 发送给 follower。
type Progress struct {
	Match, Next uint64
}

type Raft struct {
	id uint64

	Term uint64
	Vote uint64

	// the log
	RaftLog *RaftLog

	// log replication progress of each peers

	//记录每个peer的复制进度
	Prs map[uint64]*Progress

	// this peer's role

	// 角色
	State StateType

	// votes records
	votes map[uint64]bool

	// msgs need to send
	msgs []pb.Message

	// the leader id
	Lead uint64

	// heartbeat interval, should send
	//心跳间隔，应该发送
	heartbeatTimeout int

	// baseline of election interval
	//选举间隔基线
	electionTimeout     int
	electionTimeoutRand int

	// number of ticks since it reached last heartbeatTimeout.
	// only leader keeps heartbeatElapsed.

	//自上次到达 heartbeatTimeout 以来的滴答数。只有leader保持heartbeatElapsed。
	heartbeatElapsed int

	// Ticks since it reached last electionTimeout when it is leader or candidate.
	// Number of ticks since it reached last electionTimeout or received a
	// valid message from current leader when it is a follower.
	//当它是领导者或候选人时，自上次达到选举超时以来的滴答声。 自从它达到上次选举超时或从当前领导者作为跟随者时收到有效消息以来的滴答数
	electionElapsed int

	// leadTransferee is id of the leader transfer target when its value is not zero.
	// Follow the procedure defined in section 3.10 of Raft phd thesis.
	// (https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf)
	// (Used in 3A leader transfer)

	//当leadTransferee的值不为零时，leadTransferee是leader转移目标的id 。 按照Raft博士论文第 3.10 节中定义的程序进行操作。 （ https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf  ）（用于3A领导转移）
	leadTransferee uint64

	// Only one conf change may be pending (in the log, but not yet
	// applied) at a time. This is enforced via PendingConfIndex, which
	// is set to a value >= the log index of the latest pending
	// configuration change (if any). Config changes are only allowed to
	// be proposed if the leader's applied index is greater than this
	// value.
	// (Used in 3A conf change)

	//一次可能只有一个 conf 更改未决（在日志中，但尚未应用）。 这是通过 PendingConfIndex 强制执行的，该值设置为 >= 最新挂起配置更改（如果有）的日志索引的值。 仅当领导者的应用索引大于此值时才允许提出配置更改。 （用于 3A conf 更改）
	PendingConfIndex uint64
}

// newRaft return a raft peer with the given config
// newRaft 返回具有给定配置的 raft peer
func newRaft(c *Config) *Raft {

	if err := c.validate(); err != nil {
		panic(err.Error())
	}
	//type Config struct {
	//    ID            uint64
	//    peers         []uint64
	//    ElectionTick  int
	//    HeartbeatTick int
	//    Storage       Storage
	//    Applied       uint64
	//}

	//
	//type RaftLog struct {
	//	storage         Storage
	//	committed       uint64
	//	applied         uint64
	//	stabled         uint64
	//	entries         []eraftpb.Entry
	//	pendingSnapshot *eraftpb.Snapshot
	//}

	//	r := newTestRaft(1, []uint64{1, 2, 3}, 10, 1, NewMemoryStorage())
	// 						id, peers, election, heartbeat, storage
	peers := make(map[uint64]*Progress)
	//fmt.Print(c.peers)
	for _, i := range c.peers {
		//fmt.Print(i)
		peers[uint64(i)] = new(Progress)
	}
	return &Raft{
		id:               c.ID,
		Term:             0,
		Vote:             0,
		RaftLog:          newLog(c.Storage),
		Prs:              peers,
		State:            StateFollower,
		votes:            make(map[uint64]bool),
		msgs:             make([]pb.Message, 0),
		Lead:             0,
		heartbeatTimeout: c.HeartbeatTick,
		electionTimeout:  c.ElectionTick,
		heartbeatElapsed: 0,
		leadTransferee:   0,
		PendingConfIndex: 0,
	}

	// Your Code Here (2A).
}

// sendAppend sends an append RPC with new entries (if any) and the
// current commit index to the given peer. Returns true if a message was sent

//sendAppend 将带有新条目（如果有）和当前提交索引的附加 RPC 发送到给定的对等方。 如果发送了消息，则返回 true。
func (r *Raft) sendAppend(to uint64) bool {
	// Your Code Here (2A).
	return false
}

// handleAppendEntries handle AppendEntries RPC request
//处理 AppendEntries RPC 请求
func (r *Raft) handleAppendEntries(m pb.Message) {

	// Your Code Here (2A).
}
func (r *Raft) handleAppendEntriesResponse(m pb.Message) {

}

// sendHeartbeat sends a heartbeat RPC to the given peer.
// sendHeartbeat 将心跳 RPC 发送到给定的对等方。
func (r *Raft) sendHeartbeat(to uint64) {
	//带有m.Index = 0，m.LogTerm = 0和空条目的MessageType_MsgHeartbeat作为心跳。
	// raft.msgs need to send
	//	msgs []pb.Message
	msg := &pb.Message{
		MsgType: pb.MessageType_MsgHeartbeat,
		To:      to,
		Index:   0,
		LogTerm: 0,
		From:    r.id,
		Term:    r.Term,
		//Entries[] * Entry,
		//Commit: 0,
		//Snapshot * Snapshot,
		//Reject               bool,
	}
	r.msgs = append(r.msgs, *msg)
	// Your Code Here (2A).
}

// handleHeartbeat handle Heartbeat RPC request
// 处理 心跳 RPC 请求
func (r *Raft) handleHeartbeat(m pb.Message) {
	for i := range r.Prs {
		//fmt.Print(i)
		if i != r.id {
			r.sendHeartbeat(i)
		}
		r.heartbeatElapsed = 0
	}
}
func (r *Raft) handleHeartbeatResponse(m pb.Message) {
	//if m.GetFrom() == r.Lead {
	r.electionElapsed = 0
	//}
	// Your Code Here (2A).
}

//请求投票   RPC 发送到给定的对等方。
func (r *Raft) sendVoteRequest(to uint64) {
	msg := &pb.Message{
		MsgType: pb.MessageType_MsgRequestVote,
		To:      to,
		//Index:   0,
		//LogTerm: 0,
		From: r.id,
		Term: r.Term,
		//Entries[] * Entry,
		//Commit: 0,
		//Snapshot * Snapshot,
		//Reject               bool,
	}
	r.msgs = append(r.msgs, *msg)
}
func (r *Raft) sendVoteResponse(to uint64, ok bool) {
	msg := &pb.Message{
		MsgType: pb.MessageType_MsgRequestVoteResponse,
		To:      to,
		//Index:   0,
		//LogTerm: 0,
		From: r.id,
		Term: r.Term,
		//Entries[] * Entry,
		//Commit: 0,
		//Snapshot * Snapshot,
		Reject: ok,
	}
	r.msgs = append(r.msgs, *msg)
}
func (r *Raft) handleVote(m pb.Message) {
	r.becomeCandidate()
	r.votes[r.id] = true
	for i := range r.Prs {
		if i != r.id {
			r.sendVoteRequest(i)
		}
	}
	half := len(r.Prs) / 2
	agree := 0
	for _, ok := range r.votes {
		if ok {
			agree++
			if agree > half {
				r.becomeLeader()
				return
			}
		}
	}

}
func (r *Raft) handleVoteRequest(m pb.Message) {
	if m.GetTerm() < r.Term {
		r.sendVoteResponse(m.GetFrom(), true)
	} else {

		if m.GetFrom() == r.Vote || r.Vote == 0 {
			r.Vote = m.GetFrom()
			r.sendVoteResponse(m.GetFrom(), false)
		} else {
			r.sendVoteResponse(m.GetFrom(), true)
		}
	}
}
func (r *Raft) handleVoteResponse(m pb.Message) {
	r.votes[m.GetFrom()] = !m.Reject
	//	Prs map[uint64]*Progress
	half := len(r.Prs) / 2
	agree := 0
	for _, ok := range r.votes {
		if ok {
			agree++
			if agree > half {
				r.becomeLeader()
				return
			}
		}
	}
	//r.becomeFollower(r.Term-1, r.Lead)
	//r.becomeCandidate()
}

// tick advances the internal logical clock by a single tick.
//滴答将内部逻辑时钟提前一个滴答
func (r *Raft) tick() {
	//测试如果leader收到心跳tick，它将向所有follower发送一个带有m.Index = 0，m.LogTerm = 0和空条目的MessageType_MsgHeartbeat作为心跳。
	var err error
	if r.State == StateLeader {
		r.heartbeatElapsed++
		if r.heartbeatElapsed >= r.heartbeatTimeout {
			//log.S().Info("client: ")
			err = r.Step(pb.Message{
				MsgType:              pb.MessageType_MsgBeat,
				To:                   r.id,
				From:                 r.id,
				Term:                 r.Term,
				LogTerm:              0,
				Index:                0,
				Entries:              nil,
				Commit:               0,
				Snapshot:             nil,
				Reject:               false,
				XXX_NoUnkeyedLiteral: struct{}{},
				XXX_unrecognized:     nil,
				XXX_sizecache:        0,
			})
			if err != nil {
				log.S().Error(err.Error())
			}
			//fmt.Println()
		}
	}
	if r.State != StateLeader {
		r.electionElapsed++
		if r.electionElapsed >= r.electionTimeout {
			r.electionElapsed = 0
			err = r.Step(pb.Message{
				MsgType:              pb.MessageType_MsgHup,
				To:                   r.id,
				From:                 r.id,
				Term:                 r.Term,
				LogTerm:              0,
				Index:                0,
				Entries:              nil,
				Commit:               0,
				Snapshot:             nil,
				Reject:               false,
				XXX_NoUnkeyedLiteral: struct{}{},
				XXX_unrecognized:     nil,
				XXX_sizecache:        0,
			})
			if err != nil {
				log.S().Error(err.Error())
			}
		}
	}
	// Your Code Here (2A).
}

// becomeFollower transform this peer's state to Follower
//becomeFollower 将这个 peer 的状态转换为 Follower
func (r *Raft) becomeFollower(term uint64, lead uint64) {
	//s, _ := huge.ToIndentJSON(r)
	//fmt.Printf("%v", s)
	r.Term = term
	r.Lead = lead
	r.Vote = 0
	r.State = StateFollower
	r.votes = make(map[uint64]bool)

	// Your Code Here (2A).
}

// becomeCandidate transform this peer's state to candidate
func (r *Raft) becomeCandidate() {
	r.Term++
	r.State = StateCandidate
	r.votes = make(map[uint64]bool)
	r.Vote = r.id
	// Your Code Here (2A).
}

// becomeLeader transform this peer's state to leader
func (r *Raft) becomeLeader() {
	r.State = StateLeader
	r.Lead = r.id
	r.electionElapsed = 0
	r.Vote = 0
	r.votes = make(map[uint64]bool)
	// Your Code Here (2A).
	// NOTE: Leader should propose a noop entry on its term
	//领导者应该在其任期内提出一个 noop 条目
}

// Step the entrance of handle message, see `MessageType`
// on `eraftpb.proto` for what msgs should be handled
//步骤处理消息的入口，处理什么msgs见`erraftpb.proto`上的`MessageType`
func (r *Raft) Step(m pb.Message) error {
	// Your Code Here (2A).

	//type Message struct {
	//	MsgType              MessageType `protobuf:"varint,1,opt,name=msg_type,json=msgType,proto3,enum=eraftpb.MessageType" json:"msg_type,omitempty"`
	//	To                   uint64      `protobuf:"varint,2,opt,name=to,proto3" json:"to,omitempty"`
	//	From                 uint64      `protobuf:"varint,3,opt,name=from,proto3" json:"from,omitempty"`
	//	Term                 uint64      `protobuf:"varint,4,opt,name=term,proto3" json:"term,omitempty"`
	//	LogTerm              uint64      `protobuf:"varint,5,opt,name=log_term,json=logTerm,proto3" json:"log_term,omitempty"`
	//	Index                uint64      `protobuf:"varint,6,opt,name=index,proto3" json:"index,omitempty"`
	//	Entries              []*Entry    `protobuf:"bytes,7,rep,name=entries" json:"entries,omitempty"`
	//	Commit               uint64      `protobuf:"varint,8,opt,name=commit,proto3" json:"commit,omitempty"`
	//	Snapshot             *Snapshot   `protobuf:"bytes,9,opt,name=snapshot" json:"snapshot,omitempty"`
	//	Reject               bool        `protobuf:"varint,10,opt,name=reject,proto3" json:"reject,omitempty"`
	//	XXX_NoUnkeyedLiteral struct{}    `json:"-"`
	//	XXX_unrecognized     []byte      `json:"-"`
	//	XXX_sizecache        int32       `json:"-"`
	//}
	//ty := m.GetMsgType()

	switch m.GetMsgType() {

	case pb.MessageType_MsgBeat:
		r.handleHeartbeat(m)
	//case pb.MessageType_MsgHeartbeat:
	//r.handleHeartbeat(m)
	case pb.MessageType_MsgHeartbeatResponse:
		r.handleHeartbeatResponse(m)

	case pb.MessageType_MsgHup:
		r.handleVote(m)
	case pb.MessageType_MsgRequestVote:
		r.handleVoteRequest(m)
	case pb.MessageType_MsgRequestVoteResponse:
		r.handleVoteResponse(m)

	case pb.MessageType_MsgAppend:
		r.handleAppendEntries(m)
	case pb.MessageType_MsgAppendResponse:
		r.handleAppendEntriesResponse(m)
	case pb.MessageType_MsgSnapshot:
		r.handleSnapshot(m)
		//case pb.MessageType_MsgPropose:
		//r.RaftLog.Propose()
		//en := m.Entries
		//raw := &RawNode{Raft: r}
		//raw.Propose(en)

	}
	switch r.State {
	case StateFollower:
		r.Term = m.GetTerm()
	case StateCandidate:
		if m.GetTerm() > r.Term {
			r.becomeFollower(m.GetTerm(), m.GetFrom())
		}
	case StateLeader:
		if m.GetTerm() > r.Term {
			r.becomeFollower(m.GetTerm(), m.GetFrom())
		}
	}

	return nil
}

// handleSnapshot handle Snapshot RPC request
//处理快照 RPC 请求
func (r *Raft) handleSnapshot(m pb.Message) {
	// Your Code Here (2C).
}

// addNode add a new node to raft group
//addNode 添加一个新节点到 raft 组
func (r *Raft) addNode(id uint64) {
	// Your Code Here (3A).
}

// removeNode remove a node from raft group
//removeNode 从 raft 组中删除一个节点
func (r *Raft) removeNode(id uint64) {
	// Your Code Here (3A).
}
