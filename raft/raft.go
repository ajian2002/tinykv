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
	"bytes"
	"errors"
	"math/rand"
	"sort"

	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
	"github.com/pingcap/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	var coreArr []zapcore.Core

	//获取编码器
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	//encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	//encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder //按级别显示不同颜色，不需要的话取值zapcore.CapitalLevelEncoder就可以
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	//encoder := zapcore.NewJSONEncoder(encoderConfig)

	//日志级别
	highPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev >= zap.InfoLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev >= zap.DebugLevel
	})

	//debug文件writeSyncer
	DebugFileWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "./log/debug.log", //日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    1,                 //文件大小限制,单位MB
		MaxBackups: 5,                 //最大保留日志文件数量
		MaxAge:     30,                //日志文件保留天数
		Compress:   false,             //是否压缩处理
	})
	DebugFileCore := zapcore.NewCore(encoder, DebugFileWriteSyncer, lowPriority)
	//DebugFileCore := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(DebugFileWriteSyncer, zapcore.AddSync(os.Stderr)), lowPriority)
	//info文件writeSyncer
	InfoFileWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "./log/info.log", //日志文件存放目录
		MaxSize:    1,                //文件大小限制,单位MB
		MaxBackups: 5,                //最大保留日志文件数量
		MaxAge:     30,               //日志文件保留天数
		Compress:   false,            //是否压缩处理
	})
	//InfoFileCore := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(InfoFileWriteSyncer, zapcore.AddSync(os.Stderr)), highPriority)
	InfoFileCore := zapcore.NewCore(encoder, InfoFileWriteSyncer, highPriority)
	coreArr = append(coreArr, DebugFileCore)
	coreArr = append(coreArr, InfoFileCore)
	lg := zap.New(zapcore.NewTee(coreArr...), zap.AddCaller(), zap.AddStacktrace(zap.FatalLevel)) //zap.AddCaller()为显示文件名和行号，可省略
	zap.ReplaceGlobals(lg)
}

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
	peers := make(map[uint64]*Progress)
	for _, i := range c.peers {
		peers[uint64(i)] = new(Progress)
	}
	r := &Raft{
		id:                  c.ID,
		Term:                0,
		Vote:                0,
		RaftLog:             newLog(c.Storage),
		Prs:                 peers,
		State:               StateFollower,
		votes:               make(map[uint64]bool),
		msgs:                make([]pb.Message, 0),
		Lead:                0,
		heartbeatTimeout:    c.HeartbeatTick,
		electionTimeout:     c.ElectionTick,
		electionTimeoutRand: rand.Intn(c.ElectionTick) + c.ElectionTick,
		heartbeatElapsed:    0,
		electionElapsed:     0,
		leadTransferee:      0,
		PendingConfIndex:    0,
	}
	state, _, err := c.Storage.InitialState()
	if err == nil {
		r.Term = state.GetTerm()
		r.Vote = state.GetVote()
	}
	return r
	// Your Code Here (2A).
}

