package algorithms

import (
	"fmt"
	"sort"

	hw "gomercator/handlware"
)

// ==================== MERCATOR ADAPTIVE算法 ====================
// MERCATOR ADAPTIVE: 自适应Geohash精度的Mercator变种
// 核心思想:
// 1. 根据节点密度动态调整每个节点的Geohash精度
// 2. 密集区域使用高精度Geohash（细粒度划分）
// 3. 稀疏区域使用低精度Geohash（粗粒度划分）
// 4. 通过迭代细化直到k0桶大小满足阈值要求

// MercatorAdaptive 自适应Mercator算法实现
type MercatorAdaptive struct {
	*Mercator                        // 继承基础Mercator
	NodePrecision []int              // 每个节点的geohash精度
	InitPrecision int                // 初始精度（默认2）
	MaxPrecision  int                // 最大精度（默认6）
	K0Threshold   int                // k0桶阈值（默认50）
	MaxIterations int                // 最大迭代次数（默认10）
	encoder       *hw.GeohashEncoder // Geohash编码器（最大精度）
}

// NewMercatorAdaptive 创建新的自适应Mercator算法实例
// 参数:
//   - n: 节点数
//   - realCoords: 真实坐标（用于延迟计算）
//   - displayCoords: 显示坐标（用于Geohash生成）
//   - root: 广播根节点
//   - initPrec: 初始Geohash精度（默认2）
//   - maxPrec: 最大Geohash精度（默认6）
//   - k0Threshold: k0桶阈值（默认50）
//   - bucketSize: K桶大小
//   - karyFactor: K-ary树分支因子
func NewMercatorAdaptive(n int, realCoords, displayCoords []hw.LatLonCoordinate, root int,
	initPrec, maxPrec, k0Threshold, bucketSize, karyFactor int) *MercatorAdaptive {

	if initPrec <= 0 {
		initPrec = 2
	}
	if maxPrec <= 0 || maxPrec < initPrec {
		maxPrec = 6
	}
	if k0Threshold <= 0 {
		k0Threshold = 50
	}

	// 使用最大精度创建基础Mercator
	baseMercator := NewMercator(n, realCoords, displayCoords, root, maxPrec, bucketSize, 9999, karyFactor)

	ma := &MercatorAdaptive{
		Mercator:      baseMercator,
		NodePrecision: make([]int, n),
		InitPrecision: initPrec,
		MaxPrecision:  maxPrec,
		K0Threshold:   k0Threshold,
		MaxIterations: 10,
		encoder:       hw.NewGeohashEncoder(maxPrec),
	}

	// 初始化所有节点精度为initPrec
	for i := 0; i < n; i++ {
		ma.NodePrecision[i] = initPrec
	}

	// 执行自适应细化
	ma.adaptiveRefine()

	// 使用自适应geohash重新填充K桶
	ma.rebuildKBuckets()

	return ma
}

// adaptiveRefine 自适应细化geohash精度
func (ma *MercatorAdaptive) adaptiveRefine() {
	fmt.Println("开始自适应细化Geohash精度...")

	for iter := 0; iter < ma.MaxIterations; iter++ {
		fmt.Printf("迭代 %d: ", iter+1)

		// 使用当前精度计算geohash
		ma.updateGeohash()

		// 计算当前分组
		groups := ma.computeGroups()

		// 检查是否需要继续细化
		changed := false
		refinedCount := 0

		for prefix, group := range groups {
			if len(group) > ma.K0Threshold {
				// 这个组太大，需要细化
				for _, nodeID := range group {
					if ma.NodePrecision[nodeID] < ma.MaxPrecision {
						ma.NodePrecision[nodeID]++
						changed = true
						refinedCount++
					}
				}
				fmt.Printf("组 '%s' 有 %d 个节点（>%d），细化 %d 个节点; ",
					prefix, len(group), ma.K0Threshold, refinedCount)
			}
		}

		if !changed {
			fmt.Printf("收敛，无需继续细化\n")
			break
		}

		fmt.Printf("共细化 %d 个节点\n", refinedCount)
	}

	// 输出精度分布统计
	ma.printPrecisionStats()
}

