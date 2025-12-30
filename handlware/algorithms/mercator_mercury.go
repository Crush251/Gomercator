package algorithms

import (
	"fmt"
	"math"
	"sort"

	hw "gomercator/handlware"
)

// ==================== MERCATOR-MERCURY混合算法 ====================
// MERCATOR-MERCURY: 结合Mercury分层Hub思想的Mercator变种
// 核心思想:
// 1. 多层Hub网络：Global Hubs（精度2区域代表）+ Regional Hubs（精度3子区域代表）
// 2. Hub之间形成骨干网（类似Mercury的root+second层）
// 3. 普通节点连接到Hub + 采样K0邻居
// 4. 传播路径：源节点 → K0邻居 + Hub → 其他Hubs → 下层节点

// MercatorMercury Mercator-Mercury混合算法
type MercatorMercury struct {
	*Mercator

	// Hub相关
	GlobalHubs   []int          // 全局Hub节点列表（精度2区域代表）
	RegionalHubs map[string]int // 区域前缀 -> Hub节点ID（包括精度2和精度3）
	NodeHub      []int          // 每个节点对应的Hub
	IsHub        []bool         // 标记是否为Hub节点

	// 连接结构
	K0Neighbors    [][]int   // 采样后的K0邻居
	HubConnections [][]int   // Hub之间的连接
	HubChildren    [][]int   // 每个Hub的子节点列表
	K0SampleSize   int       // K0桶采样大小
	HubFanout      int       // Hub转发扇出
}

// NewMercatorMercury 创建Mercator-Mercury混合算法实例
func NewMercatorMercury(n int, realCoords, displayCoords []hw.LatLonCoordinate, root int,
	geoPrec, bucketSize, k0Threshold, karyFactor, k0SampleSize, hubFanout int) *MercatorMercury {

	// 先创建标准Mercator
	baseMercator := NewMercator(n, realCoords, displayCoords, root, geoPrec, bucketSize, k0Threshold, karyFactor)

	mm := &MercatorMercury{
		Mercator:       baseMercator,
		GlobalHubs:     make([]int, 0),
		RegionalHubs:   make(map[string]int),
		NodeHub:        make([]int, n),
		IsHub:          make([]bool, n),
		K0Neighbors:    make([][]int, n),
		HubConnections: make([][]int, n),
		HubChildren:    make([][]int, n),
		K0SampleSize:   k0SampleSize,
		HubFanout:      hubFanout,
	}

	// 初始化NodeHub为-1（无Hub）
	for i := 0; i < n; i++ {
		mm.NodeHub[i] = -1
	}

	// 构建Hub网络
	mm.selectHubs()
	mm.buildHubNetwork()
	mm.sampleK0Buckets()
	mm.connectNodesToHubs()

	return mm
}

// selectHubs 选择Hub节点
func (mm *MercatorMercury) selectHubs() {
	fmt.Println("开始选择Hub节点...")

	// 1. 按精度2分组
	groups := make(map[string][]int)
	for i := 0; i < len(mm.NodeGeohash); i++ {
		if len(mm.NodeGeohash[i]) < 2 {
			continue
		}
		prefix := mm.NodeGeohash[i][:2]
		groups[prefix] = append(groups[prefix], i)
	}

	// 2. 为每个精度2区域选择Global Hub
	for prefix, nodes := range groups {
		if len(nodes) == 0 {
			continue
		}

		// 选择该区域的Hub（地理中心或度数最高）
		hub := mm.selectHubFromGroup(nodes)
		mm.GlobalHubs = append(mm.GlobalHubs, hub)
		mm.IsHub[hub] = true
		mm.RegionalHubs[prefix] = hub

		// 如果区域很大（>50个节点），选择精度3的子区域Hub
		if len(nodes) > 50 {
			subGroups := make(map[string][]int)
			for _, node := range nodes {
				if len(mm.NodeGeohash[node]) < 3 {
					continue
				}
				subPrefix := mm.NodeGeohash[node][:3]
				subGroups[subPrefix] = append(subGroups[subPrefix], node)
			}

			for subPrefix, subNodes := range subGroups {
				if len(subNodes) > 10 {
					subHub := mm.selectHubFromGroup(subNodes)
					if subHub != hub { // 避免重复
						mm.IsHub[subHub] = true
						mm.RegionalHubs[subPrefix] = subHub

						// 子Hub连接到父Hub（双向）
						mm.HubConnections[subHub] = append(mm.HubConnections[subHub], hub)
						mm.HubConnections[hub] = append(mm.HubConnections[hub], subHub)
					}
				}
			}
		}
	}

	hubCount := 0
	for i := 0; i < len(mm.IsHub); i++ {
		if mm.IsHub[i] {
			hubCount++
		}
	}

	fmt.Printf("Hub选择完成:\n")
	fmt.Printf("  Global Hubs: %d\n", len(mm.GlobalHubs))
	fmt.Printf("  总Hub数量: %d (占比%.2f%%)\n", hubCount, 100.0*float64(hubCount)/float64(len(mm.IsHub)))
}

