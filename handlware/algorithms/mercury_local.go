package algorithms

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	hw "gomercator/handlware"
)

// ==================== Mercury_Local算法 (本地聚类版本) ====================
// Mercury_Local: 基于本地邻居的分布式聚类广播
// 1. 每个节点随机选择固定数量的邻居
// 2. 每个节点基于自己的邻居进行Vivaldi虚拟坐标测量
// 3. 每个节点对自己的邻居进行K-means聚类
// 4. 基于本地聚类结果构建转发图（优先同簇内邻居）
// 5. 支持EnableNearest选项，用于早期爆发（early burst）
// 6. 根节点有更高的扇出度

// MercuryLocal Mercury本地聚类算法实现
type MercuryLocal struct {
	Graph         *hw.Graph             // 网络图
	GraphNear     *hw.Graph             // 最近邻图（用于early burst）
	Coords        []hw.LatLonCoordinate // 真实坐标
	VivaldiModels []*hw.VivaldiModel    // Vivaldi模型（每个节点基于自己的邻居测量）

	// 本地聚类相关
	LocalClusters     [][][]int // LocalClusters[i][k] 存储节点i的第k个簇中的邻居ID列表
	NeighborList      [][]int   // NeighborList[i] 存储节点i的所有邻居ID
	ClusterID         []int     // ClusterID[i] 存储节点i在自己的局部聚类中属于哪个簇
	NeighborClusterID [][]int   // NeighborClusterID[i][j] 存储节点i的第j个邻居属于哪个局部簇

	TreeRoot      int        // 当前广播树根节点
	RootFanout    int        // 根节点扇出度
	SecondFanout  int        // 第二层扇出度
	Fanout        int        // 普通节点扇出度
	InnerDeg      int        // 簇内连接度
	EnableNearest bool       // 是否启用最近邻策略
	Rng           *rand.Rand // 随机数生成器
	K             int        // 局部聚类K值
}

// NewMercuryLocal 创建新的MercuryLocal算法实例
// 参数:
//   - n: 节点数
//   - coords: 节点坐标数组
//   - root: 广播根节点
//   - neighborCount: 每个节点选择的邻居数量
//   - k: 局部聚类K值
//   - vivaldiRounds: Vivaldi更新轮数
//   - rootFanout, secondFanout, fanout: 扇出度参数
//   - innerDeg: 簇内连接度
//   - enableNearest: 是否启用最近邻策略
func NewMercuryLocal(n int, coords []hw.LatLonCoordinate, root int, neighborCount, k, vivaldiRounds int,
	rootFanout, secondFanout, fanout, innerDeg int, enableNearest bool) *MercuryLocal {

	ml := &MercuryLocal{
		Graph:             hw.NewGraph(n),
		GraphNear:         hw.NewGraph(n),
		Coords:            coords,
		VivaldiModels:     make([]*hw.VivaldiModel, n),
		LocalClusters:     make([][][]int, n),
		NeighborList:      make([][]int, n),
		ClusterID:         make([]int, n),
		NeighborClusterID: make([][]int, n),
		TreeRoot:          root,
		RootFanout:        rootFanout,
		SecondFanout:      secondFanout,
		Fanout:            fanout,
		InnerDeg:          innerDeg,
		EnableNearest:     enableNearest,
		Rng:               rand.New(rand.NewSource(100)),
		K:                 k,
	}

	// 步骤1：选择邻居
	fmt.Printf("步骤1: 为每个节点随机选择邻居（每个节点%d个）...\n", neighborCount)
	ml.selectNeighbors(n, neighborCount)

	// 步骤2：本地Vivaldi测量
	fmt.Printf("步骤2: 每个节点基于自己的邻居进行Vivaldi虚拟坐标测量（%d轮）...\n", vivaldiRounds)
	ml.buildLocalVivaldi(n, vivaldiRounds)

	// 步骤3：本地聚类
	fmt.Printf("步骤3: 每个节点对自己的邻居进行K-means聚类（K=%d）...\n", k)
	ml.buildLocalClusters(n, k)

	// 步骤4：构建拓扑
	fmt.Printf("步骤4: 基于本地聚类结果构建转发图...\n")
	ml.buildTopology(n)

	return ml
}