// updateGeohash 根据当前精度更新每个节点的geohash
// 方案2：所有节点都存储最大精度的geohash，NodePrecision记录有效精度
func (ma *MercatorAdaptive) updateGeohash() {
	for i := 0; i < len(ma.DisplayCoords); i++ {
		// 所有节点都生成并存储最大精度的geohash
		fullHash := ma.encoder.Encode(ma.DisplayCoords[i].Lat, ma.DisplayCoords[i].Lon)
		ma.NodeGeohash[i] = fullHash
		ma.NodeGeohashBin[i] = hw.ToBinary(fullHash)
	}
}

// computeGroups 计算当前geohash分组
// 按每个节点的有效精度截断后分组
func (ma *MercatorAdaptive) computeGroups() map[string][]int {
	groups := make(map[string][]int)

	for i := 0; i < len(ma.NodeGeohash); i++ {
		prec := ma.NodePrecision[i]
		// 截断到有效精度进行分组
		hash := ma.NodeGeohash[i][:prec]
		groups[hash] = append(groups[hash], i)
	}

	return groups
}

// rebuildKBuckets 使用自适应geohash重建K桶
func (ma *MercatorAdaptive) rebuildKBuckets() {
	fmt.Println("使用自适应Geohash重建K桶...")

	n := len(ma.NodeGeohash)

	// 重新计算TotalBits（使用最大精度）
	ma.TotalBits = ma.MaxPrecision * hw.GeoBitsPerChar

	// 重新初始化K桶
	ma.KBuckets = hw.InitializeKBuckets(n, ma.TotalBits)

	// 重新构建前缀树
	ma.PrefixTree = hw.BuildPrefixTree(ma.NodeGeohash)

	// 重新分组
	ma.GeohashGroups = make(map[string][]int)
	for i := 0; i < n; i++ {
		hash := ma.NodeGeohash[i]
		ma.GeohashGroups[hash] = append(ma.GeohashGroups[hash], i)
	}

	// 填充K0桶（使用前缀匹配）
	fmt.Println("填充K0桶（自适应前缀匹配）...")
	k0Count := ma.fillAdaptiveK0Bucket()
	fmt.Printf("K0桶填充完成，添加%d对连接\n", k0Count)

	// 填充其他K桶
	fmt.Println("填充其他K桶...")
	connections := ma.fillAdaptiveOtherKBuckets()
	fmt.Printf("其他K桶填充完成，添加%d个连接\n", connections)

	// 重建网络连接
	fmt.Println("重建网络连接...")
	edges := 0
	for i := 0; i < n; i++ {
		for bucketIdx := 0; bucketIdx < len(ma.KBuckets[i]); bucketIdx++ {
			for _, neighbor := range ma.KBuckets[i][bucketIdx] {
				if ma.Graph.AddEdge(i, neighbor) {
					edges++
				}
			}
		}
	}
	fmt.Printf("网络连接构建完成，共%d条边\n", edges)
}

// fillAdaptiveK0Bucket 使用自适应前缀匹配填充K0桶
func (ma *MercatorAdaptive) fillAdaptiveK0Bucket() int {
	pairCount := 0
	n := len(ma.NodeGeohash)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			// 判断是否为k0桶关系：一个是另一个的前缀
			if ma.isK0Relation(i, j) {
				ma.KBuckets[i][0] = append(ma.KBuckets[i][0], j)
				pairCount++
			}
		}
	}

	return pairCount
}

