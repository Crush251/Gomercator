package handlware

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// SimulatorConfig 模拟器配置
type SimulatorConfig struct {
	Bandwidth float64 // 带宽（bps）
	DataSize  float64 // 数据包大小（Bytes）
	MaxNodes  int     // 最大节点数
}

// NewSimulatorConfig 创建默认配置
func NewSimulatorConfig() *SimulatorConfig {
	return &SimulatorConfig{
		Bandwidth: BandwidthDefault,
		DataSize:  DataSizeSmall,
		MaxNodes:  8000,
	}
}

// ==================== 单根节点模拟 ====================

// SingleRootSimulation 单个根节点的广播模拟
// 参数:
//   - root: 广播根节点ID
//   - reptTime: 重复测试次数
//   - coords: 节点坐标数组
//   - malFlags: 恶意节点标记（true表示恶意，拒绝转发）
//   - leaveFlags: 节点离开标记（true表示离开，接收但不转发）
//   - algo: 广播算法实现
//   - config: 模拟器配置
//   - clusterResult: 聚类结果（可选，用于统计）
//
// 返回: 测试结果
func SingleRootSimulation(
	root int,
	reptTime int,
	coords []LatLonCoordinate,
	malFlags []bool,
	leaveFlags []bool,
	algo Algorithm,
	config *SimulatorConfig,
	clusterResult *ClusterResult,
) *TestResult {

	const inf = 1e8
	n := len(coords)
	result := NewTestResult(n)
	// 用本地数组承接，结束时赋回结果
	successChildren := make([][]int, n)

	// 如果算法需要指定根节点，则重新初始化
	// 注意：这在外部已经处理，此处不需要重建
	algo.SetRoot(root)

	for rept := 0; rept < reptTime; rept++ {
		// 初始化状态
		recvFlag := make([]bool, n)
		recvTime := make([]float64, n)
		recvDist := make([]float64, n)
		recvParent := make([]int, n)
		depth := make([]int, n)
		recvList := make([]int, 0, n)

		for i := 0; i < n; i++ {
			recvParent[i] = -1
		}

		dupMsg := 0

		// 初始化消息队列
		msgQueue := NewPriorityQueue()
		msgQueue.Push(NewMessage(root, root, root, 0, 0, 0))

		// 事件驱动模拟
		for !msgQueue.Empty() {
			msg := msgQueue.Pop()
			u := msg.Dst // 当前接收节点

			// 重复消息，忽略
			if recvFlag[u] {
				dupMsg++
				continue
			}
			// 首次成功到达：计入“成功转发边”
			// 注意：忽略 root->root 的初始化自环
			if msg.Src != u {
				successChildren[msg.Src] = append(successChildren[msg.Src], u)
			}
			// 标记为已接收
			recvFlag[u] = true
			recvTime[u] = msg.RecvTime
			recvDist[u] = msg.RecvTime - msg.SendTime
			recvParent[u] = msg.Src
			recvList = append(recvList, u)

			if u != root {
				depth[u] = depth[msg.Src] + 1
			}

			// 恶意节点或离开节点，不转发
			if malFlags[u] || leaveFlags[u] {
				continue
			}

			// 调用算法的respond函数，获取转发节点列表
			relayList := algo.Respond(msg)

			// 计算处理延迟
			delayTime := CalculateProcessingDelay()

			// 向转发列表中的节点发送消息
			for _, v := range relayList {
				// 计算传播延迟
				// 注意：普通算法两种情况都使用系数3（与C++ single_root_simulation对齐）
				dist := CalculatePropagationDelay(u, v, coords, config.Bandwidth, config.DataSize)

				newMsg := NewMessage(root, u, v, msg.Step+1, recvTime[u]+delayTime, recvTime[u]+dist+delayTime)
				msgQueue.Push(newMsg)
			}
		}

		// 统计结果
		clusterRecvCount := make([]int, K)
		recvCount := 0
		avgLatency := 0.0

		for i := 0; i < n; i++ {
			if !recvFlag[i] && !malFlags[i] && !leaveFlags[i] {
				// 未覆盖的节点
				recvTime[i] = inf
				recvList = append(recvList, i)
				depth[i] = MaxDepth - 1
			} else if recvFlag[i] {
				recvCount++
				avgLatency += recvTime[i]

				// 簇统计
				if clusterResult != nil {
					c := clusterResult.ClusterID[i]
					if c >= 0 && c < K {
						clusterRecvCount[c]++
						result.ClusterAvgDepth[c] += float64(depth[i])
						result.ClusterAvgLatency[c] += recvTime[i]
					}
				}
			}
		}

		if recvCount > 0 {
			avgLatency /= float64(recvCount)
		}

		// 簇统计平均值
		if clusterResult != nil {
			for c := 0; c < K; c++ {
				if clusterRecvCount[c] > 0 {
					result.ClusterAvgDepth[c] /= float64(clusterRecvCount[c])
					result.ClusterAvgLatency[c] /= float64(clusterRecvCount[c])
				}
			}
		}

		// 计算带宽消耗
		nonMalNode := len(recvList)
		result.AvgBandwidth += float64(dupMsg+nonMalNode) / float64(nonMalNode)

		// 深度统计
		depthCnt := make([]int, MaxDepth)
		for _, u := range recvList {
			d := depth[u]
			if d >= 0 && d < MaxDepth {
				result.DepthCDF[d] += 1
				result.AvgDist[d] += recvDist[u]
				depthCnt[d]++
			}
		}

		result.AvgLatency = avgLatency

		// 归一化
		for i := 0; i < MaxDepth; i++ {
			result.DepthCDF[i] /= float64(nonMalNode)
			if depthCnt[i] > 0 {
				result.AvgDist[i] /= float64(depthCnt[i])
			}
		}

		// 计算延迟百分位
		// 按接收时间排序
		sort.Slice(recvList, func(i, j int) bool {
			return recvTime[recvList[i]] < recvTime[recvList[j]]
		})

		cnt := 0
		for pct := 0.05; pct <= 1.0; pct += 0.05 {
			idx := int(float64(nonMalNode) * pct)
			if idx >= nonMalNode {
				idx = nonMalNode - 1
			}
			result.Latency[cnt] += recvTime[recvList[idx]]
			cnt++
		}

		// 打印收到消息的节点数
		if rept == 0 {
			fmt.Printf("收到消息的节点数: %d/%d (%.1f%%)\n", recvCount, n, float64(recvCount)*100.0/float64(n))
		}
		result.SuccessChildren = successChildren
	}

	// 多次重复的平均值
	result.AvgBandwidth /= float64(reptTime)
	for i := 0; i < MaxDepth; i++ {
		result.DepthCDF[i] /= float64(reptTime)
	}

	for i := 0; i < len(result.Latency); i++ {
		tmp := int(result.Latency[i] / inf)
		result.Latency[i] -= float64(tmp) * inf
		validCount := reptTime - tmp

		if validCount == 0 {
			result.Latency[i] = 0
		} else {
			result.Latency[i] /= float64(validCount)
		}

		if result.Latency[i] < 0.1 {
			result.Latency[i] = inf
		}
	}

	return result
}

