package algorithms

import (
	"fmt"
	"math/rand"
	"sort"

	hw "gomercator/handlware"
)

// ==================== Mercury算法 (K-means Cluster with Vivaldi) ====================
// Mercury: 基于Vivaldi虚拟坐标的聚类广播
// 1. 使用Vivaldi生成虚拟坐标
// 2. 基于虚拟坐标进行K-means聚类
// 3. 簇内连接：选择最近的InnerDeg个节点
// 4. 支持EnableNearest选项，用于早期爆发（early burst）
// 5. 根节点有更高的扇出度

// Mercury Mercury算法实现
type Mercury struct {
	Graph         *hw.Graph              // 网络图
	GraphNear     *hw.Graph              // 最近邻图（用于early burst）
	Coords        []hw.LatLonCoordinate  // 真实坐标
	VivaldiModels []*hw.VivaldiModel     // Vivaldi模型
	ClusterResult *hw.ClusterResult      // 聚类结果
	TreeRoot      int                    // 当前广播树根节点
	RootFanout    int                    // 根节点扇出度
	SecondFanout  int                    // 第二层扇出度
	Fanout        int                    // 普通节点扇出度
	InnerDeg      int                    // 簇内连接度
	EnableNearest bool                   // 是否启用最近邻策略
	Rng           *rand.Rand             // 随机数生成器
}

// NewMercury 创建新的Mercury算法实例
// 参数:
//   - n: 节点数
//   - coords: 节点坐标数组
//   - vmodels: Vivaldi模型数组
//   - clusterResult: 聚类结果
//   - root: 广播根节点
//   - rootFanout, secondFanout, fanout: 扇出度参数
//   - innerDeg: 簇内连接度
//   - enableNearest: 是否启用最近邻策略
func NewMercury(n int, coords []hw.LatLonCoordinate, vmodels []*hw.VivaldiModel, clusterResult *hw.ClusterResult,
	root int, rootFanout, secondFanout, fanout, innerDeg int, enableNearest bool) *Mercury {

	m := &Mercury{
		Graph:         hw.NewGraph(n),
		GraphNear:     hw.NewGraph(n),
		Coords:        coords,
		VivaldiModels: vmodels,
		ClusterResult: clusterResult,
		TreeRoot:      root,
		RootFanout:    rootFanout,
		SecondFanout:  secondFanout,
		Fanout:        fanout,
		InnerDeg:      innerDeg,
		EnableNearest: enableNearest,
		Rng:           rand.New(rand.NewSource(100)),
	}

	// 构建网络拓扑
	m.buildTopology(n)

	return m
}

