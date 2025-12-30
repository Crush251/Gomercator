package algorithms

import (
	"fmt"
	"math/rand"

	hw "gomercator/handlware"
)

// ==================== Kadcast 算法 ====================
// Kadcast: 基于 Kademlia 风格 k-bucket 的结构化广播
// 1. 使用 128-bit NodeID 和 XOR 距离度量
// 2. k-bucket 路由表：桶号由 XOR 距离的最高位位置决定
// 3. 转发策略：对桶 0..h-1 各选 F 个节点转发（h 为消息来源所在桶号）

// Kadcast Kadcast算法实现
type Kadcast struct {
	hw.BaseAlgorithm                       // 继承基础算法
	NodeIDs          []hw.NodeID128        // 每个节点的 128-bit ID
	KBuckets         []hw.KBucketTable     // 每个节点的 k-bucket 路由表
	Coords           []hw.LatLonCoordinate // 真实坐标（用于 RTT 评估）
	Config           hw.KBucketConfig      // k-bucket 配置
	Visited          [][]bool              // 访问标记 Visited[nodeID][step]
	Rng              *rand.Rand            // 随机数生成器
}

// NewKadcast 创建新的 Kadcast 算法实例
// 参数:
//   - n: 节点数
//   - coords: 节点坐标数组
//   - config: k-bucket 配置参数
//
// 返回: Kadcast 算法实例
func NewKadcast(n int, coords []hw.LatLonCoordinate, config hw.KBucketConfig) *Kadcast {
	kc := &Kadcast{
		BaseAlgorithm: hw.BaseAlgorithm{
			Name:          "kadcast",
			SpecifiedRoot: false,
			Graph:         hw.NewGraph(n),
			Coords:        coords,
			Root:          0,
		},
		NodeIDs:  make([]hw.NodeID128, n),
		KBuckets: make([]hw.KBucketTable, n),
		Coords:   coords,
		Config:   config,
		Visited:  make([][]bool, n),
		Rng:      rand.New(rand.NewSource(42)),
	}

	// 初始化 Visited 数组
	for i := 0; i < n; i++ {
		kc.Visited[i] = make([]bool, hw.MaxDepth)
	}

	fmt.Println("构建 Kadcast 拓扑...")

	// 步骤1：为每个节点生成随机 128-bit NodeID
	fmt.Printf("  步骤1: 生成 %d 个随机 NodeID...\n", n)
	kc.generateNodeIDs(n)

	// 步骤2：预构建所有节点的 k-buckets
	fmt.Printf("  步骤2: 构建 k-bucket 路由表（每桶最多 %d 个节点）...\n", config.K)
	kc.buildKBuckets(n)

	// 统计信息
	kc.printStatistics(n)

	return kc
}

// generateNodeIDs 为每个节点生成随机 128-bit NodeID
func (kc *Kadcast) generateNodeIDs(n int) {
	for i := 0; i < n; i++ {
		kc.NodeIDs[i] = hw.GenerateRandomNodeID()
	}
}

// buildKBuckets 预构建所有节点的 k-buckets
func (kc *Kadcast) buildKBuckets(n int) {
	// 初始化每个节点的 k-bucket 表
	for i := 0; i < n; i++ {
		kc.KBuckets[i].Buckets = make([][]int, kc.Config.NumBits)
		for j := 0; j < kc.Config.NumBits; j++ {
			kc.KBuckets[i].Buckets[j] = make([]int, 0, kc.Config.K)
		}
	}

	// 对每个节点 i，遍历所有其他节点 j
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			// 计算 XOR 距离
			dist := hw.XORDistance(kc.NodeIDs[i], kc.NodeIDs[j])

			// 计算桶索引
			bucketIdx := hw.BucketIndex(dist)
			if bucketIdx < 0 {
				// 距离为 0（不应该发生，因为 NodeID 应该唯一）
				continue
			}

			// 将节点 j 添加到节点 i 的对应桶中（不超过 K 个）
			if len(kc.KBuckets[i].Buckets[bucketIdx]) < kc.Config.K {
				kc.KBuckets[i].Buckets[bucketIdx] = append(kc.KBuckets[i].Buckets[bucketIdx], j)
			}
		}
	}
}

