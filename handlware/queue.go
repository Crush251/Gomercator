package handlware

import (
	"container/heap"
)

// ==================== 消息优先队列 ====================
// 用于模拟消息传播过程，按接收时间（RecvTime）排序

// MessageQueue 消息优先队列（最小堆）
type MessageQueue []*Message

// Len 实现heap.Interface
func (mq MessageQueue) Len() int {
	return len(mq)
}

// Less 实现heap.Interface，按RecvTime升序排列
func (mq MessageQueue) Less(i, j int) bool {
	return mq[i].RecvTime < mq[j].RecvTime
}

// Swap 实现heap.Interface
func (mq MessageQueue) Swap(i, j int) {
	mq[i], mq[j] = mq[j], mq[i]
}

// Push 实现heap.Interface
func (mq *MessageQueue) Push(x interface{}) {
	msg := x.(*Message)
	*mq = append(*mq, msg)
}

// Pop 实现heap.Interface
func (mq *MessageQueue) Pop() interface{} {
	old := *mq
	n := len(old)
	msg := old[n-1]
	*mq = old[0 : n-1]
	return msg
}

// ==================== 优先队列包装器 ====================

// PriorityQueue 优先队列包装器，提供更友好的接口
type PriorityQueue struct {
	queue MessageQueue
}

// NewPriorityQueue 创建新的优先队列
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		queue: make(MessageQueue, 0),
	}
	heap.Init(&pq.queue)
	return pq
}

// Push 添加消息到队列
func (pq *PriorityQueue) Push(msg *Message) {
	heap.Push(&pq.queue, msg)
}

// Pop 取出接收时间最早的消息
func (pq *PriorityQueue) Pop() *Message {
	if pq.Empty() {
		return nil
	}
	return heap.Pop(&pq.queue).(*Message)
}

// Empty 检查队列是否为空
func (pq *PriorityQueue) Empty() bool {
	return pq.queue.Len() == 0
}

// Len 获取队列长度
func (pq *PriorityQueue) Len() int {
	return pq.queue.Len()
}
