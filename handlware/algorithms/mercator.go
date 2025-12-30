package algorithms

import (
	"fmt"
	"sort"
	"strings"

	hw "gomercator/handlware"
)

// ==================== MERCATOR算法 ====================
// MERCATOR: 基于Geohash的地理感知广播算法
// 核心思想:
// 1. 使用Geohash编码节点位置
// 2. 构建K桶结构(类似Kademlia)
// 3. K0桶：相同Geohash的节点（可能使用K-ary树）
// 4. K1-Kn桶：Geohash二进制表示不同位的节点
// 5. 消息转发策略：从近到远逐层扩散
// 6. 支持谎报坐标攻击场景

// Mercator Mercator算法实现
type Mercator struct {
	Graph          *hw.Graph             // 网络拓扑图
	Coords         []hw.LatLonCoordinate // 真实坐标（用于计算延迟）
	DisplayCoords  []hw.LatLonCoordinate // 显示坐标（可能是伪造的）
	NodeGeohash    []string              // 每个节点的Geohash
	NodeGeohashBin []string              // Geohash的二进制表示
	KBuckets       [][][]int             // K桶 [节点][桶ID][节点列表]
	GeohashGroups  map[string][]int      // Geohash分组
	PrefixTree     *hw.GeoPrefixNode     // 前缀树
	TreeRoot       int                   // 当前广播树根节点
	Visited        [][]bool              // 访问标记 [节点][Step]
	GeoPrec        int                   // Geohash精度
	BucketSize     int                   // K桶大小
	K0Threshold    int                   // K0桶阈值（超过则用K-ary树）
	KaryFactor     int                   // K-ary树分支因子
	TotalBits      int                   // Geohash总位数
	KaryMsgInfo    []*hw.KaryMessage     // K-ary消息信息
}

// NewMercator 创建新的Mercator算法实例
// 参数:
//   - n: 节点数
//   - realCoords: 真实坐标（用于延迟计算）
//   - displayCoords: 显示坐标（用于Geohash生成，可能被伪造）
//   - root: 广播根节点
//   - geoPrec: Geohash精度
//   - bucketSize: K桶大小
//   - k0Threshold: K0桶阈值
//   - karyFactor: K-ary树分支因子
func NewMercator(n int, realCoords, displayCoords []hw.LatLonCoordinate, root int,
	geoPrec, bucketSize, k0Threshold, karyFactor int) *Mercator {

	totalBits := geoPrec * hw.GeoBitsPerChar

	m := &Mercator{
		Graph:          hw.NewGraph(n),
		Coords:         realCoords,
		DisplayCoords:  displayCoords,
		NodeGeohash:    make([]string, n),
		NodeGeohashBin: make([]string, n),
		GeohashGroups:  make(map[string][]int),
		TreeRoot:       root,
		Visited:        make([][]bool, n),
		GeoPrec:        geoPrec,
		BucketSize:     bucketSize,
		K0Threshold:    k0Threshold,
		KaryFactor:     karyFactor,
		TotalBits:      totalBits,
		KaryMsgInfo:    make([]*hw.KaryMessage, n),
	}

	// 初始化访问标记
	for i := 0; i < n; i++ {
		m.Visited[i] = make([]bool, hw.MaxDepth+1)
		m.KaryMsgInfo[i] = &hw.KaryMessage{RootNode: -1, IsKary: false}
	}

	// 填充K桶并构建网络
	m.fillKBuckets(n)

	return m
}