// selectNeighbors 为每个节点随机选择固定数量的邻居
func (ml *MercuryLocal) selectNeighbors(n int, neighborCount int) {
	for i := 0; i < n; i++ {
		neighbors := make([]int, 0, neighborCount)
		selected := make(map[int]bool)

		for len(neighbors) < neighborCount {
			candidate := ml.Rng.Intn(n)
			if candidate != i && !selected[candidate] {
				neighbors = append(neighbors, candidate)
				selected[candidate] = true
			}
		}

		ml.NeighborList[i] = neighbors
	}

	// 统计信息
	totalNeighbors := 0
	for i := 0; i < n; i++ {
		totalNeighbors += len(ml.NeighborList[i])
	}
	avgNeighbors := float64(totalNeighbors) / float64(n)
	fmt.Printf("  邻居选择完成：平均每个节点 %.2f 个邻居\n", avgNeighbors)
}

// buildLocalVivaldi 每个节点基于自己的邻居进行Vivaldi虚拟坐标测量
func (ml *MercuryLocal) buildLocalVivaldi(n int, rounds int) {
	dim := 3 // Vivaldi维度

	// 初始化所有节点的Vivaldi模型
	for i := 0; i < n; i++ {
		ml.VivaldiModels[i] = hw.NewVivaldiModel(i, dim)
		ml.VivaldiModels[i].LocalCoord.Error = hw.VivaldiInitError

		// 初始化随机坐标
		for d := 0; d < dim; d++ {
			ml.VivaldiModels[i].LocalCoord.Vector[d] = hw.RandomBetween01() * 1000
		}
		ml.VivaldiModels[i].LocalCoord.Height = hw.RandomBetween01() * 100
	}

	// 迭代更新坐标
	for round := 0; round < rounds; round++ {
		if round%10 == 0 && round > 0 {
			fmt.Printf("  Vivaldi轮次 %d/%d\n", round, rounds)
		}

		for x := 0; x < n; x++ {
			neighbors := ml.NeighborList[x]

			// 对每个邻居进行观测和更新
			for _, y := range neighbors {
				// 计算真实RTT（基于地理距离）
				rtt := hw.Distance(ml.Coords[x], ml.Coords[y]) + hw.FixedDelay

				// 观测并更新坐标
				hw.Observe(ml.VivaldiModels[x], y, ml.VivaldiModels[y].LocalCoord, rtt)
			}
		}
	}

	fmt.Printf("  本地Vivaldi测量完成\n")
}

