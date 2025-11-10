package handlware

import (
	"fmt"
	"math"
	"math/rand"
)

// ==================== Vivaldi坐标系统常量 ====================

const (
	// Vivaldi参数
	VivaldiDim         = 3     // 虚拟坐标维度
	VivaldiCc          = 0.25  // 权重常数
	VivaldiCe          = 0.5   // 误差权重
	VivaldiInitError   = 1.0   // 初始误差
	VivaldiMinError    = 0.01  // 最小误差
	VivaldiUpdateRound = 100   // 更新轮数
	VivaldiPeerSetSize = 16    // 每轮选择的邻居数
)

// ==================== Vivaldi坐标更新 ====================

// Observe 根据观测到的RTT更新本地坐标
// 参数:
//   - vm: 本地Vivaldi模型
//   - peerID: 邻居节点ID
//   - peerCoord: 邻居的虚拟坐标
//   - rtt: 观测到的往返时延（ms）
func Observe(vm *VivaldiModel, peerID int, peerCoord *VivaldiCoordinate, rtt float64) {
	// 计算预测的RTT（基于当前虚拟坐标）
	predictedRTT := DistanceVivaldi(vm.LocalCoord, peerCoord)

	// 计算相对误差
	relativeError := math.Abs(predictedRTT-rtt) / rtt
	if rtt < 1e-6 {
		relativeError = 0
	}

	// 更新本地误差估计
	localError := vm.LocalCoord.Error
	peerError := peerCoord.Error

	// 计算权重
	weight := localError / (localError + peerError)
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

	// 计算步长
	delta := VivaldiCc * weight
	if delta > 1.0 {
		delta = 1.0
	}

	// 计算移动方向和距离
	force := delta * (rtt - predictedRTT)

	// 更新向量部分
	if predictedRTT > 1e-6 {
		for i := 0; i < len(vm.LocalCoord.Vector); i++ {
			direction := vm.LocalCoord.Vector[i] - peerCoord.Vector[i]
			vm.LocalCoord.Vector[i] += force * direction / predictedRTT
		}
	}

	// 更新高度部分
	heightDiff := vm.LocalCoord.Height - peerCoord.Height
	if math.Abs(heightDiff) > 1e-6 {
		vm.LocalCoord.Height += force * heightDiff / math.Abs(heightDiff)
	}

	// 确保高度非负
	if vm.LocalCoord.Height < 0 {
		vm.LocalCoord.Height = 0
	}
}

// ==================== 虚拟坐标生成 ====================

