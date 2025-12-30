package handlware

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// ==================== 纯RTT驱动的Vivaldi优化 ====================
// 不使用任何外部信息（如Geohash），只依赖RTT测量

// RTTCache RTT测量缓存（避免重复测量）
type RTTCache struct {
	Cache map[int]float64 // peerID -> 真实RTT
}

// NewRTTCache 创建RTT缓存
func NewRTTCache() *RTTCache {
	return &RTTCache{
		Cache: make(map[int]float64),
	}
}

// GetOrMeasure 获取缓存的RTT，如果没有则测量并缓存
func (rc *RTTCache) GetOrMeasure(peerID int, myCoord, peerCoord LatLonCoordinate) float64 {
	if rtt, exists := rc.Cache[peerID]; exists {
		return rtt
	}

	// 测量并缓存
	rtt := Distance(myCoord, peerCoord) + FixedDelay
	rc.Cache[peerID] = rtt
	return rtt
}

// NeighborPool 动态邻居池
type NeighborPool struct {
	Candidates    []int           // 候选邻居
	PredictErrors map[int]float64 // 每个邻居的平均预测误差
	ObserveCount  map[int]int     // 每个邻居的观测次数
}

// NewNeighborPool 创建邻居池
func NewNeighborPool() *NeighborPool {
	return &NeighborPool{
		Candidates:    make([]int, 0),
		PredictErrors: make(map[int]float64),
		ObserveCount:  make(map[int]int),
	}
}

// UpdateError 更新邻居的预测误差
func (np *NeighborPool) UpdateError(peerID int, error float64) {
	count := np.ObserveCount[peerID]
	if count == 0 {
		np.PredictErrors[peerID] = error
	} else {
		// 指数移动平均
		alpha := 0.3
		np.PredictErrors[peerID] = alpha*error + (1-alpha)*np.PredictErrors[peerID]
	}
	np.ObserveCount[peerID]++
}

// ==================== 优化1：基于真实RTT的分层邻居选择 ====================

// selectNeighborsByRTT 根据真实RTT分层选择邻居
// 策略：近邻（局部精度）+ 中距离 + 远邻（全局精度）
func selectNeighborsByRTT(nodeID int, n int, rttCache *RTTCache, coords []LatLonCoordinate,
	peerSetSize int, round int, totalRounds int) []int {

	// 计算探索率（随轮次递减）
	explorationRate := 1.0 - float64(round)/float64(totalRounds)
	if explorationRate < 0.1 {
		explorationRate = 0.1 // 保持至少10%的探索
	}

	exploreCount := int(float64(peerSetSize) * explorationRate)
	exploitCount := peerSetSize - exploreCount

	selected := make([]int, 0, peerSetSize)

	// 阶段1：利用（Exploit）- 基于已知RTT分层选择
	if len(rttCache.Cache) > 0 {
		// 从缓存中获取所有已测量的邻居
		type rttPair struct {
			peerID int
			rtt    float64
		}

		measured := make([]rttPair, 0)
		for peerID, rtt := range rttCache.Cache {
			if peerID != nodeID {
				measured = append(measured, rttPair{peerID: peerID, rtt: rtt})
			}
		}

		if len(measured) > 0 {
			// 按RTT排序
			sort.Slice(measured, func(i, j int) bool {
				return measured[i].rtt < measured[j].rtt
			})

			// 分层选择：近(1/3) + 中(1/3) + 远(1/3)
			nearCount := exploitCount / 3
			midCount := exploitCount / 3
			farCount := exploitCount - nearCount - midCount

			// 近邻（前1/3）
			for i := 0; i < nearCount && i < len(measured); i++ {
				selected = append(selected, measured[i].peerID)
			}

			// 中距离（中间1/3）
			midStart := len(measured) / 3
			midEnd := len(measured) * 2 / 3
			for i := midStart; i < midEnd && len(selected)-nearCount < midCount; i++ {
				if !containsInt(selected, measured[i].peerID) {
					selected = append(selected, measured[i].peerID)
				}
			}

			// 远距离（后1/3）
			farStart := len(measured) * 2 / 3
			for i := farStart; i < len(measured) && len(selected)-nearCount-midCount < farCount; i++ {
				if !containsInt(selected, measured[i].peerID) {
					selected = append(selected, measured[i].peerID)
				}
			}
		}
	}

	// 阶段2：探索（Explore）- 随机选择新邻居
	for len(selected) < peerSetSize {
		candidate := rand.Intn(n)
		if candidate != nodeID && !containsInt(selected, candidate) {
			selected = append(selected, candidate)
		}
	}

	return selected
}

