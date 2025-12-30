package handlware

import (
	"fmt"
	"math/rand"
	"sort"
)

// ==================== 改进版Vivaldi坐标系统 ====================

// ObservationBuffer 观测值缓冲区，用于异常值过滤
type ObservationBuffer struct {
	Observations map[int][]float64 // peerID -> RTT列表
	BufferSize   int
}

// NewObservationBuffer 创建观测缓冲区
func NewObservationBuffer(bufferSize int) *ObservationBuffer {
	return &ObservationBuffer{
		Observations: make(map[int][]float64),
		BufferSize:   bufferSize,
	}
}

// AddObservation 添加观测值并返回过滤后的RTT（中位数）
func (buf *ObservationBuffer) AddObservation(peerID int, rtt float64) float64 {
	if _, exists := buf.Observations[peerID]; !exists {
		buf.Observations[peerID] = make([]float64, 0, buf.BufferSize)
	}

	buf.Observations[peerID] = append(buf.Observations[peerID], rtt)

	// 保持缓冲区大小
	if len(buf.Observations[peerID]) > buf.BufferSize {
		buf.Observations[peerID] = buf.Observations[peerID][1:]
	}

	// 返回中位数
	return median(buf.Observations[peerID])
}

// median 计算中位数
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	if len(sorted)%2 == 0 {
		return (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	return sorted[len(sorted)/2]
}

// computeAdaptiveCc 计算自适应步长
// 策略：根据当前误差和轮次动态调整
func computeAdaptiveCc(currentError float64, currentRound int, totalRounds int) float64 {
	baseCc := 0.25

	// 策略1：基于误差的自适应
	if currentError > 0.5 {
		baseCc = 0.4 // 误差大，加速收敛
	} else if currentError < 0.1 {
		baseCc = 0.15 // 误差小，避免振荡
	}

	// 策略2：后期衰减
	if currentRound > totalRounds/2 {
		progress := float64(currentRound-totalRounds/2) / float64(totalRounds/2)
		decay := 1.0 - progress*0.3 // 最多衰减30%
		baseCc *= decay
	}

	return baseCc
}

// // ObserveImproved 改进版坐标更新（信任度加权 + 自适应步长）
// func ObserveImproved(vm *VivaldiModel, peerID int, peerCoord *VivaldiCoordinate,
// 	rtt float64, currentRound int, totalRounds int) {

// 	predictedRTT := DistanceVivaldi(vm.LocalCoord, peerCoord)

// 	relativeError := math.Abs(predictedRTT-rtt) / rtt
// 	if rtt < 1e-6 {
// 		relativeError = 0
// 	}

// 	localError := vm.LocalCoord.Error
// 	peerError := peerCoord.Error

// 	// 改进1：信任度加权
// 	// 邻居误差越小，信任度越高，权重越大
// 	peerTrust := 1.0 / (1.0 + peerError)
// 	localTrust := 1.0 / (1.0 + localError)
// 	weight := (localError * peerTrust) / (localError*peerTrust + peerError*localTrust)

// 	if weight > 1.0 {
// 		weight = 1.0
// 	}
// 	if weight < 0.0 {
// 		weight = 0.0
// 	}

// 	// 更新误差
// 	vm.LocalCoord.Error = relativeError*VivaldiCe*weight + localError*(1-VivaldiCe*weight)
// 	if vm.LocalCoord.Error < VivaldiMinError {
// 		vm.LocalCoord.Error = VivaldiMinError
// 	}

// 	// 改进2：自适应步长
// 	adaptiveCc := computeAdaptiveCc(localError, currentRound, totalRounds)
// 	delta := adaptiveCc * weight
// 	if delta > 1.0 {
// 		delta = 1.0
// 	}

// 	force := delta * (rtt - predictedRTT)

// 	// 更新向量部分
// 	if predictedRTT > 1e-6 {
// 		for i := 0; i < len(vm.LocalCoord.Vector); i++ {
// 			direction := vm.LocalCoord.Vector[i] - peerCoord.Vector[i]
// 			vm.LocalCoord.Vector[i] += force * direction / predictedRTT
// 		}
// 	}

// 	// 更新高度部分
// 	heightDiff := vm.LocalCoord.Height - peerCoord.Height
// 	if math.Abs(heightDiff) > 1e-6 {
// 		vm.LocalCoord.Height += force * heightDiff / math.Abs(heightDiff)
// 	}

// 	if vm.LocalCoord.Height < 0 {
// 		vm.LocalCoord.Height = 0
// 	}
// }

// selectStratifiedNeighbors 分层邻居选择
// 改进3：近邻（局部精度）+ 中距离 + 远距离（全局精度）
func selectStratifiedNeighbors(nodeID int, n int, geohashes []string, peerSetSize int) []int {
	nearCount := peerSetSize / 3
	midCount := peerSetSize / 3
	farCount := peerSetSize - nearCount - midCount

	nodeHash := geohashes[nodeID]
	near := make([]int, 0, nearCount)
	mid := make([]int, 0, midCount)
	far := make([]int, 0, farCount)

	// 近邻：相同Geohash前2位
	for i := 0; i < n && len(near) < nearCount; i++ {
		if i == nodeID {
			continue
		}
		if len(nodeHash) >= 2 && len(geohashes[i]) >= 2 &&
			nodeHash[:2] == geohashes[i][:2] {
			near = append(near, i)
		}
	}

	// 中距离：相同Geohash前1位但第2位不同
	for i := 0; i < n && len(mid) < midCount; i++ {
		if i == nodeID {
			continue
		}
		if len(nodeHash) >= 2 && len(geohashes[i]) >= 2 &&
			nodeHash[0] == geohashes[i][0] && nodeHash[1] != geohashes[i][1] {
			mid = append(mid, i)
		}
	}

	// 远距离：第1位也不同
	for i := 0; i < n && len(far) < farCount; i++ {
		if i == nodeID {
			continue
		}
		if len(nodeHash) >= 1 && len(geohashes[i]) >= 1 &&
			nodeHash[0] != geohashes[i][0] {
			far = append(far, i)
		}
	}

	// 如果某层不够，从随机节点补充
	allSelected := append(near, mid...)
	allSelected = append(allSelected, far...)

	for len(allSelected) < peerSetSize {
		candidate := rand.Intn(n)
		if candidate != nodeID && !contains(allSelected, candidate) {
			allSelected = append(allSelected, candidate)
		}
	}

	// 打乱顺序
	rand.Shuffle(len(allSelected), func(i, j int) {
		allSelected[i], allSelected[j] = allSelected[j], allSelected[i]
	})

	return allSelected[:peerSetSize]
}

// selectNeighborsPreferAnchors 选择邻居（优先选择锚点）
func selectNeighborsPreferAnchors(nodeID int, n int, anchors []int, geohashes []string, peerSetSize int) []int {
	anchorCount := peerSetSize / 2 // 一半选锚点
	regularCount := peerSetSize - anchorCount

	selected := make([]int, 0, peerSetSize)

	// 先选锚点
	shuffledAnchors := make([]int, len(anchors))
	copy(shuffledAnchors, anchors)
	rand.Shuffle(len(shuffledAnchors), func(i, j int) {
		shuffledAnchors[i], shuffledAnchors[j] = shuffledAnchors[j], shuffledAnchors[i]
	})

	for _, anchor := range shuffledAnchors {
		if anchor != nodeID && len(selected) < anchorCount {
			selected = append(selected, anchor)
		}
	}

	// 再选常规节点（分层）
	regularNodes := selectStratifiedNeighbors(nodeID, n, geohashes, regularCount+len(anchors))
	for _, node := range regularNodes {
		if node != nodeID && !contains(selected, node) && !contains(anchors, node) {
			selected = append(selected, node)
			if len(selected) >= peerSetSize {
				break
			}
		}
	}

	return selected
}

// selectLowestErrorNodes 选择误差最小的节点作为锚点
func selectLowestErrorNodes(models []*VivaldiModel, count int) []int {
	type errorPair struct {
		nodeID int
		error  float64
	}

	pairs := make([]errorPair, len(models))
	for i, model := range models {
		pairs[i] = errorPair{nodeID: i, error: model.LocalCoord.Error}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].error < pairs[j].error
	})

	anchors := make([]int, count)
	for i := 0; i < count; i++ {
		anchors[i] = pairs[i].nodeID
	}

	return anchors
}