// isK0Relation 判断节点i和节点j是否为K0桶关系
// 核心逻辑："在i眼里，世界按i的精度划分"
// - 节点i（精度3，"wx4"）的K0桶：所有前3位为"wx4"的节点
// - 节点i（精度2，"wt"）的K0桶：所有前2位为"wt"的节点
//
// 重要特性：K0关系是对称的
// 原因：自适应细化是按组进行的，同一组内所有节点同时细化到相同精度
// 例如："wt"组被细化时，所有"wt"节点都变成精度3（"wta"、"wtb"等）
// 不可能存在"wt"（精度2）和"wtt"（精度3）同时存在的情况
func (ma *MercatorAdaptive) isK0Relation(i, j int) bool {
	precI := ma.NodePrecision[i]
	precJ := ma.NodePrecision[j]

	// 方法1：使用i的精度判断
	// （由于对称性，也可以用j的精度，结果相同）
	effectivePrec := precI

	// 如果精度不同，它们来自不同组，肯定不是K0关系
	// （理论上这种情况在按组细化时不会导致K0关系）
	if len(ma.NodeGeohash[j]) < effectivePrec {
		return false
	}

	hashI := ma.NodeGeohash[i][:effectivePrec]
	hashJ := ma.NodeGeohash[j][:effectivePrec]

	isK0 := hashI == hashJ

	// 验证对称性（调试用）：如果precI != precJ且isK0，说明逻辑有问题
	if isK0 && precI != precJ {
		// 这种情况理论上不应该发生（按组细化保证了对称性）
		// 如果发生，说明细化逻辑有bug
		// fmt.Printf("警告：K0关系但精度不同！i=%d(prec=%d,%s) j=%d(prec=%d,%s)\n",
		//	i, precI, ma.NodeGeohash[i][:precI], j, precJ, ma.NodeGeohash[j][:precJ])
	}

	return isK0
}

// fillAdaptiveOtherKBuckets 使用自适应逻辑填充其他K桶
// 核心逻辑："在i眼里，世界按i的精度划分"
// 节点i使用自己的精度precI来计算所有其他节点的桶索引
func (ma *MercatorAdaptive) fillAdaptiveOtherKBuckets() int {
	n := len(ma.NodeGeohash)
	connections := 0

	for i := 0; i < n; i++ {
		precI := ma.NodePrecision[i]

		// 节点i的最大桶索引 = precI * 5
		maxBucketI := precI * hw.GeoBitsPerChar

		// 节点i按自己的精度划分世界
		hashI := ma.NodeGeohash[i][:precI]
		binI := hw.ToBinary(hashI)

		// 只遍历有效范围内的桶
		for bucketIdx := 1; bucketIdx <= maxBucketI; bucketIdx++ {
			if len(ma.KBuckets[i][bucketIdx]) >= ma.BucketSize {
				continue
			}

			candidates := make([]hw.PairFloatInt, 0)

			for j := 0; j < n; j++ {
				if i == j {
					continue
				}

				// 跳过k0关系的节点
				if ma.isK0Relation(i, j) {
					continue
				}

				// 关键修正：使用节点i的精度来看节点j
				// 截断j的geohash到i的精度
				if len(ma.NodeGeohash[j]) < precI {
					continue
				}

				hashJ := ma.NodeGeohash[j][:precI]
				binJ := hw.ToBinary(hashJ)

				// 找到首个不同位
				diffPos := hw.FirstDiffBitPos(binI, binJ)
				if diffPos < 0 {
					continue // 应该在K0桶中
				}

				// 计算桶索引（基于节点i的精度）
				totalBits := precI * hw.GeoBitsPerChar
				calcBucketIdx := totalBits - diffPos

				if calcBucketIdx == bucketIdx {
					dist := hw.Distance(ma.Coords[i], ma.Coords[j])
					candidates = append(candidates, hw.PairFloatInt{First: dist, Second: j})
				}
			}

			// 按距离排序，选择最近的
			if len(candidates) > 0 {
				sort.Slice(candidates, func(a, b int) bool {
					return candidates[a].First < candidates[b].First
				})

				for c := 0; c < len(candidates) && len(ma.KBuckets[i][bucketIdx]) < ma.BucketSize; c++ {
					ma.KBuckets[i][bucketIdx] = append(ma.KBuckets[i][bucketIdx], candidates[c].Second)
					connections++
				}
			}
		}

		// 每处理100个节点打印一次进度
		if (i+1)%100 == 0 {
			fmt.Printf("  已处理 %d/%d 个节点...\n", i+1, n)
		}
	}

	return connections
}