// ==================== 优化2：错误驱动的邻居选择 ====================

// selectNeighborsByError 根据预测误差动态选择邻居
// 原理：误差大的邻居多观测（提高精度），误差小的少观测（节省资源）
func selectNeighborsByError(nodeID int, n int, pool *NeighborPool, rttCache *RTTCache,
	coords []LatLonCoordinate, peerSetSize int) []int {

	selected := make([]int, 0, peerSetSize)

	if len(pool.PredictErrors) == 0 {
		// 冷启动：随机选择
		for len(selected) < peerSetSize {
			candidate := rand.Intn(n)
			if candidate != nodeID && !containsInt(selected, candidate) {
				selected = append(selected, candidate)
			}
		}
		return selected
	}

	// 按预测误差排序（误差大的优先）
	type errorPair struct {
		peerID int
		error  float64
	}

	pairs := make([]errorPair, 0)
	for peerID, err := range pool.PredictErrors {
		if peerID != nodeID {
			pairs = append(pairs, errorPair{peerID: peerID, error: err})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].error > pairs[j].error // 降序：误差大的在前
	})

	// 策略：70%选误差大的，30%随机探索
	focusCount := int(float64(peerSetSize) * 0.7)

	// 选择误差大的邻居
	for i := 0; i < focusCount && i < len(pairs); i++ {
		selected = append(selected, pairs[i].peerID)
	}

	// 随机探索
	for len(selected) < peerSetSize {
		candidate := rand.Intn(n)
		if candidate != nodeID && !containsInt(selected, candidate) {
			selected = append(selected, candidate)
		}
	}

	return selected
}

// ==================== 优化3：混合策略 ====================

// selectNeighborsHybrid 混合策略：RTT分层 + 误差驱动 + 锚点优先
func selectNeighborsHybrid(nodeID int, n int, anchors []int, pool *NeighborPool,
	rttCache *RTTCache, coords []LatLonCoordinate,
	peerSetSize int, round int, totalRounds int) []int {

	selected := make([]int, 0, peerSetSize)

	// 策略1：优先选择锚点（如果存在）
	anchorCount := peerSetSize / 3
	if len(anchors) > 0 && round >= totalRounds/2 {
		shuffled := make([]int, len(anchors))
		copy(shuffled, anchors)
		rand.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		for _, anchor := range shuffled {
			if anchor != nodeID && len(selected) < anchorCount {
				selected = append(selected, anchor)
			}
		}
	}

	remaining := peerSetSize - len(selected)

	// 策略2：根据阶段选择策略
	progress := float64(round) / float64(totalRounds)

	if progress < 0.3 {
		// 早期（0-30%）：基于RTT分层（快速建立全局拓扑）
		rttNeighbors := selectNeighborsByRTT(nodeID, n, rttCache, coords, remaining, round, totalRounds)
		for _, peer := range rttNeighbors {
			if !containsInt(selected, peer) {
				selected = append(selected, peer)
			}
		}
	} else {
		// 后期（30-100%）：错误驱动（精细优化）
		errorNeighbors := selectNeighborsByError(nodeID, n, pool, rttCache, coords, remaining)
		for _, peer := range errorNeighbors {
			if !containsInt(selected, peer) {
				selected = append(selected, peer)
			}
		}
	}

	return selected
}

// ==================== 优化4：自适应步长增强版 ====================