// sendAppend sends an append RPC with new entries (if any) and the
// current commit index to the given peer. Returns true if a message was sent
// sendAppend 将带有新条目（如果有）和当前提交索引的附加 RPC 发送到给定的对等方。 如果发送了消息，则返回 true。
func (r *Raft) sendAppend(to uint64) bool {
	g := r.RaftLog

	en := make([]*pb.Entry, 0)
	enx := g.getIndextoEntry(r.Prs[to].Match+1, r.Prs[to].Next)
	for i := range enx {
		en = append(en, &(enx[i]))
	}
	msg := &pb.Message{
		MsgType:  pb.MessageType_MsgAppend,
		To:       to,
		From:     r.id,
		Term:     r.Term,
		LogTerm:  r.Term,
		Index:    r.Prs[to].Match,
		Entries:  en, //[]*Entry
		Commit:   g.committed,
		Snapshot: g.pendingSnapshot, //* Snapshot
		//Reject:   false,
	}
	r.msgs = append(r.msgs, *msg)
	return true
}
func (r *Raft) handlePropose(m pb.Message) {
	if r.State == StateLeader {
		//set entry
		//if m.Entries == nil || len(m.Entries) == 0 {
		//	return
		//}
		for _, s := range m.Entries {
			ss := *s
			ss.Term = r.Term
			ss.Index = r.RaftLog.LastIndex() + 1 //2
			r.RaftLog.entries = append(r.RaftLog.entries, ss)
			//last ++,commit 不变
		}
		// 什么时候更新next
		// 向所有人转发
		for i := range r.Prs {
			r.Prs[i].Next = r.RaftLog.LastIndex() + 1
			if i != r.id {
				r.sendAppend(i)
			} else {
				r.Prs[i].Match = r.RaftLog.LastIndex()
			}
		}
		//TODO:单一leader情况
		if len(r.Prs) == 1 {
			r.RaftLog.committed = r.RaftLog.LastIndex()
			r.RaftLog.stabled = r.RaftLog.committed
			return
		}
	}
}
func (r *Raft) handleAppendEntries(m pb.Message) {

	//返回假 如果领导人的任期小于接收者的当前任期（译者注：这里的接收者是指跟随者或者候选人）（5.1 节）
	//返回假 如果接收者日志中没有包含这样一个条目 即该条目的任期在 prevLogIndex 上能和 prevLogTerm 匹配上 （译者注：在接收者日志中 如果能找到一个和 prevLogIndex 以及 prevLogTerm 一样的索引和任期的日志条目 则继续执行下面的步骤 否则返回假）（5.3 节）
	//如果一个已经存在的条目和新条目（译者注：即刚刚接收到的日志条目）发生了冲突（因为索引相同，任期不同），那么就删除这个已经存在的条目以及它之后的所有条目 （5.3 节）
	//追加日志中尚未存在的任何新条目
	//如果领导人的已知已提交的最高日志条目的索引大于接收者的已知已提交最高日志条目的索引（leaderCommit > commitIndex），则把接收者的已知已经提交的最高的日志条目的索引commitIndex 重置为 领导人的已知已经提交的最高的日志条目的索引 leaderCommit 或者是 上一个新条目的索引 取两者的最小值

	if r.State != StateFollower {
		term := m.Term
		if term >= r.Term {
			r.becomeFollower(term, m.GetFrom())
		} else {
			return
		}
	}
	{
		r.Lead = m.GetFrom()
		//success	如果跟随者所含有的条目和 prevLogIndex 以及 prevLogTerm 匹配上了，则为 true
		Rej := true
		g := r.RaftLog
		last := g.LastIndex()
		//if m.GetFrom() == r.Lead && m.GetTerm() >= r.Term {
		if m.GetTerm() >= r.Term {
			if m.Index > last || m.Index < 0 {
				goto over
			}
			begin := -1
			for i := 0; i < len(g.entries); i++ {
				if g.entries[i].Index == m.Index {
					begin = i
					break
				}
			}
			//ent := g.getIndextoEntry(m.Index, m.Index)
			if begin != -1 { //找到
				e := g.entries[begin]
				if e.Term == m.LogTerm {
					//匹配
					Rej = false
					r.updateEntries(m.GetEntries())
				} else {
					//reject
				}
			} else {
				//index=0
				Rej = false
				r.updateEntries(m.GetEntries())

			}
		}
		//如果领导人的已知已提交的最高日志条目的索引大于接收者的已知已提交最高日志条目的索引（leaderCommit > commitIndex），则把接收者的已知已经提交的最高的日志条目的索引commitIndex 重置为 领导人的已知已经提交的最高的日志条目的索引 leaderCommit 或者是 上一个新条目的索引 取两者的最小值

	over:
		if Rej == false && m.GetCommit() > g.committed {
			r.RaftLog.committed = min(m.GetCommit(), m.Index+uint64(len(m.Entries)))
			r.RaftLog.stabled = r.RaftLog.committed
		}
		msg := &pb.Message{
			MsgType: pb.MessageType_MsgAppendResponse,
			To:      m.GetFrom(),
			Index:   r.RaftLog.LastIndex(), //回复目前的最新
			//LogTerm: 0,
			From: r.id,
			Term: r.Term,
			//Entries[] * Entry,
			Commit: r.RaftLog.committed,
			//Snapshot * Snapshot,
			Reject: Rej,
		}
		r.msgs = append(r.msgs, *msg)
	}
	//}

	// Your Code Here (2A).
}
func (r *Raft) handleAppendEntriesResponse(m pb.Message) {
	if r.State == StateLeader {
		//agree := 0
		//half := len(r.Prs) / 2
		min := uint64(9223372036854775807)
		//old := r.Prs[m.GetFrom()].Match
		r.Prs[m.GetFrom()].Match = m.Index
		cliet := make(uint64Slice, len(r.Prs))
		for i, v := range r.Prs {
			cliet[i-1] = v.Match
			//if v.Match == v.Next {
			//	agree++
			//	//oo = append(oo, v.Match)
			//	if v.Match < min {
			//		min = v.Match
			//	}
			//}
		}
		sort.Sort(cliet)
		min = cliet[(len(r.Prs)-1)/2]
		//if agree > half {
		//if min != uint64(9223372036854775807){
		//
		//}
		//sort.Sort(oo)
		if r.RaftLog.committed != min {
			//to all
			indexterm, err := r.RaftLog.Term(min)
			if err == nil && indexterm == r.Term {

				r.RaftLog.committed = min
				//msg := &pb.Message{
				//	MsgType: pb.MessageType_MsgAppend,
				//	To:      m.GetFrom(),
				//	//Index:   r.RaftLog.LastIndex(), //回复目前的最新
				//	//LogTerm: 0,
				//	From: r.id,
				//	Term: r.Term,
				//	//Entries[] * Entry,
				//	Commit: r.RaftLog.committed,
				//	//Snapshot * Snapshot,
				//	//Reject: Rej,
				//}
				//r.msgs = append(r.msgs, *msg)

				for i := range r.Prs {
					if i != r.id {
						//r.sendAppend(i)
						t, _ := r.RaftLog.Term(r.Prs[m.GetFrom()].Match)
						msg := &pb.Message{
							MsgType: pb.MessageType_MsgAppend,
							To:      i,
							Index:   r.Prs[m.GetFrom()].Match, //回复目前的最新
							LogTerm: t,
							From:    r.id,
							Term:    r.Term,
							//Entries[] * Entry,
							Commit: r.RaftLog.committed,
							//Snapshot * Snapshot,
							//Reject: Rej,
						}
						r.msgs = append(r.msgs, *msg)
					}
				}

			}
		}
		r.RaftLog.stabled = r.RaftLog.committed
		//}
		//			r.RaftLog.committed = r.RaftLog.LastIndex()
	}

	//}

}

