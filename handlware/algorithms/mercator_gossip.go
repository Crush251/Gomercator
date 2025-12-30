package algorithms

import (
	"fmt"
	"math/rand"

	hw "gomercator/handlware"
)

// ==================== MERCATOR GOSSIP算法 ====================
// MERCATOR GOSSIP: 基于Mercator拓扑，在K0桶使用Gossip策略
// 核心思想:
// 1. 复用Mercator的K桶拓扑结构
// 2. K0桶：使用Gossip策略（随机选择部分节点）而不是Flooding或K-ary树
// 3. 其他桶：保持Mercator的原有策略
// 4. 每个节点收到k0桶消息后，都会在k0桶内进行gossip传播

// MercatorGossip Mercator Gossip算法实现
type MercatorGossip struct {
	*Mercator               // 嵌入Mercator，复用所有拓扑结构
	GossipFanout int        // Gossip扇出（每次随机选择的节点数）
	Rng          *rand.Rand // 随机数生成器
}

// NewMercatorGossip 创建新的MercatorGossip算法实例
// 参数:
//   - mercator: 已构建好的Mercator实例（复用其拓扑）
//   - gossipFanout: Gossip扇出（每次随机选择的节点数，默认使用BucketSize）
//
// 返回: MercatorGossip实例
func NewMercatorGossip(mercator *Mercator, gossipFanout int) *MercatorGossip {
	if gossipFanout <= 0 {
		gossipFanout = mercator.BucketSize
	}
	if gossipFanout <= 0 {
		gossipFanout = 10 // 默认值
	}

	return &MercatorGossip{
		Mercator:     mercator,
		GossipFanout: gossipFanout,
		Rng:          rand.New(rand.NewSource(100)), // 固定种子确保可重复性
	}
}

// Respond 实现Algorithm接口 - 响应消息（K0桶使用Gossip策略）
func (mg *MercatorGossip) Respond(msg *hw.Message) []int {
	u := msg.Dst
	relayNodes := make([]int, 0)

	// 如果已访问过，返回空列表
	if mg.Visited[u][msg.Step] {
		return relayNodes
	}

	mg.Visited[u][msg.Step] = true

	// 策略：K0桶使用Gossip，其他桶保持Mercator策略
	if msg.Step == 0 {
		// 消息源节点
		// 这是Gossip策略的核心：每个收到消息的节点都会在k0桶内进行gossip传播
		if len(mg.KBuckets[u][0]) > 40 {
			relayNodes = gossipnodes(mg, u, msg, relayNodes)
		} else {
			for _, v := range mg.KBuckets[u][0] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

		// 第二步：从每个非K0桶中选择节点进行传播（保持Mercator策略）
		for bucketIdx := 1; bucketIdx < len(mg.KBuckets[u]); bucketIdx++ {
			for _, v := range mg.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

	} else {
		// 非消息源节点
		// 获取消息源所在的桶号
		srcBucket := hw.GetGeoBucketIndex(mg.NodeGeohash[u], mg.NodeGeohash[msg.Src], mg.TotalBits)

		// 关键修复：无论消息从哪个桶传来，只要当前节点有k0桶节点，都应该进行k0桶的gossip
		// 这是Gossip策略的核心：每个收到消息的节点都会在k0桶内进行gossip传播
		if len(mg.KBuckets[u][0]) > 20 {
			relayNodes = gossipnodes(mg, u, msg, relayNodes)
		} else {
			for _, v := range mg.KBuckets[u][0] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

		// 从小于srcBucket的桶中选择节点转发（保持Mercator策略）
		for bucketIdx := 1; bucketIdx < srcBucket; bucketIdx++ {
			for _, v := range mg.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

		// 字符级 XOR 触发的额外转发（保持Mercator策略）
		picked := make(map[int]struct{}, len(relayNodes)+4)
		for _, v := range relayNodes {
			picked[v] = struct{}{}
		}
		extra := mg.extraForwardByCharXOR(u, msg.Src, picked)
		if len(extra) > 0 {
			relayNodes = append(relayNodes, extra...)
		}
	}

	return relayNodes
}

func gossipnodes(mg *MercatorGossip, u int, msg *hw.Message, relayNodes []int) []int {
	k0Nodes := make([]int, 0)
	for _, v := range mg.KBuckets[u][0] {
		if v != msg.Src {
			k0Nodes = append(k0Nodes, v)
		}
	}

	// Gossip策略：随机选择部分节点（无论srcBucket是多少）
	if len(k0Nodes) > 0 {
		selected := mg.selectGossipNodes(k0Nodes, mg.GossipFanout)
		relayNodes = append(relayNodes, selected...)
	}
	return relayNodes
}

// selectGossipNodes 从节点列表中随机选择gossip节点
// 参数:
//   - nodes: 候选节点列表
//   - fanout: 需要选择的节点数
//
// 返回: 选中的节点列表
func (mg *MercatorGossip) selectGossipNodes(nodes []int, fanout int) []int {
	if len(nodes) <= fanout {
		// 如果候选节点数少于等于fanout，全部选择
		return nodes
	}

	// 随机选择fanout个节点
	selected := make([]int, 0, fanout)
	indices := make([]int, len(nodes))
	for i := range indices {
		indices[i] = i
	}

	// Fisher-Yates洗牌算法
	for i := len(indices) - 1; i >= 0 && len(selected) < fanout; i-- {
		j := mg.Rng.Intn(i + 1)
		indices[i], indices[j] = indices[j], indices[i]
		selected = append(selected, nodes[indices[i]])
	}

	return selected
}

// extraForwardByCharXOR 复用Mercator的字符级XOR转发逻辑
func (mg *MercatorGossip) extraForwardByCharXOR(u, sender int, already map[int]struct{}) []int {
	return mg.Mercator.extraForwardByCharXOR(u, sender, already)
}

// SetRoot 实现Algorithm接口 - 设置广播根节点
func (mg *MercatorGossip) SetRoot(root int) {
	mg.Mercator.SetRoot(root)
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (mg *MercatorGossip) GetAlgoName() string {
	return "mercator_gossip"
}

// NeedSpecifiedRoot 实现Algorithm接口 - 是否需要为每个根重建
func (mg *MercatorGossip) NeedSpecifiedRoot() bool {
	return false // 复用Mercator拓扑，不需要重建
}

// PrintInfo 打印算法信息（调试用）
func (mg *MercatorGossip) PrintInfo() {
	fmt.Printf("MERCATOR GOSSIP: 基于Mercator拓扑，K0桶使用Gossip策略\n")
	fmt.Printf("  Gossip扇出: %d\n", mg.GossipFanout)
	mg.Mercator.PrintInfo()
}