// buildLocalClusters 每个节点对自己的邻居进行K-means聚类
func (ml *MercuryLocal) buildLocalClusters(n int, k int) {
	for nodeID := 0; nodeID < n; nodeID++ {
		neighbors := ml.NeighborList[nodeID]
		neighborNum := len(neighbors)

		if neighborNum == 0 {
			// 没有邻居，跳过
			ml.ClusterID[nodeID] = 0
			ml.LocalClusters[nodeID] = make([][]int, 0)
			ml.NeighborClusterID[nodeID] = make([]int, 0)
			continue
		}

		// 执行本地K-means聚类
		clusterAssignments := ml.kMeansLocal(nodeID, neighbors, k)

		// 存储聚类结果
		ml.NeighborClusterID[nodeID] = clusterAssignments

		// 确定节点nodeID属于哪个簇
		// 策略：节点属于距离自己最近的簇中心所在的簇
		actualK := k
		if neighborNum < k {
			actualK = neighborNum
		}

		// 计算每个簇的中心
		clusterCenters := make([][]float64, actualK)
		clusterCounts := make([]int, actualK)
		dim := len(ml.VivaldiModels[nodeID].LocalCoord.Vector)

		for i := 0; i < actualK; i++ {
			clusterCenters[i] = make([]float64, dim)
		}

		for idx, neighborID := range neighbors {
			c := clusterAssignments[idx]
			vec := ml.VivaldiModels[neighborID].Vector()
			for d := 0; d < dim; d++ {
				clusterCenters[c][d] += vec[d]
			}
			clusterCounts[c]++
		}

		for i := 0; i < actualK; i++ {
			if clusterCounts[i] > 0 {
				for d := 0; d < dim; d++ {
					clusterCenters[i][d] /= float64(clusterCounts[i])
				}
			}
		}

		// 找到节点nodeID最近的簇中心
		minDist := math.MaxFloat64
		bestCluster := 0
		nodeVec := ml.VivaldiModels[nodeID].Vector()
		for i := 0; i < actualK; i++ {
			if clusterCounts[i] > 0 {
				dist := hw.DistanceEuclidean(nodeVec, clusterCenters[i])
				if dist < minDist {
					minDist = dist
					bestCluster = i
				}
			}
		}
		ml.ClusterID[nodeID] = bestCluster

		// 构建LocalClusters：按簇分组邻居
		ml.LocalClusters[nodeID] = make([][]int, actualK)
		for i := 0; i < actualK; i++ {
			ml.LocalClusters[nodeID][i] = make([]int, 0)
		}

		for idx, neighborID := range neighbors {
			c := clusterAssignments[idx]
			ml.LocalClusters[nodeID][c] = append(ml.LocalClusters[nodeID][c], neighborID)
		}
	}

	fmt.Printf("  本地聚类完成\n")
}

// kMeansLocal 对节点nodeID的邻居进行局部K-means聚类
// 返回：clusterAssignments[i] 表示第i个邻居属于哪个簇
func (ml *MercuryLocal) kMeansLocal(nodeID int, neighbors []int, k int) []int {
	neighborNum := len(neighbors)
	if neighborNum == 0 {
		return make([]int, 0)
	}

	dim := len(ml.VivaldiModels[nodeID].LocalCoord.Vector)
	actualK := k
	if neighborNum < k {
		actualK = neighborNum
	}

	// 初始化中心点（从邻居中随机选择actualK个）
	centers := make([][]float64, actualK)
	tmpList := make([]int, 0, actualK)

	for i := 0; i < actualK; i++ {
		for {
			u := ml.Rng.Intn(neighborNum)
			if !hw.Contains(tmpList, u) {
				neighborID := neighbors[u]
				centers[i] = make([]float64, dim)
				copy(centers[i], ml.VivaldiModels[neighborID].Vector())
				tmpList = append(tmpList, u)
				break
			}
		}
	}

	// K-means迭代
	maxIter := 100
	clusterAssignments := make([]int, neighborNum)

	for iter := 0; iter < maxIter; iter++ {
		// 1. 为每个邻居分配最近的中心点
		for idx, neighborID := range neighbors {
			minDist := math.MaxFloat64
			bestCluster := 0

			for j := 0; j < actualK; j++ {
				dist := hw.DistanceEuclidean(centers[j], ml.VivaldiModels[neighborID].Vector())
				if dist < minDist {
					minDist = dist
					bestCluster = j
				}
			}

			clusterAssignments[idx] = bestCluster
		}

		// 2. 重新计算中心点
		avg := make([][]float64, actualK)
		for i := 0; i < actualK; i++ {
			avg[i] = make([]float64, dim)
		}
		counts := make([]int, actualK)

		for idx, neighborID := range neighbors {
			c := clusterAssignments[idx]
			vec := ml.VivaldiModels[neighborID].Vector()
			for d := 0; d < dim; d++ {
				avg[c][d] += vec[d]
			}
			counts[c]++
		}

		// 检查是否收敛（中心点变化很小）
		converged := true
		for i := 0; i < actualK; i++ {
			if counts[i] > 0 {
				for d := 0; d < dim; d++ {
					newCenter := avg[i][d] / float64(counts[i])
					if math.Abs(newCenter-centers[i][d]) > 1e-6 {
						converged = false
					}
					centers[i][d] = newCenter
				}
			}
		}

		if converged {
			break
		}
	}

	return clusterAssignments
}

