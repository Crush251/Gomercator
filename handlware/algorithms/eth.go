package algorithms

import (
	"fmt"
	"math/rand"

	hw "gomercator/handlware"
)

// ==================== ETH 算法 ====================
// ETH: 基于 Kademlia 风格 k-bucket 的 Gossip 广播
// 1. 使用 128-bit NodeID 和 XOR 距离度量
// 2. k-bucket 路由表：桶号由 XOR 距离的最高位位置决定
// 3. 转发策略：从所有连接节点（PeerSet）中随机选择 X 个转发
//    其中 X = 非空桶数量 × F

// ETH ETH算法实现
type ETH struct {
	hw.BaseAlgorithm                       // 继承基础算法
	NodeIDs          []hw.NodeID128        // 每个节点的 128-bit ID
	KBuckets         []hw.KBucketTable     // 每个节点的 k-bucket 路由表
	PeerSets         [][]int               // PeerSets[i] = 节点 i 的所有连接节点（所有桶的并集）
	Coords           []hw.LatLonCoordinate // 真实坐标（用于 RTT 评估）
	Config           hw.KBucketConfig      // k-bucket 配置
	Visited          [][]bool              // 访问标记 Visited[nodeID][step]
	Rng              *rand.Rand            // 随机数生成器
}

// NewETH 创建新的 ETH 算法实例
// 参数:
//   - n: 节点数
//   - coords: 节点坐标数组
//   - config: k-bucket 配置参数
//
// 返回: ETH 算法实例
func NewETH(n int, coords []hw.LatLonCoordinate, config hw.KBucketConfig) *ETH {
	eth := &ETH{
		BaseAlgorithm: hw.BaseAlgorithm{
			Name:          "eth",
			SpecifiedRoot: false,
			Graph:         hw.NewGraph(n),
			Coords:        coords,
			Root:          0,
		},
		NodeIDs:  make([]hw.NodeID128, n),
		KBuckets: make([]hw.KBucketTable, n),
		PeerSets: make([][]int, n),
		Coords:   coords,
		Config:   config,
		Visited:  make([][]bool, n),
		Rng:      rand.New(rand.NewSource(42)),
	}

	// 初始化 Visited 数组
	for i := 0; i < n; i++ {
		eth.Visited[i] = make([]bool, hw.MaxDepth)
	}

	fmt.Println("构建 ETH 拓扑...")

	// 步骤1：为每个节点生成随机 128-bit NodeID
	fmt.Printf("  步骤1: 生成 %d 个随机 NodeID...\n", n)
	eth.generateNodeIDs(n)

	// 步骤2：预构建所有节点的 k-buckets
	fmt.Printf("  步骤2: 构建 k-bucket 路由表（每桶最多 %d 个节点）...\n", config.K)
	eth.buildKBuckets(n)

	// 步骤3：构建 PeerSets（所有桶的并集）
	fmt.Printf("  步骤3: 构建 PeerSets（所有连接节点集合）...\n")
	eth.buildPeerSets(n)

	// 统计信息
	eth.printStatistics(n)

	return eth
}

// generateNodeIDs 为每个节点生成随机 128-bit NodeID
func (eth *ETH) generateNodeIDs(n int) {
	for i := 0; i < n; i++ {
		eth.NodeIDs[i] = hw.GenerateRandomNodeID()
	}
}

// buildKBuckets 预构建所有节点的 k-buckets
func (eth *ETH) buildKBuckets(n int) {
	// 初始化每个节点的 k-bucket 表
	for i := 0; i < n; i++ {
		eth.KBuckets[i].Buckets = make([][]int, eth.Config.NumBits)
		for j := 0; j < eth.Config.NumBits; j++ {
			eth.KBuckets[i].Buckets[j] = make([]int, 0, eth.Config.K)
		}
	}

	// 对每个节点 i，遍历所有其他节点 j
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			// 计算 XOR 距离
			dist := hw.XORDistance(eth.NodeIDs[i], eth.NodeIDs[j])

			// 计算桶索引
			bucketIdx := hw.BucketIndex(dist)
			if bucketIdx < 0 {
				// 距离为 0（不应该发生，因为 NodeID 应该唯一）
				continue
			}

			// 将节点 j 添加到节点 i 的对应桶中（不超过 K 个）
			if len(eth.KBuckets[i].Buckets[bucketIdx]) < eth.Config.K {
				eth.KBuckets[i].Buckets[bucketIdx] = append(eth.KBuckets[i].Buckets[bucketIdx], j)
			}
		}
	}
}