// printStatistics 打印统计信息
func (kc *Kadcast) printStatistics(n int) {
	// 统计每个节点的桶分布
	totalBuckets := 0
	totalPeers := 0
	nonEmptyBuckets := 0

	for i := 0; i < n; i++ {
		for bucketIdx := 0; bucketIdx < kc.Config.NumBits; bucketIdx++ {
			bucketSize := len(kc.KBuckets[i].Buckets[bucketIdx])
			if bucketSize > 0 {
				nonEmptyBuckets++
				totalPeers += bucketSize
			}
		}
		totalBuckets += kc.Config.NumBits
	}

	avgPeersPerNode := float64(totalPeers) / float64(n)
	avgNonEmptyBucketsPerNode := float64(nonEmptyBuckets) / float64(n)

	fmt.Printf("  统计信息:\n")
	fmt.Printf("    平均每节点连接数: %.2f\n", avgPeersPerNode)
	fmt.Printf("    平均每节点非空桶数: %.2f\n", avgNonEmptyBucketsPerNode)
}

// Respond 实现 Algorithm 接口 - 响应消息
// Kadcast 转发策略：
//  1. 计算消息来源所在的桶号 h
//  2. 对桶 i=0..h-1，从每个桶随机选择 F 个节点转发
func (kc *Kadcast) Respond(msg *hw.Message) []int {

	u := msg.Dst
	relayNodes := make([]int, 0)

	// 检查是否已访问过
	if kc.Visited[u][msg.Step] {
		return relayNodes
	}
	//如果是初始消息，则直接转发所有桶的随机F个节点
	if msg.Step == 0 {
		fmt.Println("初始消息，直接转发所有桶的随机F个节点")
		kc.Visited[u][msg.Step] = true
		for i := 0; i < kc.Config.NumBits; i++ {
			bucket := kc.KBuckets[u].Buckets[i]
			selected := kc.randomSelectN(bucket, kc.Config.Fanout)
			for _, peer := range selected {
				if peer != msg.Src {
					relayNodes = append(relayNodes, peer)
				}
			}
		}
		return relayNodes
	}
	kc.Visited[u][msg.Step] = true

	// 计算消息来源所在的桶号 h
	srcDist := hw.XORDistance(kc.NodeIDs[u], kc.NodeIDs[msg.Src])
	h := hw.BucketIndex(srcDist)

	if h < 0 {
		// 消息来源与当前节点 NodeID 相同（不应该发生）
		h = 0
	}

	// 对桶 i=0..h-1 执行转发
	for i := 0; i < h; i++ {
		bucket := kc.KBuckets[u].Buckets[i]
		if len(bucket) == 0 {
			continue
		}

		// 从桶 i 随机选择 F 个节点
		selected := kc.randomSelectN(bucket, kc.Config.Fanout)
		for _, peer := range selected {
			if peer != msg.Src {
				relayNodes = append(relayNodes, peer)
			}
		}
	}

	return relayNodes
}

// randomSelectN 从候选节点中随机选择 n 个
func (kc *Kadcast) randomSelectN(candidates []int, n int) []int {
	if len(candidates) <= n {
		return candidates
	}

	// 使用 Fisher-Yates shuffle 选择 n 个
	selected := make([]int, n)
	indices := kc.Rng.Perm(len(candidates))
	for i := 0; i < n; i++ {
		selected[i] = candidates[indices[i]]
	}

	return selected
}

// SetRoot 实现 Algorithm 接口 - 设置广播根节点
func (kc *Kadcast) SetRoot(root int) {
	kc.Root = root
	// 重置 Visited 标记
	for i := 0; i < len(kc.Visited); i++ {
		for j := 0; j < len(kc.Visited[i]); j++ {
			kc.Visited[i][j] = false
		}
	}
}

// GetAlgoName 实现 Algorithm 接口 - 获取算法名称
func (kc *Kadcast) GetAlgoName() string {
	return fmt.Sprintf("kadcast_k%d_f%d", kc.Config.K, kc.Config.Fanout)
}

// NeedSpecifiedRoot 实现 Algorithm 接口 - 是否需要为每个根重建
func (kc *Kadcast) NeedSpecifiedRoot() bool {
	return false
}

// PrintInfo 打印算法信息（调试用）
func (kc *Kadcast) PrintInfo() {
	fmt.Printf("Kadcast: K=%d, Fanout=%d, NumBits=%d\n",
		kc.Config.K, kc.Config.Fanout, kc.Config.NumBits)
}
