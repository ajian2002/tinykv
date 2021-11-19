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

import pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"

// RaftLog manage the log entries, its struct look like:
//
//  snapshot/first.....applied....committed....stabled.....last
//  --------|------------------------------------------------|
//                            log entries
//
// for simplify the RaftLog implement should manage all log entries
// that not truncated
// RaftLog 管理日志条目，其结构如下：
// snapshot/first.....applied....committed....stabled.....last
// --------|------------------------------------------------|
//                           log entries
//为了简化 RaftLog 工具应该管理所有未被截断的日志条目
type RaftLog struct {
	// storage contains all stable entries since the last snapshot.
	//storage 包含自上次快照以来的所有稳定条目。
	storage Storage

	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	// 已提交是已知在法定节点上的稳定存储中的最高日志位置
	committed uint64

	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	//应用是已指示应用程序应用于其状态机的最高日志位置。 不变量：已应用 <= 已提交
	applied uint64

	// log entries with index <= stabled are persisted to storage.
	// It is used to record the logs that are not persisted by storage yet.
	// Everytime handling `Ready`, the unstabled logs will be included.
	//索引 <= stable 的日志条目被持久化到存储中。用于记录还没有被storage持久化的日志。每次处理 `Ready` 时，都会包含不稳定的日志。
	stabled uint64

	// all entries that have not yet compact.
	//所有尚未压缩的条目
	entries []pb.Entry

	// the incoming unstable snapshot, if any.
	// (Used in 2C)
	//传入的不稳定快照（如果有）。 （用于 2C）
	pendingSnapshot *pb.Snapshot

	// Your Data Here (2A).
}

// newLog returns log using the given storage. It recovers the log
// to the state that it just commits and applies the latest snapshot.
//newLog 使用给定的存储返回日志。 它将日志恢复到它刚刚提交并应用最新快照的状态。
func newLog(storage Storage) *RaftLog {
	//	//type RaftLog struct {
	//	//	storage         Storage
	//	//	committed       uint64
	//	//	applied         uint64
	//	//	stabled         uint64
	//	//	entries         []eraftpb.Entry
	//	//	pendingSnapshot *eraftpb.Snapshot
	//	//}

	//type Storage interface {
	//    InitialState() (eraftpb.HardState, eraftpb.ConfState, error)
	//    Entries(lo uint64, hi uint64) ([]eraftpb.Entry, error)
	//    Term(i uint64) (uint64, error)
	//    LastIndex() (uint64, error)
	//    FirstIndex() (uint64, error)
	//    Snapshot() (eraftpb.Snapshot, error)
	//}
	return &RaftLog{
		storage:   storage,
		committed: 0,
		applied:   0,
		stabled:   0,
		//entries:        storage.Entries(),
		pendingSnapshot: nil,
	}
	// Your Code Here (2A).
}

// We need to compact the log entries in some point of time like
// storage compact stabled log entries prevent the log entries
// grow unlimitedly in memory
//我们需要在某个时间点压缩日志条目，例如存储压缩稳定的日志条目，防止日志条目在内存中无限增长
func (l *RaftLog) maybeCompact() {

	// Your Code Here (2C).
}

// unstableEntries return all the unstable entries
//不稳定条目返回所有不稳定的条目
func (l *RaftLog) unstableEntries() []pb.Entry {
	return l.entries
	// Your Code Here (2A).
	//return nil
}

// nextEnts returns all the committed but not applied entries
//nextEnts 返回所有已提交但未应用的条目
func (l *RaftLog) nextEnts() (ents []pb.Entry) {
	// Your Code Here (2A).
	//return
	return nil
}

// LastIndex return the last index of the log entries
//LastIndex 返回日志条目的最后一个索引
func (l *RaftLog) LastIndex() uint64 {
	// Your Code Here (2A).
	return l.committed
	//return 0
}

// Term return the term of the entry in the given index
//Term 返回给定索引中条目entry的术语term
func (l *RaftLog) Term(i uint64) (uint64, error) {
	// Your Code Here (2A).
	if i > uint64(len(l.entries)) {
		return 0, ErrStepPeerNotFound
	} else {
		return l.entries[i].Term, nil
	}
	//return 0, nil
}