// buildTopology 构建Mercury网络拓扑
func (m *Mercury) buildTopology(n int) {
	// 为每个节点构建簇内连接
	for i := 0; i < n; i++ {
		c := m.ClusterResult.ClusterID[i]
		clusterSize := m.ClusterResult.ClusterCnt[c]

		// 检查虚拟坐标误差
		if m.VivaldiModels[i].LocalCoord.Error < 0.4 {
			// 簇内连接：选择最近的InnerDeg个节点
			if clusterSize <= m.InnerDeg+1 {
				// 小簇：全连接
				for _, j := range m.ClusterResult.ClusterList[c] {
					if i != j {
						m.Graph.AddEdge(i, j)
					}
				}
			} else {
				// 大簇：选择最近的InnerDeg个节点
				clusterPeers := make([]hw.PairFloatInt, 0)

				for trial := 0; trial < 100 && len(clusterPeers) < m.InnerDeg; trial++ {
					j := m.ClusterResult.ClusterList[c][rand.Intn(clusterSize)]
					j1 := m.ClusterResult.ClusterList[c][rand.Intn(clusterSize)]

					// 选择更近的节点
					distJ := hw.DistanceEuclidean(m.VivaldiModels[i].Vector(), m.VivaldiModels[j].Vector())
					distJ1 := hw.DistanceEuclidean(m.VivaldiModels[i].Vector(), m.VivaldiModels[j1].Vector())

					if distJ > distJ1 {
						j = j1
						distJ = distJ1
					}

					if i != j {
						clusterPeers = append(clusterPeers, hw.PairFloatInt{First: distJ, Second: j})
					}
				}

				// 按距离排序
				sort.Slice(clusterPeers, func(a, b int) bool {
					return clusterPeers[a].First < clusterPeers[b].First
				})

				// 添加最近的InnerDeg个节点
				cnt := 0
				for _, peer := range clusterPeers {
					if cnt >= m.InnerDeg {
						break
					}
					if m.Graph.AddEdge(i, peer.Second) {
						cnt++
					}
				}
			}

			// 构建最近邻图（用于early burst）
			if m.EnableNearest {
				nearestPeers := make([]hw.PairFloatInt, 0)

				for _, j := range m.ClusterResult.ClusterList[c] {
					if i != j {
						dist := hw.DistanceEuclidean(m.VivaldiModels[i].Vector(), m.VivaldiModels[j].Vector())
						nearestPeers = append(nearestPeers, hw.PairFloatInt{First: dist, Second: j})
					}
				}

				// 按距离排序
				sort.Slice(nearestPeers, func(a, b int) bool {
					return nearestPeers[a].First < nearestPeers[b].First
				})

				// 保留最近的InnerDeg个
				for idx := 0; idx < len(nearestPeers) && idx < m.InnerDeg; idx++ {
					m.GraphNear.AddEdge(i, nearestPeers[idx].Second)
				}
			}
		}
	}
}

// Respond 实现Algorithm接口 - 响应消息
func (m *Mercury) Respond(msg *hw.Message) []int {
	u := msg.Dst
	ret := make([]int, 0)

	// 检查是否使用最近邻策略
	// 条件：
	// 1. EnableNearest = true
	// 2. 跨簇消息 或 消息源节点 或 延迟过高
	if m.EnableNearest && (m.ClusterResult.ClusterID[msg.Src] != m.ClusterResult.ClusterID[u] ||
		msg.Step == 0 || msg.RecvTime-msg.SendTime > 100) {
		// 使用最近邻图
		for _, v := range m.GraphNear.OutBound[u] {
			if v != msg.Src {
				ret = append(ret, v)
			}
		}
	} else {
		// 使用普通图
		for _, v := range m.Graph.OutBound[u] {
			if v != msg.Src {
				ret = append(ret, v)
			}
		}
	}

	// 根据step决定额外转发数量
	remainDeg := 0
	if msg.Step == 0 {
		remainDeg = m.RootFanout - len(ret)
	} else if msg.Step == 1 {
		remainDeg = m.SecondFanout - len(ret)
	} else {
		remainDeg = m.Fanout - len(ret)
	}

	// 添加随机节点
	for i := 0; i < remainDeg; i++ {
		v := m.Rng.Intn(m.Graph.N)
		if u != v && !hw.Contains(ret, v) {
			ret = append(ret, v)
		}
	}

	return ret
}

// SetRoot 实现Algorithm接口 - 设置广播根节点
func (m *Mercury) SetRoot(root int) {
	m.TreeRoot = root
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (m *Mercury) GetAlgoName() string {
	if m.EnableNearest {
		return "mercury_nearest"
	}
	return "mercury"
}

// NeedSpecifiedRoot 实现Algorithm接口 - 是否需要为每个根重建
func (m *Mercury) NeedSpecifiedRoot() bool {
	return false
}

// PrintInfo 打印图信息（调试用）
func (m *Mercury) PrintInfo() {
	avgOutbound := 0.0
	for i := 0; i < m.Graph.N; i++ {
		avgOutbound += float64(len(m.Graph.OutBound[i]))
	}
	avgOutbound /= float64(m.Graph.N)
	
	fmt.Printf("Mercury: 平均出度 = %.2f\n", avgOutbound)
	if m.EnableNearest {
		fmt.Println("  启用最近邻策略（early burst）")
	}
}