// sendHeartbeat sends a heartbeat RPC to the given peer.
// sendHeartbeat 将心跳 RPC 发送到给定的对等方。
func (r *Raft) sendHeartbeat(to uint64) {
	//带有m.Index = 0，m.LogTerm = 0和空条目的MessageType_MsgHeartbeat作为心跳。
	//3 - 4

	if r.State == StateLeader {
		g := r.RaftLog
		var en []*pb.Entry
		// en := make([]*pb.Entry, 0)
		// enx := g.getIndextoEntry(r.Prs[to].Match+1, r.Prs[to].Next)
		// for i := range enx {
		// 	en = append(en, &(enx[i]))
		// }
		// if len(en) == 0 {
		// 	en = nil
		// }
		msg := &pb.Message{
			MsgType: pb.MessageType_MsgHeartbeat,
			To:      to,
			Index:   0,
			LogTerm: 0,
			From:    r.id,
			Term:    r.Term,
			Entries: en,
			Commit:  g.committed,
			//Snapshot * Snapshot,
			//Reject               bool,
		}
		r.msgs = append(r.msgs, *msg)
	}
	// Your Code Here (2A).
}
func (r *Raft) handleHeartbeat(m pb.Message) {
	if r.State == StateLeader {
		for i := range r.Prs {
			if i != r.id {
				r.sendHeartbeat(i)
			}
			r.heartbeatElapsed = 0
		}
	}
}
func (r *Raft) handleHeartbeatRequest(m pb.Message) {
	if m.GetFrom() == r.Lead && r.State != StateLeader {
		r.electionElapsed = 0
	}

	g := r.RaftLog
	r.updateEntries(m.GetEntries())
	if m.GetCommit() > g.committed && m.GetCommit() <= r.RaftLog.LastIndex() {
		r.RaftLog.committed = m.GetCommit()
		r.RaftLog.stabled = r.RaftLog.committed
	}
	msg := &pb.Message{
		MsgType: pb.MessageType_MsgHeartbeatResponse,
		To:      m.GetFrom(),
		Index:   r.RaftLog.LastIndex(), //回复目前的最新
		//LogTerm: 0,
		From: r.id,
		Term: r.Term,
		//Entries[] * Entry,
		Commit: r.RaftLog.committed,
		//Snapshot * Snapshot,
		//Reject: Rej,
	}
	r.msgs = append(r.msgs, *msg)
}
func (r *Raft) handleHeartbeatResponse(m pb.Message) {
	min := uint64(9223372036854775807)
	r.Prs[m.GetFrom()].Match = m.Index
	cliet := make(uint64Slice, len(r.Prs))
	for i, v := range r.Prs {
		cliet[i-1] = v.Match
	}
	sort.Sort(cliet)
	min = cliet[(len(r.Prs)-1)/2]
	if r.RaftLog.committed != min {
		//to all
		indexterm, err := r.RaftLog.Term(min)
		if err == nil && indexterm == r.Term {
			r.RaftLog.committed = min
		}
		r.RaftLog.stabled = r.RaftLog.LastIndex()
	}
	if r.Prs[m.From].Match != r.Prs[m.From].Next {
		r.sendAppend(m.GetFrom())
	}
	// Your Code Here (2A).
}

