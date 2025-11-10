package algorithms

import (
	"fmt"
	"math/rand"

	hw "gomercator/handlware"
)

// ==================== Perigee UCB算法（完整实现）====================
// Perigee: 基于观测的动态邻居选择
// 论文: https://arxiv.org/pdf/2006.14186.pdf
//
// 核心思想:
// 1. 初始化: 构建随机图，为入边创建观测对象
// 2. Warmup阶段: 发送640条随机消息，记录延迟观测
// 3. 动态优化: 每10条消息后，使用UCB方法重选邻居
// 4. 运行时: 使用优化后的拓扑进行广播

const (
	TotalWarmupMessage = 640 // Warmup阶段消息总数
	WarmupRoundLen     = 10  // 每10条消息执行一次重选
)

// PerigeeUCB Perigee UCB算法完整实现
type PerigeeUCB struct {
	Graph        *hw.Graph                  // 网络图
	Coords       []hw.LatLonCoordinate      // 节点坐标
	TreeRoot     int                        // 当前广播树根节点
	RootFanout   int                        // 根节点扇出度
	Fanout       int                        // 普通节点扇出度
	MaxOutbound  int                        // 最大出度
	Observations [][]*hw.PerigeeObservation // 观测数据 [节点][观测索引]
	Rng          *rand.Rand                 // 随机数生成器
}

// NewPerigeeUCB 创建新的Perigee UCB算法实例（完整实现）
// 参数:
//   - n: 节点数
//   - coords: 节点坐标数组
//   - root: 广播根节点
//   - rootFanout: 根节点扇出度
//   - fanout: 普通节点扇出度
//   - maxOutbound: 最大出度
func NewPerigeeUCB(n int, coords []hw.LatLonCoordinate, root int, rootFanout, fanout, maxOutbound int) *PerigeeUCB {
	pg := &PerigeeUCB{
		Graph:        hw.NewGraph(n),
		Coords:       coords,
		TreeRoot:     root,
		RootFanout:   rootFanout,
		Fanout:       fanout,
		MaxOutbound:  maxOutbound,
		Observations: make([][]*hw.PerigeeObservation, n),
		Rng:          rand.New(rand.NewSource(int64(root))),
	}

	// 初始化观测数据结构
	for i := 0; i < n; i++ {
		pg.Observations[i] = make([]*hw.PerigeeObservation, 0)
	}

	fmt.Println("Perigee UCB: 完整实现（包含Warmup Phase）")
	fmt.Printf("  - Warmup消息数: %d\n", TotalWarmupMessage)
	fmt.Printf("  - 重选周期: 每%d条消息（与C++对齐）\n", WarmupRoundLen)

	// 构建初始图并执行warmup
	pg.buildInitialGraph(n, fanout)
	pg.warmupPhase(n, coords)

	return pg
}

// buildInitialGraph 构建初始随机图
func (pg *PerigeeUCB) buildInitialGraph(n, fanout int) {
	innerDeg := hw.InnerDeg // 4

	// 第一步: 构建出边（fanout - innerDeg个）
	for u := 0; u < n; u++ {
		dg := fanout - innerDeg
		for k := 0; k < dg; k++ {
			v := pg.Rng.Intn(n)
			for !pg.Graph.AddEdge(u, v) {
				v = pg.Rng.Intn(n)
			}
		}
	}

	// 第二步: 构建入边（innerDeg个），并创建观测对象
	for u := 0; u < n; u++ {
		dg := innerDeg
		for k := 0; k < dg; k++ {
			v := pg.Rng.Intn(n)
			for !pg.Graph.AddEdge(u, v) {
				v = pg.Rng.Intn(n)
			}

			// 为这条入边创建观测对象（v -> u的边，u记录观测）
			if len(pg.Observations[v]) < innerDeg {
				obs := hw.NewPerigeeObservation(u, v)
				pg.Observations[v] = append(pg.Observations[v], obs)
			}
		}
	}

	fmt.Printf("初始图构建完成: %d个节点，%d条边\n", n, pg.Graph.M)
}