// GenerateVirtualCoordinate 基于真实地理坐标生成Vivaldi虚拟坐标
// 参数:
//   - coords: 真实地理坐标数组
//   - rounds: 更新轮数
//   - dim: 虚拟坐标维度
//
// 返回: Vivaldi模型数组
func GenerateVirtualCoordinate(coords []LatLonCoordinate, rounds int, dim int) []*VivaldiModel {
	n := len(coords)
	models := make([]*VivaldiModel, n)

	// 初始化所有节点的Vivaldi模型
	for i := 0; i < n; i++ {
		models[i] = NewVivaldiModel(i, dim)
		models[i].LocalCoord.Error = VivaldiInitError

		// 初始化随机坐标
		for d := 0; d < dim; d++ {
			models[i].LocalCoord.Vector[d] = RandomBetween01() * 1000
		}
		models[i].LocalCoord.Height = RandomBetween01() * 100
	}

	fmt.Printf("开始生成虚拟坐标（%d轮，%d维）...\n", rounds, dim)

	// 迭代更新坐标
	for round := 0; round < rounds; round++ {
		if round%10 == 0 {
			fmt.Printf("  轮次 %d/%d\n", round, rounds)
		}

		for x := 0; x < n; x++ {
			var selectedNeighbors []int

			// 如果有足够的邻居集合，使用它；否则随机选择
			if models[x].HaveEnoughPeer {
				selectedNeighbors = models[x].RandomPeerSet
			} else {
				selectedNeighbors = make([]int, 0, VivaldiPeerSetSize)
				for j := 0; j < VivaldiPeerSetSize; j++ {
					y := rand.Intn(n)
					for y == x {
						y = rand.Intn(n)
					}
					selectedNeighbors = append(selectedNeighbors, y)
				}
			}

			// 对每个邻居进行观测和更新
			for _, y := range selectedNeighbors {
				// 计算真实RTT（基于地理距离）
				rtt := Distance(coords[x], coords[y]) + FixedDelay

				// 观测并更新坐标
				Observe(models[x], y, models[y].LocalCoord, rtt)
			}
		}
	}

	// 统计误差分布
	errorCount := make(map[string]int)
	for i := 0; i < n; i++ {
		err := models[i].LocalCoord.Error
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

	fmt.Println("虚拟坐标生成完成！误差分布:")
	fmt.Printf("  <0.1: %d (%.1f%%)\n", errorCount["<0.1"], float64(errorCount["<0.1"])*100/float64(n))
	fmt.Printf("  0.1-0.2: %d (%.1f%%)\n", errorCount["0.1-0.2"], float64(errorCount["0.1-0.2"])*100/float64(n))
	fmt.Printf("  0.2-0.4: %d (%.1f%%)\n", errorCount["0.2-0.4"], float64(errorCount["0.2-0.4"])*100/float64(n))
	fmt.Printf("  0.4-0.6: %d (%.1f%%)\n", errorCount["0.4-0.6"], float64(errorCount["0.4-0.6"])*100/float64(n))
	fmt.Printf("  >=0.6: %d (%.1f%%)\n", errorCount[">=0.6"], float64(errorCount[">=0.6"])*100/float64(n))

	return models
}

// GenerateRandomVirtualCoordinate 生成随机虚拟坐标（不基于真实RTT）
// 用于测试或快速初始化
func GenerateRandomVirtualCoordinate(n int, dim int) []*VivaldiModel {
	models := make([]*VivaldiModel, n)

	for i := 0; i < n; i++ {
		models[i] = NewVivaldiModel(i, dim)
		models[i].LocalCoord.Error = VivaldiMinError

		// 随机坐标
		for d := 0; d < dim; d++ {
			models[i].LocalCoord.Vector[d] = RandomBetween01() * 1000
		}
		models[i].LocalCoord.Height = RandomBetween01() * 100
	}

	fmt.Printf("生成随机虚拟坐标完成（%d个节点，%d维）\n", n, dim)
	return models
}

// ==================== 坐标质量评估 ====================

// EvaluateCoordinateQuality 评估虚拟坐标的质量
// 通过比较预测距离和真实距离的相关性
func EvaluateCoordinateQuality(models []*VivaldiModel, coords []LatLonCoordinate, sampleSize int) {
	n := len(models)
	if sampleSize > n*n {
		sampleSize = n * n
	}

	fmt.Printf("评估虚拟坐标质量（采样%d对）...\n", sampleSize)

	totalError := 0.0
	maxError := 0.0
	errorDistribution := make([]int, 10) // 0-10%, 10-20%, ..., 90-100%

	for sample := 0; sample < sampleSize; sample++ {
		i := rand.Intn(n)
		j := rand.Intn(n)
		if i == j {
			continue
		}

		// 真实RTT
		realRTT := Distance(coords[i], coords[j]) + FixedDelay

		// 预测RTT（基于虚拟坐标）
		predictedRTT := DistanceVivaldi(models[i].LocalCoord, models[j].LocalCoord)

		// 相对误差
		relativeError := math.Abs(predictedRTT-realRTT) / realRTT
		totalError += relativeError

		if relativeError > maxError {
			maxError = relativeError
		}

		// 误差分布
		bucket := int(relativeError * 10)
		if bucket > 9 {
			bucket = 9
		}
		if bucket >= 0 && bucket < 10 {
			errorDistribution[bucket]++
		}
	}

	avgError := totalError / float64(sampleSize)
	fmt.Printf("平均相对误差: %.2f%%\n", avgError*100)
	fmt.Printf("最大相对误差: %.2f%%\n", maxError*100)
	fmt.Println("误差分布:")
	for i := 0; i < 10; i++ {
		pct := float64(errorDistribution[i]) * 100 / float64(sampleSize)
		fmt.Printf("  %d-%d%%: %d (%.1f%%)\n", i*10, (i+1)*10, errorDistribution[i], pct)
	}
}

// ==================== 辅助函数 ====================

// BuildPeerSet 为每个节点构建邻居集合
// 用于加速Vivaldi收敛
func BuildPeerSet(models []*VivaldiModel, peerSetSize int) {
	n := len(models)

	for i := 0; i < n; i++ {
		models[i].RandomPeerSet = make([]int, peerSetSize)
		for j := 0; j < peerSetSize; j++ {
			peer := rand.Intn(n)
			for peer == i {
				peer = rand.Intn(n)
			}
			models[i].RandomPeerSet[j] = peer
		}
		models[i].HaveEnoughPeer = true
	}

	fmt.Printf("为%d个节点构建邻居集合完成（每个节点%d个邻居）\n", n, peerSetSize)
}

// ExportVirtualCoordinates 导出虚拟坐标到文件（用于调试）
func ExportVirtualCoordinates(filename string, models []*VivaldiModel) error {
	// 这里可以调用io.go中的函数，或者实现新的导出格式
	// 暂时留空，后续可以补充
	fmt.Printf("虚拟坐标导出功能待实现: %s\n", filename)
	return nil
}