// buildTopology 基于本地聚类结果构建转发图
func (ml *MercuryLocal) buildTopology(n int) {
	// 为每个节点构建转发连接
	for i := 0; i < n; i++ {
		if len(ml.NeighborList[i]) == 0 {
			continue
		}

		// 检查虚拟坐标误差
		if ml.VivaldiModels[i].LocalCoord.Error >= 0.4 {
			continue
		}

		// 获取节点i所属的局部簇
		myCluster := ml.ClusterID[i]

		// 获取同簇内的邻居和其他簇的邻居
		var sameClusterNeighbors []hw.PairFloatInt
		var otherClusterNeighbors []hw.PairFloatInt

		if myCluster < len(ml.LocalClusters[i]) {
			// 同簇内邻居
			for _, neighborID := range ml.LocalClusters[i][myCluster] {
				dist := hw.DistanceEuclidean(ml.VivaldiModels[i].Vector(), ml.VivaldiModels[neighborID].Vector())
				sameClusterNeighbors = append(sameClusterNeighbors, hw.PairFloatInt{First: dist, Second: neighborID})
			}
		}

		// 其他簇的邻居
		for clusterIdx := 0; clusterIdx < len(ml.LocalClusters[i]); clusterIdx++ {
			if clusterIdx == myCluster {
				continue
			}
			for _, neighborID := range ml.LocalClusters[i][clusterIdx] {
				dist := hw.DistanceEuclidean(ml.VivaldiModels[i].Vector(), ml.VivaldiModels[neighborID].Vector())
				otherClusterNeighbors = append(otherClusterNeighbors, hw.PairFloatInt{First: dist, Second: neighborID})
			}
		}

		// 按距离排序
		sort.Slice(sameClusterNeighbors, func(a, b int) bool {
			return sameClusterNeighbors[a].First < sameClusterNeighbors[b].First
		})
		sort.Slice(otherClusterNeighbors, func(a, b int) bool {
			return otherClusterNeighbors[a].First < otherClusterNeighbors[b].First
		})

		// 优先添加同簇内的邻居（最多InnerDeg个）
		cnt := 0
		for _, peer := range sameClusterNeighbors {
			if cnt >= ml.InnerDeg {
				break
			}
			if ml.Graph.AddEdge(i, peer.Second) {
				cnt++
			}
		}

		// 如果同簇内邻居不足，添加其他簇的邻居
		for _, peer := range otherClusterNeighbors {
			if cnt >= ml.InnerDeg {
				break
			}
			if ml.Graph.AddEdge(i, peer.Second) {
				cnt++
			}
		}

		// 构建最近邻图（用于early burst）
		if ml.EnableNearest {
			allNeighbors := make([]hw.PairFloatInt, 0)
			for _, neighborID := range ml.NeighborList[i] {
				dist := hw.DistanceEuclidean(ml.VivaldiModels[i].Vector(), ml.VivaldiModels[neighborID].Vector())
				allNeighbors = append(allNeighbors, hw.PairFloatInt{First: dist, Second: neighborID})
			}

			// 按距离排序
			sort.Slice(allNeighbors, func(a, b int) bool {
				return allNeighbors[a].First < allNeighbors[b].First
			})

			// 保留最近的InnerDeg个
			for idx := 0; idx < len(allNeighbors) && idx < ml.InnerDeg; idx++ {
				ml.GraphNear.AddEdge(i, allNeighbors[idx].Second)
			}
		}
	}

	// 统计信息
	avgOutbound := 0.0
	for i := 0; i < n; i++ {
		avgOutbound += float64(len(ml.Graph.OutBound[i]))
	}
	avgOutbound /= float64(n)
	fmt.Printf("  拓扑构建完成：平均出度 = %.2f\n", avgOutbound)
}

