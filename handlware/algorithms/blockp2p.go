package algorithms

import (
	hw "gomercator/handlware"
)

// ==================== BlockP2P算法 ====================
// BlockP2P: 基于地理聚类的P2P广播
// 1. 使用K-means将节点分成K个簇
// 2. 每个簇的第一个节点是入口点（entry point）
// 3. 所有入口点之间全连接
// 4. 簇内节点连接成Chord类型的拓扑（如果簇大小<=8则全连接）

// BlockP2P BlockP2P算法实现
type BlockP2P struct {
	Graph         *hw.Graph              // 网络图
	Coords        []hw.LatLonCoordinate  // 节点坐标
	ClusterResult *hw.ClusterResult      // 聚类结果
	TreeRoot      int                    // 当前广播树根节点（未使用，但保持接口兼容）
	Fanout        int                    // 扇出度参数
}

// NewBlockP2P 创建新的BlockP2P算法实例
// 参数:
//   - n: 节点数
//   - coords: 节点坐标数组
//   - clusterResult: 聚类结果
//   - root: 广播根节点（未使用，保持兼容性）
//   - fanout: 扇出度参数
func NewBlockP2P(n int, coords []hw.LatLonCoordinate, clusterResult *hw.ClusterResult, root int, fanout int) *BlockP2P {
	bp := &BlockP2P{
		Graph:         hw.NewGraph(n),
		Coords:        coords,
		ClusterResult: clusterResult,
		TreeRoot:      root,
		Fanout:        fanout,
	}

	// 构建网络拓扑
	bp.buildTopology()

	return bp
}

// buildTopology 构建BlockP2P网络拓扑
func (bp *BlockP2P) buildTopology() {
	k := bp.ClusterResult.K

	// 1. 连接所有簇的入口点（每个簇的第一个节点）
	entryPoints := make([]int, k)
	for i := 0; i < k; i++ {
		if len(bp.ClusterResult.ClusterList[i]) > 0 {
			entryPoints[i] = bp.ClusterResult.ClusterList[i][0]
		}
	}

	// 入口点之间全连接
	for i := 0; i < k; i++ {
		for j := 0; j < k; j++ {
			if i != j && len(bp.ClusterResult.ClusterList[i]) > 0 && len(bp.ClusterResult.ClusterList[j]) > 0 {
				bp.Graph.AddEdge(entryPoints[i], entryPoints[j])
			}
		}
	}

	// 2. 构建每个簇内的Chord类型拓扑
	for i := 0; i < k; i++ {
		clusterNodes := bp.ClusterResult.ClusterList[i]
		cn := len(clusterNodes)

		if cn <= 1 {
			continue
		}

		if cn <= 8 {
			// 小簇：全连接
			for j := 0; j < cn; j++ {
				u := clusterNodes[j]
				for l := 0; l < cn; l++ {
					v := clusterNodes[l]
					if u != v {
						bp.Graph.AddEdge(u, v)
					}
				}
			}
		} else {
			// 大簇：Chord类型拓扑
			for j := 0; j < cn; j++ {
				u := clusterNodes[j]

				// 连接到距离为2^k的节点
				for k := 1; k < cn; k *= 2 {
					targetIdx := (j + k) % cn
					v := clusterNodes[targetIdx]
					bp.Graph.AddEdge(u, v)
				}

				// 连接到对角节点
				diagonalIdx := (j + cn/2) % cn
				v := clusterNodes[diagonalIdx]
				bp.Graph.AddEdge(u, v)
			}
		}
	}
}

// Respond 实现Algorithm接口 - 响应消息
func (bp *BlockP2P) Respond(msg *hw.Message) []int {
	u := msg.Dst
	nbU := bp.Graph.Outbound(u)
	ret := make([]int, 0, len(nbU))

	// 向所有出边邻居转发（除了消息来源）
	for _, v := range nbU {
		if v != msg.Src {
			ret = append(ret, v)
		}
	}

	return ret
}

// SetRoot 实现Algorithm接口 - 设置广播根节点
func (bp *BlockP2P) SetRoot(root int) {
	bp.TreeRoot = root
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (bp *BlockP2P) GetAlgoName() string {
	return "blockp2p"
}

// NeedSpecifiedRoot 实现Algorithm接口 - 是否需要为每个根重建
func (bp *BlockP2P) NeedSpecifiedRoot() bool {
	return false // BlockP2P不需要为每个根重建图
}

// PrintInfo 打印图信息（调试用）
func (bp *BlockP2P) PrintInfo() {
	avgOutbound := 0.0
	for i := 0; i < bp.Graph.N; i++ {
		avgOutbound += float64(len(bp.Graph.OutBound[i]))
	}
	avgOutbound /= float64(bp.Graph.N)
	
	// 这里可以打印详细信息，暂时简化
	// fmt.Printf("BlockP2P: 平均出度 = %.2f\n", avgOutbound)
	_ = avgOutbound
}