// computeAdaptiveCcEnhanced 增强版自适应步长
// 新增：基于邻居质量的动态调整
func computeAdaptiveCcEnhanced(currentError float64, peerError float64,
	currentRound int, totalRounds int) float64 {
	baseCc := 0.25

	// 因素1：基于本地误差
	if currentError > 0.5 {
		baseCc = 0.4
	} else if currentError < 0.1 {
		baseCc = 0.15
	}

	// 因素2：基于邻居质量
	// 邻居误差越小，步长可以越大（更信任它的位置）
	peerQuality := 1.0 / (1.0 + peerError)
	baseCc *= (0.5 + 0.5*peerQuality) // 0.5x ~ 1.0x

	// 因素3：后期衰减
	if currentRound > totalRounds/2 {
		progress := float64(currentRound-totalRounds/2) / float64(totalRounds/2)
		decay := 1.0 - progress*0.4 // 最多衰减40%
		baseCc *= decay
	}

	return baseCc
}

// ==================== 优化5：观测质量评分 ====================

// ObservationQuality 评估单次观测的质量
type ObservationQuality struct {
	IsReliable bool    // 是否可靠
	Confidence float64 // 置信度（0-1）
}

// evaluateObservation 评估观测质量
func evaluateObservation(predictedRTT, actualRTT float64, localError, peerError float64) ObservationQuality {
	relativeError := math.Abs(predictedRTT-actualRTT) / actualRTT

	// 计算置信度
	// 因素1：预测误差（越小越好）
	errorFactor := 1.0 / (1.0 + relativeError)

	// 因素2：双方坐标质量
	localQuality := 1.0 / (1.0 + localError)
	peerQuality := 1.0 / (1.0 + peerError)
	qualityFactor := (localQuality + peerQuality) / 2

	confidence := errorFactor * qualityFactor

	// 判断是否可靠（置信度>0.5且相对误差<30%）
	isReliable := confidence > 0.5 && relativeError < 0.3

	return ObservationQuality{
		IsReliable: isReliable,
		Confidence: confidence,
	}
}

// ObserveImproved 改进版观测更新（简化版，去除过度优化）
// 保留有效的优化：信任度加权 + 自适应步长
func ObserveImproved(vm *VivaldiModel, peerID int, peerCoord *VivaldiCoordinate,
	rtt float64, currentRound int, totalRounds int) {

	predictedRTT := DistanceVivaldi(vm.LocalCoord, peerCoord)

	relativeError := math.Abs(predictedRTT-rtt) / rtt
	if rtt < 1e-6 {
		relativeError = 0
	}

	localError := vm.LocalCoord.Error
	peerError := peerCoord.Error

	// 信任度加权（邻居误差越小，权重越大）
	peerTrust := 1.0 / (1.0 + peerError)
	localTrust := 1.0 / (1.0 + localError)
	weight := (localError * peerTrust) / (localError*peerTrust + peerError*localTrust)

	if weight > 1.0 {
		weight = 1.0
	}
	if weight < 0.0 {
		weight = 0.0
	}

	// 更新误差
	vm.LocalCoord.Error = relativeError*VivaldiCe*weight + localError*(1-VivaldiCe*weight)
	if vm.LocalCoord.Error < VivaldiMinError {
		vm.LocalCoord.Error = VivaldiMinError
	}

	// 增强版自适应步长
	adaptiveCc := computeAdaptiveCcEnhanced(localError, peerError, currentRound, totalRounds)
	delta := adaptiveCc * weight
	if delta > 1.0 {
		delta = 1.0
	}

	force := delta * (rtt - predictedRTT)

	// 更新向量
	if predictedRTT > 1e-6 {
		for i := 0; i < len(vm.LocalCoord.Vector); i++ {
			direction := vm.LocalCoord.Vector[i] - peerCoord.Vector[i]
			vm.LocalCoord.Vector[i] += force * direction / predictedRTT
		}
	}

	// 更新高度
	heightDiff := vm.LocalCoord.Height - peerCoord.Height
	if math.Abs(heightDiff) > 1e-6 {
		vm.LocalCoord.Height += force * heightDiff / math.Abs(heightDiff)
	}

	if vm.LocalCoord.Height < 0 {
		vm.LocalCoord.Height = 0
	}
}