// fillKBuckets 填充K桶并构建网络连接
func (m *Mercator) fillKBuckets(n int) {
	fmt.Println("正在生成Geohash...")

	encoder := hw.NewGeohashEncoder(m.GeoPrec)

	// 1. 生成Geohash
	for i := 0; i < n; i++ {
		// 使用显示坐标生成Geohash（可能是伪造的）
		m.NodeGeohash[i] = encoder.Encode(m.DisplayCoords[i].Lat, m.DisplayCoords[i].Lon)
		m.NodeGeohashBin[i] = hw.ToBinary(m.NodeGeohash[i])
		m.GeohashGroups[m.NodeGeohash[i]] = append(m.GeohashGroups[m.NodeGeohash[i]], i)
	}

	fmt.Printf("为%d个节点生成Geohash完成\n", n)

	// 2. 初始化K桶,K桶结构 [节点][桶ID][节点列表]
	m.KBuckets = hw.InitializeKBuckets(n, m.TotalBits)

	// 3. 构建前缀树
	m.PrefixTree = hw.BuildPrefixTree(m.NodeGeohash)

	// 4. 填充K0桶
	fmt.Println("填充K0桶...")
	pairCount := hw.FillK0Bucket(m.KBuckets, m.GeohashGroups)
	fmt.Printf("K0桶填充完成，添加%d对连接\n", pairCount)

	// 5. 填充其他K桶
	fmt.Println("填充其他K桶...")
	connections := hw.FillOtherKBuckets(m.KBuckets, m.NodeGeohashBin, m.Coords, m.BucketSize, m.TotalBits)
	fmt.Printf("其他K桶填充完成，添加%d个连接\n", connections)
	// 5.1 锚点补齐：确保每个字符位的5个桶里能找到 XOR=5/10/15 的邻居（每类至少1个）
	// fmt.Println("补齐XOR锚点...")
	// xorRecords := m.EnsureXorAnchors(1) // 每类1个
	// fmt.Printf("XOR锚点补齐完成，共添加%d个锚点\n", len(xorRecords))

	// // 保存XOR锚点记录到CSV文件
	// if len(xorRecords) > 0 {
	// 	err := hw.WriteXorAnchorRecords("xor_anchors.csv", xorRecords)
	// 	if err != nil {
	// 		fmt.Printf("警告：保存XOR锚点记录失败: %v\n", err)
	// 	}
	// }

	// 6. 构建网络连接
	fmt.Println("构建网络连接...")
	edges := 0
	for i := 0; i < n; i++ {
		for bucketIdx := 0; bucketIdx < len(m.KBuckets[i]); bucketIdx++ {
			for _, neighbor := range m.KBuckets[i][bucketIdx] {
				if m.Graph.AddEdge(i, neighbor) {
					edges++
				}
			}
		}
	}
	fmt.Printf("网络连接构建完成，共%d条边\n", edges)
}

// ResetVisited 重置访问标记（在新的广播开始前调用）
func (m *Mercator) ResetVisited() {
	for i := 0; i < len(m.Visited); i++ {
		for j := 0; j < len(m.Visited[i]); j++ {
			m.Visited[i][j] = false
		}
	}

	// 重置K-ary消息信息
	for i := 0; i < len(m.KaryMsgInfo); i++ {
		m.KaryMsgInfo[i].RootNode = -1
		m.KaryMsgInfo[i].IsKary = false
	}
}