// contains 辅助函数：检查切片是否包含元素
func contains(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// ==================== 改进版虚拟坐标生成 ====================

// GenerateVirtualCoordinateImproved 改进版Vivaldi坐标生成
// 集成所有改进：分层邻居选择、自适应步长、信任度加权、全局锚点、异常值过滤
func GenerateVirtualCoordinateImproved(coords []LatLonCoordinate, rounds int, dim int) []*VivaldiModel {
	n := len(coords)
	models := make([]*VivaldiModel, n)
	observationBuffers := make([]*ObservationBuffer, n)
	geohashes := make([]string, n)

	fmt.Printf("开始生成改进版虚拟坐标（%d轮，%d维）...\n", rounds, dim)

	// 初始化
	encoder := NewGeohashEncoder(3) // 3位Geohash用于邻居分层
	for i := 0; i < n; i++ {
		models[i] = NewVivaldiModel(i, dim)
		models[i].LocalCoord.Error = VivaldiInitError

		// 随机初始化坐标
		for d := 0; d < dim; d++ {
			models[i].LocalCoord.Vector[d] = RandomBetween01() * 1000
		}
		models[i].LocalCoord.Height = RandomBetween01() * 100

		// 创建观测缓冲区（用于异常值过滤）
		observationBuffers[i] = NewObservationBuffer(5)

		// 计算Geohash（用于邻居选择）
		geohashes[i] = encoder.Encode(coords[i].Lat, coords[i].Lon)
	}

	var anchors []int
	anchorThreshold := rounds / 2 * 3 // 前75%轮次不使用锚点

	// 迭代更新坐标
	for round := 0; round < rounds; round++ {
		if round%10 == 0 {
			fmt.Printf("  轮次 %d/%d\n", round, rounds)
		}

		// 在第20%轮次选择锚点
		if round == anchorThreshold {
			anchorCount := n / 20
			if anchorCount < 5 {
				anchorCount = 5
			}
			if anchorCount > 50 {
				anchorCount = 50
			}
			anchors = selectLowestErrorNodes(models, anchorCount)
			fmt.Printf("  → 选择了 %d 个锚点（误差最小的节点）\n", len(anchors))
		}

		for x := 0; x < n; x++ {
			// 锚点不更新（保持坐标系稳定）
			if round >= anchorThreshold && contains(anchors, x) {
				continue
			}

			var selectedNeighbors []int

			// 选择邻居策略
			if round < anchorThreshold {
				// 前期：分层邻居选择
				selectedNeighbors = selectStratifiedNeighbors(x, n, geohashes, VivaldiPeerSetSize)
			} else {
				// 后期：优先选择锚点
				selectedNeighbors = selectNeighborsPreferAnchors(x, n, anchors, geohashes, VivaldiPeerSetSize)
			}

			// 对每个邻居进行观测和更新
			for _, y := range selectedNeighbors {
				// 计算真实RTT
				rtt := Distance(coords[x], coords[y]) + FixedDelay

				// 异常值过滤：使用缓冲区中位数
				filteredRTT := observationBuffers[x].AddObservation(y, rtt)

				// 改进版观测更新（信任度加权 + 自适应步长）
				ObserveImproved(models[x], y, models[y].LocalCoord, filteredRTT, round, rounds)
			}
		}
	}

	// 统计误差分布
	errorCount := make(map[string]int)
	totalError := 0.0
	for i := 0; i < n; i++ {
		err := models[i].LocalCoord.Error
		totalError += err

		if err < 0.1 {
			errorCount["<0.1"]++
		} else if err < 0.2 {
			errorCount["0.1-0.2"]++
		} else if err < 0.4 {
			errorCount["0.2-0.4"]++
		} else if err < 0.6 {
			errorCount["0.4-0.6"]++
		} else {
			errorCount[">=0.6"]++
		}
	}

	avgError := totalError / float64(n)

	fmt.Println("\n改进版虚拟坐标生成完成！")
	fmt.Printf("平均误差: %.4f\n", avgError)
	fmt.Println("误差分布:")
	fmt.Printf("  <0.1: %d (%.1f%%)\n", errorCount["<0.1"], float64(errorCount["<0.1"])*100/float64(n))
	fmt.Printf("  0.1-0.2: %d (%.1f%%)\n", errorCount["0.1-0.2"], float64(errorCount["0.1-0.2"])*100/float64(n))
	fmt.Printf("  0.2-0.4: %d (%.1f%%)\n", errorCount["0.2-0.4"], float64(errorCount["0.2-0.4"])*100/float64(n))
	fmt.Printf("  0.4-0.6: %d (%.1f%%)\n", errorCount["0.4-0.6"], float64(errorCount["0.4-0.6"])*100/float64(n))
	fmt.Printf("  >=0.6: %d (%.1f%%)\n\n", errorCount[">=0.6"], float64(errorCount[">=0.6"])*100/float64(n))

	return models
}