// selectHubFromGroup 从一组节点中选择Hub
// 策略：选择地理中心最近的节点
func (mm *MercatorMercury) selectHubFromGroup(nodes []int) int {
	if len(nodes) == 1 {
		return nodes[0]
	}

	// 计算地理中心
	var centerLat, centerLon float64
	for _, node := range nodes {
		centerLat += mm.Coords[node].Lat
		centerLon += mm.Coords[node].Lon
	}
	centerLat /= float64(len(nodes))
	centerLon /= float64(len(nodes))

	// 找到最接近中心的节点
	minDist := math.MaxFloat64
	bestNode := nodes[0]

	for _, node := range nodes {
		lat := mm.Coords[node].Lat
		lon := mm.Coords[node].Lon
		dist := math.Sqrt((lat-centerLat)*(lat-centerLat) + (lon-centerLon)*(lon-centerLon))

		if dist < minDist {
			minDist = dist
			bestNode = node
		}
	}

	return bestNode
}

// buildHubNetwork 构建Hub之间的骨干网络
func (mm *MercatorMercury) buildHubNetwork() {
	fmt.Println("构建Hub骨干网络...")

	// Global Hubs之间全连接（形成骨干网）
	for i := 0; i < len(mm.GlobalHubs); i++ {
		hubI := mm.GlobalHubs[i]

		for j := i + 1; j < len(mm.GlobalHubs); j++ {
			hubJ := mm.GlobalHubs[j]

			// 双向连接
			mm.HubConnections[hubI] = append(mm.HubConnections[hubI], hubJ)
			mm.HubConnections[hubJ] = append(mm.HubConnections[hubJ], hubI)
		}
	}

	// 统计平均度数
	if len(mm.GlobalHubs) > 0 {
		avgDegree := len(mm.HubConnections[mm.GlobalHubs[0]])
		fmt.Printf("Hub骨干网络构建完成，平均度数: %d\n", avgDegree)
	}
}

// sampleK0Buckets 对K0桶进行采样（与MercatorSampled相同策略）
func (mm *MercatorMercury) sampleK0Buckets() {
	fmt.Println("对K0桶进行采样...")

	for i := 0; i < len(mm.KBuckets); i++ {
		k0Bucket := mm.KBuckets[i][0]

		if len(k0Bucket) <= mm.K0SampleSize {
			mm.K0Neighbors[i] = make([]int, len(k0Bucket))
			copy(mm.K0Neighbors[i], k0Bucket)
		} else {
			mm.K0Neighbors[i] = mm.distanceBasedSample(i, k0Bucket, mm.K0SampleSize)
		}
	}
}