// ==================== 多根节点模拟 ====================

// Simulation 多根节点广播模拟（测试整个网络）
// 参数:
//   - reptTime: 重复测试次数
//   - coords: 节点坐标数组
//   - attackConfig: 攻击配置
//   - algo: 广播算法实现
//   - config: 模拟器配置
//   - clusterResult: 聚类结果（可选）
//
// 返回: 累积的测试结果
func Simulation(
	reptTime int,
	coords []LatLonCoordinate,
	attackConfig *AttackConfig,
	algo Algorithm,
	config *SimulatorConfig,
	clusterResult *ClusterResult,
) *TestResult {

	rand.Seed(100) // 固定种子，确保可重复性
	n := len(coords)
	result := NewTestResult(n)
	testTime := 0

	for rept := 0; rept < reptTime; rept++ {
		fmt.Printf("重复测试 %d/%d\n", rept+1, reptTime)

		// 1) 生成恶意节点列表
		malFlags := GenerateMaliciousNodes(n, attackConfig.MaliciousRatio)

		// 2) 生成节点离开列表
		leaveFlags := GenerateLeaveNodes(n, attackConfig.NodeLeaveRatio)

		// 3) 如果算法需要为每个根重建，则在外部重新创建algo实例
		// 这里假设algo已经在外部正确初始化

		// 4) 测试多个随机根节点
		testNodes := 20
		for t := 0; t < testNodes; t++ {
			fmt.Printf("  测试节点 %d/%d\n", t+1, testNodes)

			// 随机选择一个非恶意、未离开的根节点
			root := rand.Intn(n)
			for malFlags[root] || leaveFlags[root] {
				root = rand.Intn(n)
			}

			testTime++

			// 如果算法需要指定根节点重建
			if algo.NeedSpecifiedRoot() {
				// 这里需要外部重新创建算法实例
				// 暂时使用SetRoot
				algo.SetRoot(root)
			}

			// 单根模拟
			res := SingleRootSimulation(root, 1, coords, malFlags, leaveFlags, algo, config, clusterResult)
			_ = WriteSuccessChildrenCSV("success_edges.csv", root, res.SuccessChildren)
			// 累积结果
			AccumulateResults(result, res)
		}
	}

	// 计算平均值
	AverageResults(result, testTime)

	fmt.Printf("模拟完成，共测试 %d 次\n", testTime)
	return result
}

