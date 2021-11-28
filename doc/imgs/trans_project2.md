该项目有 3 个你需要做的部分，包括：

- 实现基本的 Raft 算法
- 在 Raft 之上搭建一个容错的 KV 服务器
- 增加raftlog GC和snapshot的支持



## Part B

### 实施对等存储

在本部分中，您将使用a部分中实现的Raft模块构建一个容错键值存储服务。您的键值服务将是一个复制状态机，由几个使用Raft进行复制的键值服务器组成。您的密钥/值服务应该继续处理客户机请求，只要大多数服务器处于活动状态并且可以通信，尽管存在其他故障或网络分区。

在project1中，您已经实现了一个独立的kv服务器，因此您应该已经熟悉kv服务器API和存储接口。

在介绍代码之前，您需要先了解三个术语：proto/proto/metapb.proto中定义的Store、Peer和Region。

- Store代表tinykv服务器的一个实例
- Peer表示在存储上运行的Raft节点
- 区域是对等点的集合，也称为Raft组

![img.png](img.png)

为简单起见，project2 的 Store 上只有一个 Peer，集群中只有一个 Region。所以你现在不需要考虑Region的范围。在project3中将进一步引入多个区域。

### The Code



这些元数据应该在“PeerStorage”中创建和更新。创建 PeerStorage 时，请参阅 `kvraftstorepeer_storage.go`。它初始化这个Peer的RaftLocalState、RaftApplyState，或者在重启的情况下从底层引擎获取之前的值。注意RAFT_INIT_LOG_TERM和RAFT_INIT_LOG_INDEX的值都是5（只要大于1），但不是0，不设置为0是为了区分peer修改后被动创建的情况。你现在可能不太明白，所以记住它，当你实施conf更改时，细节将在project3b中描述。

这部分你需要实现的代码只有一个函数：`PeerStorage.SaveReadyState`，这个函数的作用是将`raft.Ready`中的数据保存到badger，包括追加日志条目和保存Raft硬状态。

要附加日志项，只需将所有日志项保存在raft.Ready.entries中，并删除以前附加的、永远不会提交的日志项。另外，更新对等存储的RaftLocalState并将其保存到raftdb。

要保存硬状态也很容易，只需更新对等存储的RaftLocalState.HardState并将其保存到raftdb。

为了保存硬状态也很容易，只是更新对等体存储的raftlocalstate.hardstate并将其保存到Raftdb。

提示：
- 使用WriteBatch立即保存这些状态。
- 有关如何读取和写入这些状态，请参阅peer_storage.go上的其他函数。

### Implement Raft ready process
### 实施筏式准备流程

在Project2部分A中，您已构建基于刻度的筏模块。现在您需要编写外部进程来驱动它。大多数代码已经在kv / roaftstore / peer_msg_mandler.go和kv / raftstore / peer.go下实现。因此，您需要了解代码并完成ProposeraftCommand和Handleraftready的逻辑。以下是对框架的一些解释。

节点已使用PeerStorage创建并存储在peer中。在raft worker中，您可以看到它使用peerMsgHandler将对等机包装起来。peerMsgHandler主要有两个功能：一个是HandleMsg，另一个是HandleRaftReady。

HandleMsg处理从raftCh接收的所有消息，包括调用RawNode.Tick（）来驱动Raft的MsgTypeTick、包装来自客户端的请求的MsgTypeRaftCmd和在Raft对等方之间传输的消息MsgTypeRaftMessage。所有消息类型都在kv/raftstore/message/msg.go中定义。您可以查看详细信息，其中一些将在以下部分中使用。

处理消息后，Raft节点应该有一些状态更新。因此，HandleRaftReady应该从Raft模块准备就绪，并执行相应的操作，如持久化日志条目、应用提交的条目以及通过网络向其他对等方发送Raft消息。

在伪代码中，raftstore 使用 Raft 如下：

for {
select {
case <-s.Ticker:
Node.Tick()
default:
if Node.HasReady() {
rd := Node.Ready()
saveToStorage(rd.State, rd.Entries, rd.Snapshot)
send(rd.Messages)
for _, entry := range rd.CommittedEntries {
process(entry)
}
s.Node.Advance(rd)
}
}

在此之后，整个读或写过程如下：

- 客户机调用 RPC RawGet/RawPut/RawDelete/RawScan
- RPC 处理程序调用与 RaftStorage 相关的方法
- RaftStorage 向 raftstore 发送一个 Raft 命令请求，并等待响应
- 将 Raft 命令请求作为一个 Raft 日志提出
- Raft 模块附加日志，并通过 PeerStorage 持久化
- Raft 模块提交日志
- Raft worker在处理Raft ready时执行Raft命令，并通过回调返回响应
- RaftStorage 接收回调的响应并返回给 RPC 处理程序
- RPC 处理程序执行一些操作并将 RPC 响应返回给客户机。

您应该运行makeproject2b以通过所有测试。整个测试运行一个模拟集群，其中包括多个TinyKV实例和一个模拟网络。它执行一些读写操作，并检查返回值是否符合预期。

需要注意的是，错误处理是通过测试的一个重要部分。您可能已经注意到proto/proto/errorpb.proto中定义了一些错误，错误是gRPC响应的一个字段。此外，实现error接口的相应错误在kv/raftstore/util/error.go中定义，因此您可以将它们用作函数的返回值。

在这个阶段，您可以考虑这些错误，其他将在项目3中进行处理：

ErrNotLeader：raft命令是针对跟随者提出的。因此，使用它让客户机尝试其他对等机。

ErrStaleCommand：可能是由于领导者的更改，某些日志未提交并被新领导者的日志覆盖。但客户不知道，仍在等待响应。因此，您应该返回它，让客户端知道并再次重试该命令。

提示：

- PeerStorage实现Raft模块的存储接口，您应该使用提供的方法SaveRaftReady（）来持久化与Raft相关的状态。

- 使用engine_util中的WriteBatch以原子方式进行多个写入，例如，您需要确保在一个写入批中应用提交的条目并更新应用的索引。

- 使用Transport向其他对等方发送raft消息，它位于GlobalContext中，

- 如果get RPC不是多数的一部分，并且没有最新数据，则服务器不应完成它。您可以将get操作放入raft日志中，或者对raft文章第8节中描述的只读操作进行优化。

- 应用日志条目时，不要忘记更新并保持应用状态。

- 您可以像TiKV一样以异步方式应用提交的Raft日志条目。虽然提高性能是一个巨大的挑战，但这并不是必须的。

- 建议时记录命令的回调，应用后返回回调。

- 对于snap命令响应，应将badger Txn显式设置为callback。

