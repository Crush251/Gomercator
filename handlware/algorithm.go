package handlware

// Algorithm 广播算法接口
// 所有广播算法（Random, BlockP2P, Perigee, Mercury, Mercator）都需要实现此接口
type Algorithm interface {
	// Respond 响应消息，返回转发节点列表
	// msg: 接收到的消息
	// 返回: 需要转发到的节点ID列表
	Respond(msg *Message) []int

	// SetRoot 设置广播根节点
	// root: 广播树的根节点ID
	SetRoot(root int)

	// GetAlgoName 获取算法名称
	// 返回: 算法名称字符串，用于日志和结果输出
	GetAlgoName() string

	// NeedSpecifiedRoot 是否需要为每个根节点重新构建网络拓扑
	// 返回: true表示需要重建（如static_build_tree），false表示可复用（如random_flood）
	NeedSpecifiedRoot() bool
}

// BaseAlgorithm 算法基类，提供默认实现
type BaseAlgorithm struct {
	Name            string
	SpecifiedRoot   bool
	Graph           *Graph
	Coords          []LatLonCoordinate
	Root            int
}

// SetRoot 默认实现
func (ba *BaseAlgorithm) SetRoot(root int) {
	ba.Root = root
}

// GetAlgoName 默认实现
func (ba *BaseAlgorithm) GetAlgoName() string {
	return ba.Name
}

// NeedSpecifiedRoot 默认实现
func (ba *BaseAlgorithm) NeedSpecifiedRoot() bool {
	return ba.SpecifiedRoot
}

// Respond 需要子类实现
func (ba *BaseAlgorithm) Respond(msg *Message) []int {
	// 默认返回所有出边邻居（除了消息来源）
	u := msg.Dst
	nbU := ba.Graph.Outbound(u)
	ret := make([]int, 0, len(nbU))
	for _, v := range nbU {
		if v != msg.Src {
			ret = append(ret, v)
		}
	}
	return ret
}