// buildPeerSets 构建每个节点的 PeerSet（所有桶的并集）
func (eth *ETH) buildPeerSets(n int) {
	for i := 0; i < n; i++ {
		peerSet := make(map[int]bool)

		// 遍历所有桶，收集所有节点
		for bucketIdx := 0; bucketIdx < eth.Config.NumBits; bucketIdx++ {
			for _, peer := range eth.KBuckets[i].Buckets[bucketIdx] {
				peerSet[peer] = true
			}
		}

		// 转换为切片
		eth.PeerSets[i] = make([]int, 0, len(peerSet))
		for peer := range peerSet {
			eth.PeerSets[i] = append(eth.PeerSets[i], peer)
		}
	}
}

// printStatistics 打印统计信息
func (eth *ETH) printStatistics(n int) {
	// 统计每个节点的桶分布和 PeerSet 大小
	totalPeers := 0
	totalNonEmptyBuckets := 0

	for i := 0; i < n; i++ {
		nonEmptyBuckets := 0
		for bucketIdx := 0; bucketIdx < eth.Config.NumBits; bucketIdx++ {
			if len(eth.KBuckets[i].Buckets[bucketIdx]) > 0 {
				nonEmptyBuckets++
			}
		}
		totalNonEmptyBuckets += nonEmptyBuckets
		totalPeers += len(eth.PeerSets[i])
	}

	avgPeersPerNode := float64(totalPeers) / float64(n)
	avgNonEmptyBucketsPerNode := float64(totalNonEmptyBuckets) / float64(n)

	fmt.Printf("  统计信息:\n")
	fmt.Printf("    平均每节点 PeerSet 大小: %.2f\n", avgPeersPerNode)
	fmt.Printf("    平均每节点非空桶数: %.2f\n", avgNonEmptyBucketsPerNode)
	fmt.Printf("    平均每次转发节点数 (X=非空桶数×F): %.2f\n", avgNonEmptyBucketsPerNode*float64(eth.Config.Fanout))
}

// Respond 实现 Algorithm 接口 - 响应消息
// ETH Gossip 转发策略：
//  1. 计算非空桶的数量
//  2. X = 非空桶数量 × F
//  3. 从 PeerSet 中随机选择 X 个节点转发
func (eth *ETH) Respond(msg *hw.Message) []int {
	u := msg.Dst
	relayNodes := make([]int, 0)

	// 检查是否已访问过
	if eth.Visited[u][msg.Step] {
		return relayNodes
	}

	eth.Visited[u][msg.Step] = true

	// 计算非空桶的数量
	nonEmptyBuckets := 0
	for bucketIdx := 0; bucketIdx < eth.Config.NumBits; bucketIdx++ {
		if len(eth.KBuckets[u].Buckets[bucketIdx]) > 0 {
			nonEmptyBuckets++
		}
	}

	// X = 非空桶数量 × F
	X := nonEmptyBuckets * eth.Config.Fanout

	// 从 PeerSet 中随机选择 X 个节点
	selected := eth.randomSelectN(eth.PeerSets[u], X)
	for _, peer := range selected {
		if peer != msg.Src {
			relayNodes = append(relayNodes, peer)
		}
	}

	return relayNodes
}

// randomSelectN 从候选节点中随机选择 n 个
func (eth *ETH) randomSelectN(candidates []int, n int) []int {
	if len(candidates) <= n {
		return candidates
	}

	// 使用 Fisher-Yates shuffle 选择 n 个
	selected := make([]int, n)
	indices := eth.Rng.Perm(len(candidates))
	for i := 0; i < n; i++ {
		selected[i] = candidates[indices[i]]
	}

	return selected
}

// SetRoot 实现 Algorithm 接口 - 设置广播根节点
func (eth *ETH) SetRoot(root int) {
	eth.Root = root
	// 重置 Visited 标记
	for i := 0; i < len(eth.Visited); i++ {
		for j := 0; j < len(eth.Visited[i]); j++ {
			eth.Visited[i][j] = false
		}
	}
}

// GetAlgoName 实现 Algorithm 接口 - 获取算法名称
func (eth *ETH) GetAlgoName() string {
	return fmt.Sprintf("eth_k%d_f%d", eth.Config.K, eth.Config.Fanout)
}

// NeedSpecifiedRoot 实现 Algorithm 接口 - 是否需要为每个根重建
func (eth *ETH) NeedSpecifiedRoot() bool {
	return false
}

// PrintInfo 打印算法信息（调试用）
func (eth *ETH) PrintInfo() {
	fmt.Printf("ETH: K=%d, Fanout=%d, NumBits=%d\n",
		eth.Config.K, eth.Config.Fanout, eth.Config.NumBits)
}