// distanceBasedSample 基于距离的采样（与MercatorSampled相同）
func (mm *MercatorMercury) distanceBasedSample(nodeID int, k0Bucket []int, k int) []int {
	if len(k0Bucket) <= k {
		return k0Bucket
	}

	distances := make([]hw.PairFloatInt, 0, len(k0Bucket))
	for _, neighbor := range k0Bucket {
		dist := hw.Distance(mm.Coords[nodeID], mm.Coords[neighbor])
		distances = append(distances, hw.PairFloatInt{First: dist, Second: neighbor})
	}

	sort.Slice(distances, func(a, b int) bool {
		return distances[a].First < distances[b].First
	})

	selected := make([]int, 0, k)

	nearCount := k / 2
	for i := 0; i < nearCount && i < len(distances); i++ {
		selected = append(selected, distances[i].Second)
	}

	farCount := k - nearCount
	if farCount > 0 {
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

// connectNodesToHubs 连接节点到对应的Hub
func (mm *MercatorMercury) connectNodesToHubs() {
	fmt.Println("连接节点到Hub...")

	for i := 0; i < len(mm.NodeGeohash); i++ {
		if mm.IsHub[i] {
			continue // Hub节点不需要连接到其他Hub
		}

		hash := mm.NodeGeohash[i]

		// 先找精度3的Hub
		if len(hash) >= 3 {
			prefix3 := hash[:3]
			if hub, exists := mm.RegionalHubs[prefix3]; exists {
				mm.NodeHub[i] = hub
				mm.HubChildren[hub] = append(mm.HubChildren[hub], i)
				continue
			}
		}

		// 再找精度2的Hub
		if len(hash) >= 2 {
			prefix2 := hash[:2]
			if hub, exists := mm.RegionalHubs[prefix2]; exists {
				mm.NodeHub[i] = hub
				mm.HubChildren[hub] = append(mm.HubChildren[hub], i)
			}
		}
	}

	fmt.Println("节点到Hub连接完成")
}

// Respond 实现Algorithm接口 - 生成中继节点列表
func (mm *MercatorMercury) Respond(msg *hw.Message) []int {
	u := msg.Dst
	relayNodes := make([]int, 0)

	if mm.Visited[u][msg.Step] {
		return relayNodes
	}

	mm.Visited[u][msg.Step] = true

	if msg.Step == 0 {
		// ===== 消息源节点 =====

		// 1. Flooding采样后的K0邻居
		for _, v := range mm.K0Neighbors[u] {
			if v != msg.Src {
				relayNodes = append(relayNodes, v)
			}
		}

		// 2. 如果是Hub节点，广播到所有其他Global Hubs
		if mm.IsHub[u] {
			for _, hub := range mm.GlobalHubs {
				if hub != u {
					relayNodes = append(relayNodes, hub)
				}
			}
		} else {
			// 3. 普通节点：转发到自己的Hub
			hub := mm.NodeHub[u]
			if hub >= 0 && hub != msg.Src {
				relayNodes = append(relayNodes, hub)
			}
		}

		// 4. 转发其他K桶（标准Mercator逻辑）
		for bucketIdx := 1; bucketIdx < len(mm.KBuckets[u]); bucketIdx++ {
			for _, v := range mm.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}

	} else if mm.IsHub[u] {
		// ===== Hub节点（中继） =====

		srcIsHub := mm.IsHub[msg.Src]

		if !srcIsHub {
			// 消息来自普通节点，广播到所有其他Global Hubs
			for _, hub := range mm.GlobalHubs {
				if hub != u && hub != msg.Src {
					relayNodes = append(relayNodes, hub)
				}
			}

			// 转发到连接的子Hub
			for _, subHub := range mm.HubConnections[u] {
				if subHub != msg.Src && !mm.isGlobalHub(subHub) {
					relayNodes = append(relayNodes, subHub)
				}
			}
		} else {
			// 消息来自其他Hub，向下广播到子节点
			for _, child := range mm.HubChildren[u] {
				if child != msg.Src {
					relayNodes = append(relayNodes, child)
				}
			}

			// 转发到子Hub
			for _, subHub := range mm.HubConnections[u] {
				if subHub != msg.Src && !mm.isGlobalHub(subHub) {
					relayNodes = append(relayNodes, subHub)
				}
			}
		}

	} else {
		// ===== 普通节点（非源） =====

		// 1. Flooding采样后的K0邻居
		for _, v := range mm.K0Neighbors[u] {
			if v != msg.Src {
				relayNodes = append(relayNodes, v)
			}
		}

		// 2. 标准Mercator跨区域转发
		srcBucket := hw.GetGeoBucketIndex(mm.NodeGeohash[u], mm.NodeGeohash[msg.Src], mm.TotalBits)
		for bucketIdx := 1; bucketIdx < srcBucket; bucketIdx++ {
			for _, v := range mm.KBuckets[u][bucketIdx] {
				if v != msg.Src {
					relayNodes = append(relayNodes, v)
				}
			}
		}
	}

	return relayNodes
}

// isGlobalHub 判断是否为Global Hub
func (mm *MercatorMercury) isGlobalHub(nodeID int) bool {
	for _, hub := range mm.GlobalHubs {
		if hub == nodeID {
			return true
		}
	}
	return false
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (mm *MercatorMercury) GetAlgoName() string {
	return "mercator_mercury"
}

// PrintInfo 打印算法信息
func (mm *MercatorMercury) PrintInfo() {
	fmt.Printf("MERCATOR-MERCURY: 分层Hub混合版本\n")
	fmt.Printf("  Global Hubs: %d\n", len(mm.GlobalHubs))

	hubCount := 0
	for i := 0; i < len(mm.IsHub); i++ {
		if mm.IsHub[i] {
			hubCount++
		}
	}
	fmt.Printf("  总Hub数量: %d\n", hubCount)
	fmt.Printf("  K0采样大小: %d\n", mm.K0SampleSize)
}


