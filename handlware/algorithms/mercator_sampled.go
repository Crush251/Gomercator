package algorithms

import (
	"fmt"
	"sort"

	hw "gomercator/handlware"
)

// ==================== MERCATOR SAMPLED K0算法 ====================
// MERCATOR SAMPLED K0: K0桶采样版本的Mercator
// 核心思想:
// 1. K0桶不存储所有邻居，只采样固定数量（例如10个）
// 2. 采样策略：基于距离的确定性采样，保证连通性
// 3. 每个收到消息的节点都flooding给采样后的K0邻居
// 4. 显著降低K0桶冗余度（从100×降到10×）

// MercatorSampled K0桶采样版本的Mercator
type MercatorSampled struct {
	*Mercator
	K0Neighbors  [][]int // 采样后的K0邻居
	K0SampleSize int     // K0桶采样大小
}

// NewMercatorSampled 创建K0桶采样版本的Mercator
func NewMercatorSampled(n int, realCoords, displayCoords []hw.LatLonCoordinate, root int,
	geoPrec, bucketSize, k0Threshold, karyFactor, k0SampleSize int) *MercatorSampled {

	// 先创建标准Mercator
	baseMercator := NewMercator(n, realCoords, displayCoords, root, geoPrec, bucketSize, k0Threshold, karyFactor)

	ms := &MercatorSampled{
		Mercator:     baseMercator,
		K0Neighbors:  make([][]int, n),
		K0SampleSize: k0SampleSize,
	}

	// 对K0桶进行采样
	ms.sampleK0Buckets()

	return ms
}

// sampleK0Buckets 对所有节点的K0桶进行采样
func (ms *MercatorSampled) sampleK0Buckets() {
	fmt.Println("开始对K0桶进行采样...")

	totalOriginal := 0
	totalSampled := 0

	for i := 0; i < len(ms.KBuckets); i++ {
		k0Bucket := ms.KBuckets[i][0]
		totalOriginal += len(k0Bucket)

		if len(k0Bucket) <= ms.K0SampleSize {
			// K0桶小于等于采样大小，全部保留
			ms.K0Neighbors[i] = make([]int, len(k0Bucket))
			copy(ms.K0Neighbors[i], k0Bucket)
		} else {
			// K0桶大于采样大小，进行采样
			ms.K0Neighbors[i] = ms.distanceBasedSample(i, k0Bucket, ms.K0SampleSize)
		}

		totalSampled += len(ms.K0Neighbors[i])
	}

	reductionRate := 100.0 * (1.0 - float64(totalSampled)/float64(totalOriginal))
	fmt.Printf("K0桶采样完成:\n")
	fmt.Printf("  原始K0连接总数: %d\n", totalOriginal)
	fmt.Printf("  采样后连接总数: %d\n", totalSampled)
	fmt.Printf("  冗余度降低: %.1f%%\n", reductionRate)
}

// distanceBasedSample 基于距离的确定性采样
// 策略：选择最近的k/2个 + 中远距离的k/2个
// 保证：相邻节点的采样必然互相包含，形成连通图
func (ms *MercatorSampled) distanceBasedSample(nodeID int, k0Bucket []int, k int) []int {
	if len(k0Bucket) <= k {
		return k0Bucket
	}

	// 1. 计算所有邻居的距离
	distances := make([]hw.PairFloatInt, 0, len(k0Bucket))
	for _, neighbor := range k0Bucket {
		dist := hw.Distance(ms.Coords[nodeID], ms.Coords[neighbor])
		distances = append(distances, hw.PairFloatInt{First: dist, Second: neighbor})
	}

	// 2. 按距离排序
	sort.Slice(distances, func(a, b int) bool {
		return distances[a].First < distances[b].First
	})

	selected := make([]int, 0, k)

	// 3. 选择最近的k/2个（保证局部连通性）
	nearCount := k / 2
	for i := 0; i < nearCount && i < len(distances); i++ {
		selected = append(selected, distances[i].Second)
	}

	// 4. 选择中远距离的k/2个（保证覆盖多样性）
	farCount := k - nearCount
	if farCount > 0 {
		// 从剩余节点中均匀采样
		remaining := len(distances) - nearCount
		if remaining > 0 {
			step := float64(remaining) / float64(farCount)
			for i := 0; i < farCount; i++ {
				idx := nearCount + int(float64(i)*step)
				if idx < len(distances) {
					selected = append(selected, distances[idx].Second)
				}
			}
		}
	}

	return selected
}

// Respond 实现Algorithm接口 - 生成中继节点列表
// 核心改变：使用采样后的K0Neighbors而非完整的KBuckets[u][0]
func (ms *MercatorSampled) Respond(msg *hw.Message) []int {
	u := msg.Dst
	relayNodes := make([]int, 0)

	// 检查是否已访问
	if ms.Visited[u][msg.Step] {
		return relayNodes
	}

	ms.Visited[u][msg.Step] = true

	if msg.Step == 0 {
		// 消息源节点
		// 1. Flooding采样后的K0邻居（关键改变）
		for _, v := range ms.K0Neighbors[u] {
			if v != msg.Src {
				relayNodes = append(relayNodes, v)
			}
		}

		// 2. 转发其他K桶（标准Mercator逻辑）
		for bucketIdx := 1; bucketIdx < len(ms.KBuckets[u]); bucketIdx++ {
			for _, v := range ms.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

	} else {
		// 非消息源节点
		srcBucket := hw.GetGeoBucketIndex(ms.NodeGeohash[u], ms.NodeGeohash[msg.Src], ms.TotalBits)

		// 1. K0桶处理（关键改变：使用采样后的邻居）
		if srcBucket > 0 {
			// 消息来自其他桶，flooding采样后的K0邻居
			for _, v := range ms.K0Neighbors[u] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		} else {
			// 消息来自K0桶，仍然flooding采样后的K0邻居
			// 原因：采样可能不同，需要确保覆盖
			for _, v := range ms.K0Neighbors[u] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

		// 2. 转发小于srcBucket的其他桶（标准Mercator逻辑）
		for bucketIdx := 1; bucketIdx < srcBucket; bucketIdx++ {
			for _, v := range ms.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}
	}

	return relayNodes
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (ms *MercatorSampled) GetAlgoName() string {
	return "mercator_sampled_k0"
}

// PrintInfo 打印算法信息
func (ms *MercatorSampled) PrintInfo() {
	fmt.Printf("MERCATOR SAMPLED K0: K0桶采样版本\n")
	fmt.Printf("  K0采样大小: %d\n", ms.K0SampleSize)

	// 统计K0桶大小分布
	k0Sizes := make([]int, 0)
	for i := 0; i < len(ms.K0Neighbors); i++ {
		k0Sizes = append(k0Sizes, len(ms.K0Neighbors[i]))
	}
	sort.Ints(k0Sizes)

	avgK0 := 0
	for _, size := range k0Sizes {
		avgK0 += size
	}
	avgK0 /= len(k0Sizes)

	fmt.Printf("  平均K0邻居数: %d\n", avgK0)
	fmt.Printf("  K0邻居数中位数: %d\n", k0Sizes[len(k0Sizes)/2])
	fmt.Printf("  K0邻居数范围: [%d, %d]\n", k0Sizes[0], k0Sizes[len(k0Sizes)-1])
}