// ==================== 纯RTT驱动的虚拟坐标生成 ====================

// GenerateVirtualCoordinatePureRTT 纯RTT驱动的Vivaldi（无Geohash）
// 修复版：移除RTT缓存，简化策略，提高收敛性能
func GenerateVirtualCoordinatePureRTT(coords []LatLonCoordinate, rounds int, dim int) []*VivaldiModel {
	n := len(coords)
	models := make([]*VivaldiModel, n)

	fmt.Printf("开始生成纯RTT驱动的虚拟坐标（%d轮，%d维）...\n", rounds, dim)

	// 初始化
	for i := 0; i < n; i++ {
		models[i] = NewVivaldiModel(i, dim)
		models[i].LocalCoord.Error = VivaldiInitError

		for d := 0; d < dim; d++ {
			models[i].LocalCoord.Vector[d] = RandomBetween01() * 1000
		}
		models[i].LocalCoord.Height = RandomBetween01() * 100

		// 初始化固定邻居集（保证早期收敛效率）
		models[i].RandomPeerSet = make([]int, VivaldiPeerSetSize)
		for j := 0; j < VivaldiPeerSetSize; j++ {
			peer := rand.Intn(n)
			for peer == i {
				peer = rand.Intn(n)
			}
			models[i].RandomPeerSet[j] = peer
		}
		models[i].HaveEnoughPeer = true
	}

	// var anchors []int
	// anchorThreshold := rounds * 3 / 4 // 前75%轮次不使用锚点

	// 迭代更新
	for round := 0; round < rounds; round++ {
		if round%10 == 0 {
			fmt.Printf("  轮次 %d/%d\n", round, rounds)
		}

		// 选择锚点
		// if round == anchorThreshold {
		// 	anchorCount := n / 20
		// 	if anchorCount < 5 {
		// 		anchorCount = 5
		// 	}
		// 	if anchorCount > 50 {
		// 		anchorCount = 50
		// 	}
		// 	anchors = selectLowestErrorNodes(models, anchorCount)
		// 	fmt.Printf("  → 选择了 %d 个锚点（误差最小的节点）\n", len(anchors))
		// }

		for x := 0; x < n; x++ {
			// 跳过锚点
			// if round >= anchorThreshold && containsInt(anchors, x) {
			// 	continue
			// }

			// 使用固定邻居集（简单高效）
			selectedNeighbors := models[x].RandomPeerSet

			// 观测并更新
			for _, y := range selectedNeighbors {
				// 关键修复：每轮都重新"测量"RTT，不缓存！
				// 虽然地理距离不变，但需要用真实距离与当前虚拟坐标距离比较
				rtt := Distance(coords[x], coords[y]) + FixedDelay

				// 使用改进的观测函数（信任度加权 + 自适应步长）
				ObserveImproved(models[x], y, models[y].LocalCoord, rtt, round, rounds)
			}
		}
	}

	// 统计
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

	fmt.Println("\n纯RTT驱动虚拟坐标生成完成！")
	fmt.Printf("平均误差: %.4f\n", avgError)
	fmt.Println("误差分布:")
	fmt.Printf("  <0.1: %d (%.1f%%)\n", errorCount["<0.1"], float64(errorCount["<0.1"])*100/float64(n))
	fmt.Printf("  0.1-0.2: %d (%.1f%%)\n", errorCount["0.1-0.2"], float64(errorCount["0.1-0.2"])*100/float64(n))
	fmt.Printf("  0.2-0.4: %d (%.1f%%)\n", errorCount["0.2-0.4"], float64(errorCount["0.2-0.4"])*100/float64(n))
	fmt.Printf("  0.4-0.6: %d (%.1f%%)\n", errorCount["0.4-0.6"], float64(errorCount["0.4-0.6"])*100/float64(n))
	fmt.Printf("  >=0.6: %d (%.1f%%)\n\n", errorCount[">=0.6"], float64(errorCount[">=0.6"])*100/float64(n))

	return models
}

// containsInt 辅助函数
func containsInt(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