//请求投票   RPC 发送到给定的对等方。
func (r *Raft) sendVoteRequest(to uint64) {
	uu, _ := r.RaftLog.Term(r.RaftLog.LastIndex())
	msg := &pb.Message{
		MsgType: pb.MessageType_MsgRequestVote,
		To:      to,
		Index:   r.RaftLog.LastIndex(),
		LogTerm: uu,
		//Entries[] * Entry,
		From:   r.id,
		Term:   r.Term,
		Commit: r.RaftLog.committed,
		//Snapshot * Snapshot,
		//Reject               bool,
	}
	r.msgs = append(r.msgs, *msg)

}
func (r *Raft) sendVoteResponse(to uint64, ok bool) {
	msg := &pb.Message{
		MsgType: pb.MessageType_MsgRequestVoteResponse,
		To:      to,
		// Index:   r.RaftLog.LastIndex(),
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
	if r.State != StateLeader {
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
}
func (r *Raft) handleVoteRequest(m pb.Message) {
	//如果term < currentTerm 拒绝
	//如果 votedFor 为空或者为 candidateId，并且候选人的日志至少和自己一样新，那么就投票给他（5.2 节，5.4 节）
	Rej := true
	if m.GetTerm() > r.Term || (m.GetTerm() == r.Term && (r.Vote == 0 || r.Vote == m.GetFrom())) {
		if r.RaftLog.LastIndex() == 0 {
			Rej = false
		} else {
			e := r.RaftLog.getIndextoEntry(r.RaftLog.LastIndex(), r.RaftLog.LastIndex())
			if e != nil {
				if m.GetLogTerm() > e[0].GetTerm() {
					Rej = false
				} else if m.GetLogTerm() == e[0].GetTerm() {
					if m.GetIndex() >= e[0].GetIndex() {
						Rej = false
					}
				}
			}
		}

	}
	if Rej == false {
		if r.State != StateFollower {
			r.becomeFollower(m.GetTerm(), m.GetFrom())
			r.Lead = 0
		}
		r.Vote = m.GetFrom()
	}
	// if m.GetTerm() > r.Term {
	// 	r.Term = m.GetTerm()
	// }
	r.sendVoteResponse(m.GetFrom(), Rej)
}
func (r *Raft) handleVoteResponse(m pb.Message) {
	if r.State == StateCandidate {
		r.votes[m.GetFrom()] = !m.Reject
		// r.Prs[m.GetFrom()].Match = m.Index
		// r.Prs[r.id].Match = r.RaftLog.LastIndex()
		//	Prs map[uint64]*Progress
		half := len(r.Prs) / 2
		agree := 0
		unagree := 0
		for _, ok := range r.votes {
			if ok {
				agree++
			} else {
				unagree++
			}
		}
		if agree > half {
			r.becomeLeader()
			return
		} else if unagree > half {
			r.becomeFollower(r.Term, 0)
			return
		} else {
			//ping票

			//cliet := make(uint64Slice, len(r.Prs))
			//for i, index := range r.Prs {
			//	cliet[i-1] = index.Match
			//}
			//sort.Sort(cliet)
			//mid := len(cliet)/2
			//mid_index := cliet[mid]
			//if r.Prs[r.id].Match < mid_index {
			//	r.becomeFollower(m.Term, 0)
			//}

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
		}
	}
	if r.State != StateLeader {
		r.electionElapsed++
		if r.electionElapsed >= r.electionTimeoutRand {
			r.electionElapsed = 0
			r.electionTimeoutRand = rand.Intn(r.electionTimeout) + r.electionTimeout
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
	r.Term = term
	r.Lead = lead
	r.Vote = lead
	r.State = StateFollower
	r.votes = make(map[uint64]bool)
	r.clearProcess()
	// Your Code Here (2A).
}

// becomeCandidate transform this peer's state to candidate
func (r *Raft) becomeCandidate() {
	r.clearProcess()
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
	en := make([]*pb.Entry, 0)
	en = append(en, &pb.Entry{
		EntryType: pb.EntryType_EntryNormal,
		Term:      r.Term,
		Index:     r.RaftLog.LastIndex() + 1, //从1开始
		Data:      nil,
		//XXX_NoUnkeyedLiteral: struct{}{},
		//XXX_unrecognized:     nil,
		//XXX_sizecache:        0,
	})
	err := r.Step(pb.Message{
		MsgType: pb.MessageType_MsgPropose,
		To:      r.id,
		From:    r.id,
		//Term:    r.Term,
		//LogTerm:              0,
		//Index:                0,
		Entries: en,
		//Commit:               0,
		//Snapshot:             nil,
		//Reject:               false,
		//XXX_NoUnkeyedLiteral: struct{}{},
		//XXX_unrecognized:     nil,
		//XXX_sizecache:        0,
	})
	if err != nil {
		log.S().Error(err.Error())
	}
	r.reSetProcess()

	// Your Code Here (2A).
	// NOTE: Leader should propose a noop entry on its term
	//领导者应该在其任期内提出一个 noop 条目
}

// Step the entrance of handle message, see `MessageType`
// on `eraftpb.proto` for what msgs should be handled
//步骤处理消息的入口，处理什么msgs见`erraftpb.proto`上的`MessageType`
func (r *Raft) Step(m pb.Message) error {
	// Your Code Here (2A).

	switch m.GetMsgType() {

	case pb.MessageType_MsgBeat:
		r.handleHeartbeat(m)
	case pb.MessageType_MsgHeartbeat:
		r.handleHeartbeatRequest(m)
	case pb.MessageType_MsgHeartbeatResponse:
		r.handleHeartbeatResponse(m)
		//-----------------------------------------------------
	case pb.MessageType_MsgHup:
		r.handleVote(m)
	case pb.MessageType_MsgRequestVote:
		r.handleVoteRequest(m)
	case pb.MessageType_MsgRequestVoteResponse:
		r.handleVoteResponse(m)
	//-----------------------------------------------------
	case pb.MessageType_MsgPropose:
		r.handlePropose(m)
	case pb.MessageType_MsgAppend:
		r.handleAppendEntries(m)
	case pb.MessageType_MsgAppendResponse:
		r.handleAppendEntriesResponse(m)

		//-------------------------------------------------
	case pb.MessageType_MsgSnapshot:
		r.handleSnapshot(m)

	}
	switch r.State {
	case StateFollower:
		if m.GetTerm() > r.Term {
			r.Term = m.GetTerm()
		}
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

func (r *Raft) reSetProcess() {
	//预测进度
	for k, _ := range r.Prs {
		r.Prs[k].Next = r.RaftLog.LastIndex() + 1
		//r.RaftLog.LastIndex() //1 开始
		//r.Prs[k].Match = 0
		//nextIndex[]	对于每一台服务器，发送到该服务器的下一个日志条目的索引（初始值为领导人最后的日志条目的索引+1）
		//matchIndex[]	对于每一台服务器，已知的已经复制到该服务器的最高日志条目的索引（初始值为0，单调递增）
		if k == r.id {
			r.Prs[k].Match = r.RaftLog.LastIndex()
		}
	}
}
func (r *Raft) clearProcess() {
	for i, _ := range r.Prs {
		r.Prs[i] = new(Progress)
		//nextIndex[]	对于每一台服务器，发送到该服务器的下一个日志条目的索引（初始值为领导人最后的日志条目的索引+1）
		//matchIndex[]	对于每一台服务器，已知的已经复制到该服务器的最高日志条目的索引（初始值为0，单调递增）
		r.Prs[i].Match = 0
		if i == r.id {
			r.Prs[i].Match = r.RaftLog.LastIndex()
		}
		r.Prs[i].Next = r.RaftLog.LastIndex() + 1
	}
}
func (r *Raft) updateEntries(e []*pb.Entry) {
	g := r.RaftLog
	if e == nil || len(e) == 0 {
		return
	}
	for _, v := range e {
		i := len(g.entries) - 1
		for ; i > -1; i-- {
			d := &(g.entries[i])
			if v.GetIndex() == d.Index {
				if !bytes.Equal(v.GetData(), d.Data) || v.GetTerm() != d.Term {
					d.Data = v.GetData()
					d.Term = v.GetTerm()
					if v.GetIndex()-1 >= 0 {
						g.stabled = min(v.GetIndex()-1, g.stabled)
					} else {
						g.stabled = 0
					}
					g.entries = g.entries[:i+1]
				}

				break
			}
		}
		if i == -1 {
			g.entries = append(g.entries, *v)
		}
	}
	//g.committed=
	//g.entries = g.entries[:g.stabled]
	//g.entries = g.getIndextoEntry(1, g.stabled)
}
