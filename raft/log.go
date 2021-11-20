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
	"fmt"
	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

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
	//storage 包含自上次快照以来的所有稳定条目。 存放已经持久化数据的Storage接口。
	storage Storage

	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	// 已提交是已知在法定节点上的稳定存储中的最高日志位置 保存当前提交的日志数据索引。
	committed uint64

	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	// 已指示应用程序应用于其状态机的最高日志位置。 不变量：已应用 <= 已提交
	// 保存当前传入状态机的数据最高索引。
	applied uint64

	// log entries with index <= stabled are persisted to storage.
	// It is used to record the logs that are not persisted by storage yet.
	// Everytime handling `Ready`, the unstabled logs will be included.
	// index <= stable 的日志条目被持久化到存储中。用于记录还没有被storage持久化的日志。每次处理 `Ready` 时，都会包含不稳定的日志。
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
	log := &RaftLog{
		storage:         storage,
		committed:       0,
		applied:         0,
		stabled:         0,
		entries:         make([]pb.Entry, 0),
		pendingSnapshot: nil,
	}
	//storage.
	state, _, err2 := storage.InitialState()
	if err2 == nil {
		log.committed = state.Commit
		log.applied = state.Commit
	}
	//n,_:=log.storage.Entries()
	//first, _ := storage.FirstIndex()
	last, _ := storage.LastIndex()
	//eaaa, _ := storage.Entries(first, last+1)
	eaaa := log.getIndextoEntry(0, last)
	if eaaa != nil {
		log.entries = append(log.entries, eaaa...)
	}
	log.stabled = last //TODO:填什么
	//log.stabled = log.LastIndex()
	return log
	//  snapshot/first.....applied....committed....stabled.....last
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
	temp := make([]pb.Entry, 0)
	if l.stabled+1 > l.LastIndex() {
		return temp
	}
	r := l.getIndextoEntry(l.stabled+1, l.LastIndex())
	if r != nil {
		temp = r
	}
	return temp
	//return l.entries[l.stabled:]
	//return l.entries[l.stabled:]
	//return l.entries
	// Your Code Here (2A).
}

// nextEnts returns all the committed but not applied entries
//nextEnts 返回所有已提交但未应用的条目
func (l *RaftLog) nextEnts() (ents []pb.Entry) {
	//return l.entries[l.applied:l.committed]
	return l.getIndextoEntry(l.applied+1, l.committed)
}

// LastIndex return the last index of the log entries
//LastIndex 返回日志条目的最后一个索引
func (l *RaftLog) LastIndex() uint64 {
	// Your Code Here (2A).
	temp, _ := l.storage.LastIndex()
	if len(l.entries) != 0 {
		return l.entries[(uint64(len(l.entries)) - 1)].GetIndex()
	} else {
		return temp
	}
}

// Term return the term of the entry in the given index
//Term 返回给定索引中条目entry的术语term
func (l *RaftLog) Term(i uint64) (uint64, error) {
	// Your Code Here (2A).
	if i > l.LastIndex() || i <= 0 {
		return 0, ErrStepPeerNotFound
	} else {
		temp := l.getIndextoEntry(i, i)
		if temp != nil {
			return temp[0].Term, nil
		}
	}
	return 0, nil
}

//闭区间
func (l *RaftLog) getIndextoEntry(left, right uint64) (ents []pb.Entry) {
	//1 all storage
	//2 all log.entries
	//3 double
	//4 none
	//fmt.Println(left, right)
	n, _ := l.storage.LastIndex()
	m := l.LastIndex()
	if left < 1 {
		left = 1
	}
	if right > m {
		right = m
	}
	if left > right {
		return nil
	}

	if n == 0 && len(l.entries) == 0 {
		return nil
	}

	if n == 0 {
		return l.entries[left-1 : right]
	}

	if len(l.entries) == 0 {
		//1
		//first, _ := l.storage.FirstIndex()
		temp, _ := l.storage.Entries(left, right+1)
		return temp

	}
	if n < l.entries[0].Index {
		temp, _ := l.storage.Entries(left, n+1)
		temp = append(temp, l.entries[0:right-l.entries[0].Index+1]...)
		return temp
	} else {
		i := int(len(l.entries) - 1)
		ll := -1
		rr := -1
		for ; i > -1; i-- {
			if l.entries[i].Index == right {
				rr = i
			}
			if l.entries[i].Index == left {
				ll = i
			}
		}
		if ll != -1 && rr != -1 {
			//temp, _ := l.storage.Entries(uint64(ll), uint64(rr+1))
			temp := l.entries[uint64(ll):uint64(rr+1)]
			return temp
		} else if rr == -1 {
			//over
			fmt.Println("TODO: getIndextoEntry")
			return nil
		} else {
			//ll 上 rr 下
			tempxia := l.entries[0:uint64(rr+1)]
			tempshang, _ := l.storage.Entries(1, n+1)

			i := l.entries[0].Index - 1
			for ; i > 0; i-- {
				if tempshang[i].Index == left {
					break
				}
			}
			sum := tempshang[:i+1]
			sum = append(sum, tempxia...)
			return sum

		}

		//if right > n {
		//	temp, _ := l.storage.Entries(left, n+1)
		//	//temp2 := l.entries[:]
		//	dert := l.LastIndex() - n
		//	lee := uint64(len(l.entries))
		//	chong := lee - dert
		//
		//	temp = append(temp, l.entries[chong:i+1]...)
		//	return temp
		//} else {
		//	temp, _ := l.storage.Entries(left, right+1)
		//	return temp
		//}

	}
	//temp, _ := l.storage.Entries(left, n+1)
	////temp2 := l.entries[:]
	//dert := l.LastIndex() - n
	//lee := uint64(len(l.entries))
	//chong := lee - dert
	//temp = append(temp, l.entries[chong:right-chong+1]...)
	//return temp
	//3
	//return nil

}