// printPrecisionStats 输出精度分布统计
func (ma *MercatorAdaptive) printPrecisionStats() {
	precCount := make(map[int]int)
	for _, prec := range ma.NodePrecision {
		precCount[prec]++
	}

	fmt.Println("\n精度分布统计:")
	for prec := ma.InitPrecision; prec <= ma.MaxPrecision; prec++ {
		count := precCount[prec]
		if count > 0 {
			percentage := float64(count) * 100.0 / float64(len(ma.NodePrecision))
			fmt.Printf("  精度%d: %d个节点 (%.1f%%)\n", prec, count, percentage)
		}
	}
	fmt.Println()
}

// Respond 重写Respond方法，使用自适应精度进行判断
// 核心策略："每个节点按自己的精度看世界"
// - K0桶：flooding所有在自己K0桶中的节点
// - 其他桶：标准Mercator跨区域转发逻辑
func (ma *MercatorAdaptive) Respond(msg *hw.Message) []int {
	u := msg.Dst
	relayNodes := make([]int, 0)

	// 如果已访问过，返回空列表
	if ma.Visited[u][msg.Step] {
		return relayNodes
	}

	ma.Visited[u][msg.Step] = true

	if msg.Step == 0 {
		// ===== 消息源节点 =====

		// 1. Flooding K0桶（所有前precU位与自己相同的节点）
		for _, v := range ma.KBuckets[u][0] {
			if v != msg.Src {
				relayNodes = append(relayNodes, v)
			}
		}

		// 2. 转发所有其他桶（跨区域）
		for bucketIdx := 1; bucketIdx < len(ma.KBuckets[u]); bucketIdx++ {
			for _, v := range ma.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

	} else {
		// ===== 非消息源节点 =====

		// 计算消息源在当前节点眼中的桶索引
		srcBucket := ma.getAdaptiveBucketIndex(u, msg.Src)

		if srcBucket > 0 {
			// 消息来自其他桶，flooding K0桶
			for _, v := range ma.KBuckets[u][0] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		} else {
			// 消息来自K0桶内部，仍然flooding K0桶
			// 原因：K0关系不对称，必须确保覆盖
			for _, v := range ma.KBuckets[u][0] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

		// 转发小于srcBucket的其他桶（标准Mercator逻辑）
		for bucketIdx := 1; bucketIdx < srcBucket; bucketIdx++ {
			for _, v := range ma.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}
	}

	return relayNodes
}

// getAdaptiveBucketIndex 使用自适应精度计算桶索引
// 核心逻辑："在i眼里，j在哪个桶"
// 使用节点i的精度来计算j的桶索引
func (ma *MercatorAdaptive) getAdaptiveBucketIndex(i, j int) int {
	precI := ma.NodePrecision[i]

	// 使用节点i的精度来看节点j
	if len(ma.NodeGeohash[j]) < precI {
		return 0 // j的精度不足，视为K0
	}

	// 截断到节点i的精度
	hashI := ma.NodeGeohash[i][:precI]
	hashJ := ma.NodeGeohash[j][:precI]

	binI := hw.ToBinary(hashI)
	binJ := hw.ToBinary(hashJ)

	// 找到首个不同位
	diffPos := hw.FirstDiffBitPos(binI, binJ)
	if diffPos < 0 {
		return 0 // 完全相同，k0桶
	}

	// 计算桶索引（基于节点i的精度）
	totalBits := precI * hw.GeoBitsPerChar
	return totalBits - diffPos
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (ma *MercatorAdaptive) GetAlgoName() string {
	return "mercator_adaptive"
}

// PrintInfo 打印算法信息
func (ma *MercatorAdaptive) PrintInfo() {
	fmt.Printf("MERCATOR ADAPTIVE: 自适应Geohash精度\n")
	fmt.Printf("  初始精度: %d, 最大精度: %d, K0阈值: %d\n",
		ma.InitPrecision, ma.MaxPrecision, ma.K0Threshold)
	ma.printPrecisionStats()
	ma.Mercator.PrintInfo()
}