// Respond 实现Algorithm接口 - 响应消息
func (m *Mercator) Respond2(msg *hw.Message) []int {
	u := msg.Dst
	relayNodes := make([]int, 0)

	// 如果已访问过，返回空列表
	if m.Visited[u][msg.Step] {
		return relayNodes
	}

	m.Visited[u][msg.Step] = true

	//先把k0桶的节点添加进relayNodes
	for _, v := range m.KBuckets[u][0] {
		if v != msg.Src {
			relayNodes = append(relayNodes, v)
		}
	}

	// 策略：K0桶flooding + 跨区域转发
	if msg.Step == 0 {
		// 消息源节点：转发所有其他桶
		for bucketIdx := 1; bucketIdx < len(m.KBuckets[u]); bucketIdx++ {
			for _, v := range m.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

	} else {
		// 非消息源节点
		// 获取消息源所在的桶号
		srcBucket := hw.GetGeoBucketIndex(m.NodeGeohash[u], m.NodeGeohash[msg.Src], m.TotalBits)

		// 从小于srcBucket的桶中选择节点转发
		for bucketIdx := 1; bucketIdx < srcBucket; bucketIdx++ {
			for _, v := range m.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}
	}

	return relayNodes
}

// Respond 实现Algorithm接口 - 响应消息
func (m *Mercator) Respond(msg *hw.Message) []int {
	u := msg.Dst
	relayNodes := make([]int, 0)

	// 如果已访问过，返回空列表
	if m.Visited[u][msg.Step] {
		return relayNodes
	}

	m.Visited[u][msg.Step] = true

	// 策略1：先K0桶，然后由近到远
	if msg.Step == 0 {
		// 消息源节点
		// 第一步：处理K0桶（相同geohash的节点）
		if len(m.KBuckets[u][0]) <= m.K0Threshold {
			// K0桶节点数量少，直接flooding
			for _, v := range m.KBuckets[u][0] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		} else {
			// K0桶节点数量多，使用k-ary树
			sameGeohashNodes := m.GeohashGroups[m.NodeGeohash[u]]
			sort.Ints(sameGeohashNodes)

			// 找到u在列表中的位置
			uIdx := -1
			for idx, node := range sameGeohashNodes {
				if node == u {
					uIdx = idx
					break
				}
			}

			if uIdx != -1 {
				// 计算k-ary树的子节点
				children := hw.ComputeKaryChildren(uIdx, len(sameGeohashNodes), m.KaryFactor)
				for _, childIdx := range children {
					if childIdx < len(sameGeohashNodes) {
						v := sameGeohashNodes[childIdx]
						if v != msg.Src {
							relayNodes = append(relayNodes, v)
							m.KaryMsgInfo[v].RootNode = u
							m.KaryMsgInfo[v].IsKary = true
						}
					}
				}
			}
			// // >>> 新增：字符级 XOR 触发的额外转发
			// picked := make(map[int]struct{}, len(relayNodes)+4)
			// for _, v := range relayNodes {
			// 	picked[v] = struct{}{}
			// }
			// extra := m.extraForwardByCharXOR(u, msg.Src, picked)
			// // fmt.Println("extra:", extra)
			// // fmt.Scanln()
			// if len(extra) > 0 {
			// 	relayNodes = append(relayNodes, extra...)
			// }
		}

		// 第二步：从每个非K0桶中选择节点进行传播
		for bucketIdx := 1; bucketIdx < len(m.KBuckets[u]); bucketIdx++ {
			for _, v := range m.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

	} else {
		// 非消息源节点
		// 获取消息源所在的桶号
		srcBucket := hw.GetGeoBucketIndex(m.NodeGeohash[u], m.NodeGeohash[msg.Src], m.TotalBits)

		// 首先检查是否是k-ary树传播
		if m.KaryMsgInfo[u].IsKary {
			karyRoot := m.KaryMsgInfo[u].RootNode
			sameGeohashNodes := m.GeohashGroups[m.NodeGeohash[karyRoot]]
			sort.Ints(sameGeohashNodes)

			// 找到u在列表中的位置
			uIdx := -1
			for idx, node := range sameGeohashNodes {
				if node == u {
					uIdx = idx
					break
				}
			}

			if uIdx != -1 {
				// 计算k-ary树的子节点
				children := hw.ComputeKaryChildren(uIdx, len(sameGeohashNodes), m.KaryFactor)
				for _, childIdx := range children {
					if childIdx < len(sameGeohashNodes) {
						v := sameGeohashNodes[childIdx]
						if v != msg.Src {
							relayNodes = append(relayNodes, v)
							m.KaryMsgInfo[v].RootNode = karyRoot
							m.KaryMsgInfo[v].IsKary = true
						}
					}
				}
			}
		} else {
			if srcBucket > 0 {
				// 常规传播：处理k0桶
				if len(m.KBuckets[u][0]) <= m.K0Threshold {
					// K0桶flooding
					for _, v := range m.KBuckets[u][0] {
						if v != msg.Src {
							relayNodes = append(relayNodes, v)
						}
					}
				} else {
					// K0桶k-ary树
					sameGeohashNodes := m.GeohashGroups[m.NodeGeohash[u]]
					sort.Ints(sameGeohashNodes)

					uIdx := -1
					for idx, node := range sameGeohashNodes {
						if node == u {
							uIdx = idx
							break
						}
					}

					if uIdx != -1 {
						children := hw.ComputeKaryChildren(uIdx, len(sameGeohashNodes), m.KaryFactor)
						for _, childIdx := range children {
							if childIdx < len(sameGeohashNodes) {
								v := sameGeohashNodes[childIdx]
								if v != msg.Src {
									relayNodes = append(relayNodes, v)
									m.KaryMsgInfo[v].RootNode = u
									m.KaryMsgInfo[v].IsKary = true
								}
							}
						}
					}
				}
			}
		}

		// 从小于srcBucket的桶中选择节点转发
		for bucketIdx := 1; bucketIdx < srcBucket; bucketIdx++ {
			for _, v := range m.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}
		// // >>> 新增：字符级 XOR 触发的额外转发
		// picked := make(map[int]struct{}, len(relayNodes)+4)
		// for _, v := range relayNodes {
		// 	picked[v] = struct{}{}
		// }
		// extra := m.extraForwardByCharXOR(u, msg.Src, picked)
		// // fmt.Println("extra:", extra)
		// // fmt.Scanln()
		// if len(extra) > 0 {
		// 	relayNodes = append(relayNodes, extra...)
		// }
	}

	return relayNodes
}

// firstDiffCharIndex 找到首个不同字符的索引
func firstDiffCharIndex(a, b string) int {
	min := len(a)
	if len(b) < min {
		min = len(b)
	}
	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return -1
}

// extraForwardByCharXOR 依据“字符级 XOR 规则”生成额外转发目标
func (m *Mercator) extraForwardByCharXOR(u, sender int, already map[int]struct{}) []int {
	out := make([]int, 0)
	ghu := m.NodeGeohash[u]
	ghs := m.NodeGeohash[sender]
	if ghu == "" || ghs == "" {
		return out
	}

	i := firstDiffCharIndex(ghu, ghs)
	if i < 0 || i >= m.GeoPrec {
		return out
	}

	ui := base32IndexByte(ghu[i])
	si := base32IndexByte(ghs[i])
	if ui < 0 || si < 0 {
		return out
	}

	x := ui ^ si

	targets := make(map[int]struct{})
	if x == 2 || x == 8 || x == 10 {
		targets[5] = struct{}{}
	}
	if x == 1 || x == 4 || x == 10 {
		targets[10] = struct{}{}
	}
	if len(targets) == 0 {
		return out
	}

	// 先从 i 对应的5个桶里找
	start := i*5 + 1
	end := start + 4
	if end > m.TotalBits {
		end = m.TotalBits
	}

	addOne := func(v int) {
		if v == u || v == sender {
			return
		}
		if _, ok := already[v]; ok {
			return
		}
		already[v] = struct{}{}
		out = append(out, v)
	}

	for tgt := range targets {
		want := ui ^ tgt // b = a XOR x
		// 在 5 个桶内查
		found := 0
		for b := start; b <= end; b++ {
			for _, v := range m.KBuckets[u][b] {
				ghv := m.NodeGeohash[v]
				if len(ghv) <= i {
					continue
				}
				vi := base32IndexByte(ghv[i])
				if vi >= 0 && vi == want {
					addOne(v)
					found++
				}
			}
		}
		if found > 0 {
			continue
		}

		// 桶里没有 → 前缀树回退（同前缀 + 该字符等于 want）
		prefix := ghu[:i]
		cands := hw.FindNodesWithPrefix(m.PrefixTree, prefix)
		filtered := make([]int, 0, len(cands))
		for _, v := range cands {
			if v == u || v == sender {
				continue
			}
			ghv := m.NodeGeohash[v]
			if len(ghv) <= i {
				continue
			}
			vi := base32IndexByte(ghv[i])
			if vi >= 0 && vi == want {
				filtered = append(filtered, v)
			}
		}
		// 近邻优先
		sort.Slice(filtered, func(a, b int) bool {
			return hw.Distance(m.Coords[u], m.Coords[filtered[a]]) <
				hw.Distance(m.Coords[u], m.Coords[filtered[b]])
		})
		// 追加若干（不做严格上限，这里取前 m.BucketSize，避免爆扇出）
		limit := m.BucketSize
		if limit <= 0 || limit > len(filtered) {
			limit = len(filtered)
		}
		for i := 0; i < limit; i++ {
			addOne(filtered[i])
		}
	}

	return out
}
func base32IndexByte(b byte) int {
	return strings.IndexByte(hw.Base32Charset, b) // -1 表示不在 Base32
}

// ensureCharXorAnchorsForNode 对单节点 u 的所有字符位，补齐 XOR=5/10/15 的锚点
// 新策略：在对应字符位的桶中查找XOR=5/10/15的节点，找到后通过异或计算放入相应桶
// 返回值：添加的锚点记录列表
func (m *Mercator) ensureCharXorAnchorsForNode(u int, ensurePerTarget int) []hw.XorAnchorRecord {
	records := make([]hw.XorAnchorRecord, 0)
	ghu := m.NodeGeohash[u]
	if ghu == "" {
		return records
	}
	uBin := hw.ToBinary(ghu)

	// 对每个字符位进行处理（c从0开始，对应第c+1个字符）
	for c := 0; c < m.GeoPrec; c++ {
		if len(ghu) <= c {
			break
		}
		ui := base32IndexByte(ghu[c])
		if ui < 0 {
			continue
		}

		// 计算该字符位对应的桶范围
		// 关键理解：bucket = TotalBits - diffPos
		// 字符c（0-based）对应bit范围：c*5 到 (c+1)*5-1
		// 例如：TotalBits=20时
		//   字符0（bit 0-4）  → bucket 20,19,18,17,16
		//   字符1（bit 5-9）  → bucket 15,14,13,12,11
		//   字符2（bit 10-14） → bucket 10,9,8,7,6
		//   字符3（bit 15-19） → bucket 5,4,3,2,1
		start := m.TotalBits - (c+1)*5 + 1 // TotalBits - (c+1)*5 + 1
		end := m.TotalBits - c*5           // TotalBits - c*5
		if start < 1 {
			start = 1 // 桶索引从1开始
		}
		if end > m.TotalBits {
			end = m.TotalBits
		}
		if start > end {
			continue // 范围无效
		}

		// 对每个XOR值（5/10/15），检查是否已有，没有就补充
		for _, x := range []int{5, 10, 15} {
			// 先检查该字符位对应的桶中是否已有XOR=x的节点
			hasXor := false
			for b := start; b <= end; b++ {
				for _, v := range m.KBuckets[u][b] {
					if v == u {
						continue
					}
					ghv := m.NodeGeohash[v]
					if len(ghv) <= c {
						continue
					}
					vi := base32IndexByte(ghv[c])
					// 检查第c个字符位置上是否满足XOR=x
					if vi >= 0 && (ui^vi) == x {
						hasXor = true
						break
					}
				}
				if hasXor {
					break
				}
			}

			// 如果已有XOR=x的节点，跳过
			if hasXor {
				// fmt.Println("节点", u, "的字符位", c, "已有XOR=", x, "的邻居节点")
				// fmt.Scanln()
				continue
			}

			// 没有，从前缀树查找候选节点
			wantIdx := ui ^ x
			if wantIdx < 0 || wantIdx >= 32 {
				continue
			}

			// 用前缀树找"前c个字符相同"的候选节点
			prefix := ghu[:c]
			cands := hw.FindNodesWithPrefix(m.PrefixTree, prefix)
			filtered := make([]int, 0, len(cands))
			for _, v := range cands {
				if v == u {
					continue
				}
				ghv := m.NodeGeohash[v]
				if len(ghv) <= c {
					continue
				}
				vi := base32IndexByte(ghv[c])
				// 在第c个字符位置上满足 XOR=x 的节点
				if vi >= 0 && vi == wantIdx {
					filtered = append(filtered, v)
				}
			}

			// 如果没有找到候选节点，跳过
			if len(filtered) == 0 {
				// fmt.Println("节点", u, "的字符位", c, "没有找到", "XOR=", x, "的候选节点")
				// fmt.Scanln()
				continue
			}

			// 近邻优先排序
			sort.Slice(filtered, func(i, j int) bool {
				return hw.Distance(m.Coords[u], m.Coords[filtered[i]]) <
					hw.Distance(m.Coords[u], m.Coords[filtered[j]])
			})

			// 通过完整geohash的XOR计算桶号，放入对应的K桶
			added := 0
			for _, v := range filtered {
				if added >= ensurePerTarget {
					break
				}

				ghv := m.NodeGeohash[v]
				vBin := hw.ToBinary(ghv)

				// 找到首个不同位（基于完整geohash）
				diff := hw.FirstDiffBitPos(uBin, vBin)
				if diff < 0 {
					continue // 完全相同不该发生
				}

				// 计算桶号：bucket = TotalBits - diff
				bucket := m.TotalBits - diff
				if bucket < 1 || bucket > m.TotalBits {
					continue
				}

				// 检查是否已存在
				exists := false
				for _, existing := range m.KBuckets[u][bucket] {
					if existing == v {
						exists = true
						break
					}
				}
				if !exists {
					m.KBuckets[u][bucket] = append(m.KBuckets[u][bucket], v)
					added++

					// 记录添加信息
					record := hw.XorAnchorRecord{
						NodeID:      u,
						CharPos:     c,
						UChar:       ghu[c],
						VChar:       ghv[c],
						XorValue:    x,
						AddedNodeID: v,
						BucketID:    bucket,
						UGeohash:    ghu,
						VGeohash:    ghv,
					}
					records = append(records, record)
				}
			}
		}
	}

	// 补完做一次稳定去重
	for b := 0; b <= m.TotalBits; b++ {
		m.KBuckets[u][b] = hw.DedupIntsStable(m.KBuckets[u][b])
	}

	return records
}

// EnsureXorAnchors 全量补齐锚点（建议在 fillKBuckets 结束后调用一次）
// 返回值：所有添加的锚点记录
func (m *Mercator) EnsureXorAnchors(ensurePerTarget int) []hw.XorAnchorRecord {
	allRecords := make([]hw.XorAnchorRecord, 0)
	for u := 0; u < m.Graph.N; u++ {
		records := m.ensureCharXorAnchorsForNode(u, ensurePerTarget)
		allRecords = append(allRecords, records...)
	}
	return allRecords
}

// SetRoot 实现Algorithm接口 - 设置广播根节点
func (m *Mercator) SetRoot(root int) {
	m.TreeRoot = root
	m.ResetVisited() // 重置访问标记
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (m *Mercator) GetAlgoName() string {
	return "mercator"
}

// NeedSpecifiedRoot 实现Algorithm接口 - 是否需要为每个根重建
func (m *Mercator) NeedSpecifiedRoot() bool {
	return false // Mercator可以复用网络拓扑
}

// PrintInfo 打印图信息（调试用）
func (m *Mercator) PrintInfo() {
	avgOutbound := 0.0
	for i := 0; i < m.Graph.N; i++ {
		avgOutbound += float64(len(m.Graph.OutBound[i]))
	}
	avgOutbound /= float64(m.Graph.N)

	fmt.Printf("MERCATOR: 平均出度 = %.2f\n", avgOutbound)
	fmt.Printf("  Geohash精度: %d, K桶大小: %d, K0阈值: %d, K-ary因子: %d\n",
		m.GeoPrec, m.BucketSize, m.K0Threshold, m.KaryFactor)
}