// Respond 实现Algorithm接口 - 响应消息
// 策略：优先转发给簇内邻居（InnerDeg个），然后转发给簇外邻居（Fanout-InnerDeg个）
func (ml *MercuryLocal) Respond(msg *hw.Message) []int {
	u := msg.Dst
	ret := make([]int, 0)

	// 获取当前节点所属的簇
	myCluster := ml.ClusterID[u]

	// 确定本次转发的总扇出度
	totalFanout := ml.Fanout
	if msg.Step == 0 {
		totalFanout = ml.RootFanout
	} else if msg.Step == 1 {
		totalFanout = ml.SecondFanout
	}

	// 检查是否使用最近邻策略（early burst）
	srcCluster := ml.ClusterID[msg.Src]
	useNearestGraph := ml.EnableNearest && (srcCluster != myCluster || msg.Step == 0 || msg.RecvTime-msg.SendTime > 100)

	if useNearestGraph {
		// 使用最近邻图（不区分簇内簇外）
		for _, v := range ml.GraphNear.OutBound[u] {
			if v != msg.Src {
				ret = append(ret, v)
			}
		}
	} else {
		// 使用本地聚类结果：优先簇内，然后簇外
		// 1. 优先添加簇内邻居（最多InnerDeg个）
		innerCount := 0
		if myCluster < len(ml.LocalClusters[u]) {
			for _, v := range ml.LocalClusters[u][myCluster] {
				if v != msg.Src && innerCount < ml.InnerDeg {
					ret = append(ret, v)
					innerCount++
				}
			}
		}

		// 2. 添加簇外邻居（最多Fanout-InnerDeg个）
		outerCount := 0
		outerLimit := totalFanout - ml.InnerDeg
		if outerLimit < 0 {
			outerLimit = 0
		}

		for clusterIdx := 0; clusterIdx < len(ml.LocalClusters[u]); clusterIdx++ {
			if clusterIdx == myCluster {
				continue // 跳过簇内邻居（已处理）
			}
			for _, v := range ml.LocalClusters[u][clusterIdx] {
				if v != msg.Src && outerCount < outerLimit {
					ret = append(ret, v)
					outerCount++
				}
			}
			if outerCount >= outerLimit {
				break
			}
		}
	}

	// 如果转发数量不足，随机补充
	remainDeg := totalFanout - len(ret)
	for i := 0; i < remainDeg; i++ {
		v := ml.Rng.Intn(ml.Graph.N)
		if u != v && !hw.Contains(ret, v) {
			ret = append(ret, v)
		}
	}

	return ret
}

// SetRoot 实现Algorithm接口 - 设置广播根节点
func (ml *MercuryLocal) SetRoot(root int) {
	ml.TreeRoot = root
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (ml *MercuryLocal) GetAlgoName() string {
	if ml.EnableNearest {
		return "mercury_local_nearest"
	}
	return "mercury_local"
}

// NeedSpecifiedRoot 实现Algorithm接口 - 是否需要为每个根重建
func (ml *MercuryLocal) NeedSpecifiedRoot() bool {
	return false
}

// PrintInfo 打印图信息（调试用）
func (ml *MercuryLocal) PrintInfo() {
	avgOutbound := 0.0
	for i := 0; i < ml.Graph.N; i++ {
		avgOutbound += float64(len(ml.Graph.OutBound[i]))
	}
	avgOutbound /= float64(ml.Graph.N)

	fmt.Printf("Mercury_Local: 平均出度 = %.2f\n", avgOutbound)
	if ml.EnableNearest {
		fmt.Println("  启用最近邻策略（early burst）")
	}
}
