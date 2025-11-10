package algorithms

import (
	"math/rand"

	hw "gomercator/handlware"
)

// ==================== Random Flood算法 ====================
// 随机泛洪：每个节点随机选择若干个邻居进行转发
// 1. 预先构建随机图（每个节点连接到若干个随机邻居）
// 2. 消息转发时，向所有出边邻居转发（除了消息来源）
// 3. 根节点可以有更高的扇出度

// RandomFlood Random Flood算法实现
type RandomFlood struct {
	Graph        *hw.Graph              // 随机图
	Coords       []hw.LatLonCoordinate  // 节点坐标
	TreeRoot     int                    // 当前广播树根节点
	RootFanout   int                    // 根节点扇出度
	SecondFanout int                    // 第二层扇出度（未使用）
	Fanout       int                    // 普通节点扇出度
}

// NewRandomFlood 创建新的Random Flood算法实例
// 参数:
//   - n: 节点数
//   - coords: 节点坐标数组
//   - root: 广播根节点（用于初始化，可后续通过SetRoot更改）
//   - rootFanout: 根节点扇出度
//   - fanout: 普通节点扇出度
func NewRandomFlood(n int, coords []hw.LatLonCoordinate, root int, rootFanout, fanout int) *RandomFlood {
	rf := &RandomFlood{
		Graph:        hw.NewGraph(n),
		Coords:       coords,
		TreeRoot:     root,
		RootFanout:   rootFanout,
		SecondFanout: fanout, // 未使用，保持兼容性
		Fanout:       fanout,
	}

	// 构建随机图
	rf.buildRandomGraph(n, fanout)

	return rf
}

// buildRandomGraph 构建随机图
func (rf *RandomFlood) buildRandomGraph(n, fanout int) {
	// 为每个节点随机选择fanout个出边邻居
	for u := 0; u < n; u++ {
		for k := 0; k < fanout; k++ {
			v := rand.Intn(n)
			// 尝试添加边，避免自环和重边
			for !rf.Graph.AddEdge(u, v) {
				v = rand.Intn(n)
			}
		}
	}
}

// Respond 实现Algorithm接口 - 响应消息
func (rf *RandomFlood) Respond(msg *hw.Message) []int {
	u := msg.Dst
	nbU := rf.Graph.Outbound(u)
	ret := make([]int, 0, len(nbU))

	// 向所有出边邻居转发（除了消息来源）
	for _, v := range nbU {
		if v != msg.Src {
			ret = append(ret, v)
		}
	}

	// 如果是根节点，可能需要增加额外的随机转发
	if u == rf.TreeRoot && msg.Step == 0 {
		remainDeg := rf.RootFanout - len(ret)
		for i := 0; i < remainDeg; i++ {
			v := rand.Intn(rf.Graph.N)
			if v != msg.Src && !hw.Contains(ret, v) {
				ret = append(ret, v)
			}
		}
	}

	return ret
}

// SetRoot 实现Algorithm接口 - 设置广播根节点
func (rf *RandomFlood) SetRoot(root int) {
	rf.TreeRoot = root
}

// GetAlgoName 实现Algorithm接口 - 获取算法名称
func (rf *RandomFlood) GetAlgoName() string {
	return "random_flood"
}

// NeedSpecifiedRoot 实现Algorithm接口 - 是否需要为每个根重建
func (rf *RandomFlood) NeedSpecifiedRoot() bool {
	return false // Random Flood不需要为每个根重建图
}

// PrintInfo 打印图信息（调试用）
func (rf *RandomFlood) PrintInfo() {
	avgOutbound := 0.0
	for i := 0; i < rf.Graph.N; i++ {
		avgOutbound += float64(len(rf.Graph.OutBound[i]))
	}
	avgOutbound /= float64(rf.Graph.N)
	
	// 这里可以打印详细信息，暂时简化
	// fmt.Printf("Random Flood: 平均出度 = %.2f\n", avgOutbound)
	_ = avgOutbound
}