// ==================== 攻击场景生成 ====================

// GenerateMaliciousNodes 生成恶意节点标记（拒绝转发）
func GenerateMaliciousNodes(n int, ratio float64) []bool {
	flags := make([]bool, n)
	count := int(float64(n) * ratio)

	for i := 0; i < count; i++ {
		node := rand.Intn(n)
		for flags[node] {
			node = rand.Intn(n)
		}
		flags[node] = true
	}

	if count > 0 {
		fmt.Printf("生成 %d 个恶意节点 (%.1f%%)\n", count, ratio*100)
	}

	return flags
}

// GenerateLeaveNodes 生成节点离开标记（接收但不转发）
// 注意：节点离开与恶意节点的区别：
//   - 恶意节点：完全不响应
//   - 离开节点：接收消息但不转发，统计时不计入
func GenerateLeaveNodes(n int, ratio float64) []bool {
	flags := make([]bool, n)
	count := int(float64(n) * ratio)

	for i := 0; i < count; i++ {
		node := rand.Intn(n)
		for flags[node] {
			node = rand.Intn(n)
		}
		flags[node] = true
	}

	if count > 0 {
		fmt.Printf("生成 %d 个离开节点 (%.1f%%)\n", count, ratio*100)
	}

	return flags
}

// GenerateFakeCoordinates 生成伪造坐标（Mercator专用攻击）
// 参数:
//   - coords: 真实坐标数组
//   - ratio: 谎报坐标节点比例
//   - offsetDegree: 偏移度数（如10, 20, 30，或-1表示完全随机）
//
// 返回: (伪造坐标数组, 谎报标记数组)
func GenerateFakeCoordinates(coords []LatLonCoordinate, ratio float64, offsetDegree float64) ([]LatLonCoordinate, []bool) {
	n := len(coords)
	fakeCoords := make([]LatLonCoordinate, n)
	flags := make([]bool, n)

	// 复制真实坐标
	copy(fakeCoords, coords)

	count := int(float64(n) * ratio)
	fmt.Printf("设置 %d 个节点伪造坐标 (%.1f%%)\n", count, ratio*100)

	for i := 0; i < count; i++ {
		node := rand.Intn(n)
		for flags[node] {
			node = rand.Intn(n)
		}
		flags[node] = true

		if offsetDegree > 0 {
			// 基于真实坐标偏移
			offset := offsetDegree
			fakeCoords[node].Lat = coords[node].Lat + (rand.Float64()*2-1)*offset
			fakeCoords[node].Lon = coords[node].Lon + (rand.Float64()*2-1)*offset
		} else {
			// 完全随机
			fakeCoords[node].Lat = rand.Float64()*180 - 90
			fakeCoords[node].Lon = rand.Float64()*360 - 180
		}

		// 确保坐标在有效范围内
		fakeCoords[node].Lat = math.Max(-90, math.Min(90, fakeCoords[node].Lat))
		fakeCoords[node].Lon = math.Max(-180, math.Min(180, fakeCoords[node].Lon))
	}

	fmt.Printf("伪造坐标设置完成\n")
	return fakeCoords, flags
}