// warmupPhase Warmup阶段 - 发送随机消息并优化拓扑
func (pg *PerigeeUCB) warmupPhase(n int, coords []hw.LatLonCoordinate) {
	fmt.Println("开始Warmup Phase...")

	// Warmup状态
	recvFlag := make([]int, n)
	recvTime := make([]float64, n)
	for i := 0; i < n; i++ {
		recvFlag[i] = -1
	}

	totalReselections := 0

	// 发送640条随机消息
	for warmupMsg := 0; warmupMsg < TotalWarmupMessage; warmupMsg++ {
		if warmupMsg%100 == 0 && warmupMsg > 0 {
			fmt.Printf("  Warmup进度: %d/%d\n", warmupMsg, TotalWarmupMessage)
		}

		// 随机选择根节点
		root := pg.Rng.Intn(n)

		// 初始化消息队列
		msgQueue := hw.NewPriorityQueue()
		msgQueue.Push(hw.NewMessage(root, root, root, 0, 0, 0))

		// 模拟消息传播
		for !msgQueue.Empty() {
			msg := msgQueue.Pop()
			u := msg.Dst

			// 如果是新消息（第一次收到）
			if recvFlag[u] < warmupMsg {
				recvFlag[u] = warmupMsg
				recvTime[u] = msg.RecvTime

				// 获取转发列表
				relayList := pg.Respond(msg)
				delayTime := 0.0
				if u == root {
					delayTime = 0
				}

				// 向邻居转发
				for _, v := range relayList {
					dist := hw.Distance(coords[u], coords[v])*3 + hw.FixedDelay
					newMsg := hw.NewMessage(root, u, v, msg.Step+1,
						recvTime[u]+delayTime, recvTime[u]+dist+delayTime)
					msgQueue.Push(newMsg)
				}
			}

			// 添加观测数据（即使是重复消息也记录）
			for _, obs := range pg.Observations[u] {
				if obs.Src == msg.Src {
					// 记录时间差: 当前消息到达时间 - 第一次收到消息的时间
					obs.Add(msg.RecvTime - recvTime[u])
				}
			}
		}

		// 每10条消息后，执行邻居重选
		if (warmupMsg+1)%WarmupRoundLen == 0 {
			reselectCount := 0
			for i := 0; i < n; i++ {
				if pg.neighborReselection(i, n) {
					reselectCount++
				}
			}
			totalReselections += reselectCount
		}
	}

	// 填充到maxOutbound
	for u := 0; u < n; u++ {
		remainDeg := pg.MaxOutbound - len(pg.Graph.OutBound[u])
		for k := 0; k < remainDeg; k++ {
			v := pg.Rng.Intn(n)
			for !pg.Graph.AddEdge(u, v) {
				v = pg.Rng.Intn(n)
			}
		}
	}

	// 统计平均出度
	avgOutbound := 0.0
	for i := 0; i < n; i++ {
		avgOutbound += float64(len(pg.Graph.OutBound[i]))
	}
	avgOutbound /= float64(n)

	fmt.Printf("Warmup Phase完成!\n")
	fmt.Printf("  - 总重选次数: %d\n", totalReselections)
	fmt.Printf("  - 平均出度: %.3f\n", avgOutbound)
}

// neighborReselection 邻居重选（基于UCB）
// 返回: true表示执行了重选，false表示未重选
func (pg *PerigeeUCB) neighborReselection(nodeID int, n int) bool {
	obs := pg.Observations[nodeID]
	if len(obs) == 0 {
		return false
	}

	// 计算每个观测对象的LCB和UCB
	maxLCB := 0.0
	argMaxLCB := 0
	minUCB := 1e18

	for i := 0; i < len(obs); i++ {
		lcb, ucb := obs[i].GetLCBUCB()

		if lcb > maxLCB {
			maxLCB = lcb
			argMaxLCB = i
		}

		if ucb < minUCB {
			minUCB = ucb
		}
	}

	// 如果最大的LCB > 最小的UCB，说明最差的邻居需要被替换
	if maxLCB > minUCB {
		// 删除最差的邻居
		worstSrc := obs[argMaxLCB].Src
		pg.Graph.DelEdge(worstSrc, nodeID)

		// 随机选择新邻居
		newSrc := pg.Rng.Intn(n)
		for len(pg.Graph.OutBound[newSrc]) >= pg.MaxOutbound || !pg.Graph.AddEdge(newSrc, nodeID) {
			newSrc = pg.Rng.Intn(n)
		}

		// 重置观测对象
		obs[argMaxLCB] = hw.NewPerigeeObservation(newSrc, nodeID)
		return true
	}

	return false
}

// Respond 实现Algorithm接口 - 响应消息
func (pg *PerigeeUCB) Respond(msg *hw.Message) []int {
	u := msg.Dst
	nbU := pg.Graph.Outbound(u)
	ret := make([]int, 0, len(nbU))

	// 向所有出边邻居转发（除了消息来源）
	for _, v := range nbU {
		if v != msg.Src {
			ret = append(ret, v)
		}
	}

	// 如果是根节点，增加额外转发
	if msg.Step == 0 {
		remainDeg := pg.RootFanout - len(ret)
		for i := 0; i < remainDeg; i++ {
			v := pg.Rng.Intn(pg.Graph.N)
			if u != v && !hw.Contains(ret, v) {
				ret = append(ret, v)
			}
		}
	}

	return ret
}

// SetRoot 实现Algorithm接口 - 设置广播根节点
func (pg *PerigeeUCB) SetRoot(root int) {
	pg.TreeRoot = root
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (pg *PerigeeUCB) GetAlgoName() string {
	return "perigee_ucb"
}

// NeedSpecifiedRoot 实现Algorithm接口 - 是否需要为每个根重建
func (pg *PerigeeUCB) NeedSpecifiedRoot() bool {
	return false
}

// PrintInfo 打印图信息（调试用）
func (pg *PerigeeUCB) PrintInfo() {
	avgOutbound := 0.0
	for i := 0; i < pg.Graph.N; i++ {
		avgOutbound += float64(len(pg.Graph.OutBound[i]))
	}
	avgOutbound /= float64(pg.Graph.N)

	fmt.Printf("Perigee UCB: 平均出度 = %.2f\n", avgOutbound)
}
