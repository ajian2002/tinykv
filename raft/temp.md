```go


// 'MessageType_MsgBeat' is a local message that signals the leader to send a heartbeat
// of the 'MessageType_MsgHeartbeat' type to its followers.
//'MessageType_MsgBeat' 是一个本地消息，它通知领导者向其追随者发送一个“MessageType_MsgHeartbeat”类型的心跳。
const MessageType_MsgBeat MessageType = 1

// 'MessageType_MsgHeartbeat' sends heartbeat from leader to its followers.
//'MessageType_MsgHeartbeat' 从领导者向其追随者发送心跳。
const MessageType_MsgHeartbeat MessageType = 8

// 'MessageType_MsgHeartbeatResponse' is a response to 'MessageType_MsgHeartbeat'.
//“MessageType_MsgHeartbeatResponse”是对“MessageType_MsgHeartbeat”的响应。
const MessageType_MsgHeartbeatResponse MessageType = 9

//-------------------------------------------------------------------------------------------------------

// 'MessageType_MsgHup' is a local message used for election. If an election timeout happened,
// the node should pass 'MessageType_MsgHup' to its Step method and start a new election.
//“MessageType_MsgHup”是用于选举的本地消息。 如果发生选举超时，节点应将“MessageType_MsgHup”传递给其 Step 方法并开始新的选举。
const MessageType_MsgHup MessageType = 0

// 'MessageType_MsgRequestVote' requests votes for election.
//'MessageType_MsgRequestVote' 请求选举投票。
const MessageType_MsgRequestVote MessageType = 5

// 'MessageType_MsgRequestVoteResponse' contains responses from voting request.
//“MessageType_MsgRequestVoteResponse”包含来自投票请求的响应。
const MessageType_MsgRequestVoteResponse MessageType = 6

//-------------------------------------------------------------------------------------------------------

//“MessageType_MsgPropose”是一条本地消息，建议将数据附加到领导者的日志条目中。
// 'MessageType_MsgPropose' is a local message that proposes to append data to the leader's log entries.
const MessageType_MsgPropose MessageType = 2

// 'MessageType_MsgAppend' contains log entries to replicate.
//“MessageType_MsgAppend”包含要复制的日志条目。
const MessageType_MsgAppend MessageType = 3

// 'MessageType_MsgAppendResponse' is response to log replication request('MessageType_MsgAppend').
//'MessageType_MsgAppendResponse' 是对日志复制请求（'MessageType_MsgAppend'）的响应。
const MessageType_MsgAppendResponse MessageType = 4

// 'MessageType_MsgSnapshot' requests to install a snapshot message.
//'MessageType_MsgSnapshot' 请求安装快照消息
const MessageType_MsgSnapshot MessageType = 7

// 'MessageType_MsgTransferLeader' requests the leader to transfer its leadership.
//'MessageType_MsgTransferLeader' 请求领导者转移其领导权。
const MessageType_MsgTransferLeader MessageType = 11

// 'MessageType_MsgTimeoutNow' send from the leader to the leadership transfer target, to let
// the transfer target timeout immediately and start a new election.
//'MessageType_MsgTimeoutNow'从leader发送给leader的转移目标，让转移目标立即超时，开始新的选举。
const MessageType_MsgTimeoutNow MessageType = 12
```
