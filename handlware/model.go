package handlware

import (
	"math"
)

// ==================== 基础常量定义 ====================
const (
	K            = 8         // 聚类数量
	MaxDepth     = 40        // 最大传播深度
	FixedDelay   = 250.0     // 固定处理延迟（ms）
	RootFanout   = 64        // 根节点扇出度
	SecondFanout = 64        // 第二层扇出度
	Fanout       = 8         // 普通节点扇出度
	InnerDeg     = 4         // 簇内连接度
	MaxOutbound  = 8         // 最大出度
	EarthRadius  = 6371000.0 // 地球半径（m）
	Pi           = math.Pi

	// Mercator专用常量
	GeoBitsPerChar      = 5  // Geohash每个字符的比特数
	GeoPrecisionDefault = 4  // 默认Geohash精度
	K0BucketThreshold   = 15 // K0桶阈值
	KaryFactorDefault   = 3  // K-ary树分支因子
	MaxBucketSize       = 10 // K桶最大容量

	// 数据传输参数
	BandwidthDefault = 33000000.0 // 默认带宽（bps）
	DataSizeSmall    = 300.0      // 小数据包（Bytes）
	DataSizeLarge    = 1048576.0  // 大数据包（1MB）
)

// ==================== 坐标结构 ====================

// LatLonCoordinate 经纬度坐标
type LatLonCoordinate struct {
	Lat float64 // 纬度 [-90, 90]
	Lon float64 // 经度 [-180, 180]
}

// VivaldiCoordinate Vivaldi虚拟坐标
type VivaldiCoordinate struct {
	Vector []float64 // D维欧几里得向量
	Height float64   // 高度分量
	Error  float64   // 误差估计
}

// NewVivaldiCoordinate 创建新的Vivaldi坐标
func NewVivaldiCoordinate(dim int) *VivaldiCoordinate {
	return &VivaldiCoordinate{
		Vector: make([]float64, dim),
		Height: 0.0,
		Error:  0.1, // 初始误差
	}
}

// ==================== 消息结构 ====================

// Message 广播消息
type Message struct {
	Root     int     // 广播根节点ID
	Src      int     // 消息源节点ID
	Dst      int     // 目标节点ID
	Step     int     // 当前传播步数
	SendTime float64 // 发送时间（ms）
	RecvTime float64 // 接收时间（ms）
}

// NewMessage 创建新消息
func NewMessage(root, src, dst, step int, sendTime, recvTime float64) *Message {
	return &Message{
		Root:     root,
		Src:      src,
		Dst:      dst,
		Step:     step,
		SendTime: sendTime,
		RecvTime: recvTime,
	}
}

// ==================== 图结构 ====================

// Graph 网络拓扑图
type Graph struct {
	N        int     // 节点数
	M        int     // 边数
	InBound  [][]int // 入边列表 InBound[v] = [u1, u2, ...] 表示u1,u2,...指向v
	OutBound [][]int // 出边列表 OutBound[u] = [v1, v2, ...] 表示u指向v1,v2,...
}

// NewGraph 创建新图
func NewGraph(n int) *Graph {
	g := &Graph{
		N:        n,
		M:        0,
		InBound:  make([][]int, n),
		OutBound: make([][]int, n),
	}
	for i := 0; i < n; i++ {
		g.InBound[i] = make([]int, 0)
		g.OutBound[i] = make([]int, 0)
	}
	return g
}

// AddEdge 添加边 u -> v，返回是否成功添加（避免自环和重边）
func (g *Graph) AddEdge(u, v int) bool {
	// 避免自环
	if u == v {
		return false
	}

	// 避免重边
	for _, nb := range g.OutBound[u] {
		if nb == v {
			return false
		}
	}

	g.OutBound[u] = append(g.OutBound[u], v)
	g.InBound[v] = append(g.InBound[v], u)
	g.M++
	return true
}

// DelEdge 删除边 u -> v
func (g *Graph) DelEdge(u, v int) {
	// 从OutBound[u]中删除v
	for i, nb := range g.OutBound[u] {
		if nb == v {
			// 交换删除（快速删除）
			g.OutBound[u][i] = g.OutBound[u][len(g.OutBound[u])-1]
			g.OutBound[u] = g.OutBound[u][:len(g.OutBound[u])-1]
			break
		}
	}

	// 从InBound[v]中删除u
	for i, nb := range g.InBound[v] {
		if nb == u {
			g.InBound[v][i] = g.InBound[v][len(g.InBound[v])-1]
			g.InBound[v] = g.InBound[v][:len(g.InBound[v])-1]
			break
		}
	}

	g.M--
}

// Outbound 获取节点u的出边邻居
func (g *Graph) Outbound(u int) []int {
	return g.OutBound[u]
}

// Inbound 获取节点u的入边邻居
func (g *Graph) Inbound(u int) []int {
	return g.InBound[u]
}

// OutDegree 获取节点u的出度
func (g *Graph) OutDegree(u int) int {
	return len(g.OutBound[u])
}

// InDegree 获取节点u的入度
func (g *Graph) InDegree(u int) int {
	return len(g.InBound[u])
}

// ==================== 测试结果结构 ====================

// TestResult 模拟测试结果
type TestResult struct {
	AvgBandwidth      float64   // 平均带宽消耗（重复消息率）
	AvgLatency        float64   // 平均延迟（ms）
	Latency           []float64 // 延迟百分位数组 [5%, 10%, ..., 100%]
	DepthCDF          []float64 // 深度累积分布函数
	AvgDist           []float64 // 每层平均距离延迟
	ClusterAvgLatency []float64 // 每个簇的平均延迟
	ClusterAvgDepth   []float64 // 每个簇的平均深度
	SuccessChildren   [][]int   // 新增[u] => 成功（首次）把消息转发/传递到的子节点列表

}

// NewTestResult 创建新的测试结果
func NewTestResult(n int) *TestResult {
	return &TestResult{
		AvgBandwidth:      0,
		AvgLatency:        0,
		Latency:           make([]float64, 21), // 0.05, 0.10, ..., 1.00
		DepthCDF:          make([]float64, MaxDepth),
		AvgDist:           make([]float64, MaxDepth),
		ClusterAvgLatency: make([]float64, K),
		ClusterAvgDepth:   make([]float64, K),
		SuccessChildren:   make([][]int, n), //
	}
}

// ==================== 聚类结果结构 ====================

// ClusterResult K-means聚类结果
type ClusterResult struct {
	K           int     // 簇数量
	ClusterID   []int   // 每个节点所属簇ID ClusterID[nodeID] = clusterID
	ClusterList [][]int // 每个簇包含的节点列表 ClusterList[clusterID] = [node1, node2, ...]
	ClusterCnt  []int   // 每个簇的节点数 ClusterCnt[clusterID] = count
}

// NewClusterResult 创建新的聚类结果
func NewClusterResult(k, n int) *ClusterResult {
	return &ClusterResult{
		K:           k,
		ClusterID:   make([]int, n),
		ClusterList: make([][]int, k),
		ClusterCnt:  make([]int, k),
	}
}

// ==================== 攻击配置 ====================

// AttackConfig 攻击场景配置
type AttackConfig struct {
	MaliciousRatio float64 // 恶意节点比例（拒绝转发）
	NodeLeaveRatio float64 // 节点离开比例（接收但不转发）
	FakeCoordRatio float64 // 谎报坐标节点比例（Mercator专用）
}

// NewAttackConfig 创建默认攻击配置
func NewAttackConfig() *AttackConfig {
	return &AttackConfig{
		MaliciousRatio: 0.0,
		NodeLeaveRatio: 0.0,
		FakeCoordRatio: 0.0,
	}
}

// ==================== Vivaldi模型 ====================

// VivaldiModel Vivaldi虚拟坐标模型
type VivaldiModel struct {
	NodeID         int                // 节点ID
	LocalCoord     *VivaldiCoordinate // 本地虚拟坐标
	RandomPeerSet  []int              // 随机邻居集合
	HaveEnoughPeer bool               // 是否有足够的邻居
}

// NewVivaldiModel 创建新的Vivaldi模型
func NewVivaldiModel(nodeID, dim int) *VivaldiModel {
	return &VivaldiModel{
		NodeID:         nodeID,
		LocalCoord:     NewVivaldiCoordinate(dim),
		RandomPeerSet:  make([]int, 0),
		HaveEnoughPeer: false,
	}
}

// Coordinate 获取坐标
func (vm *VivaldiModel) Coordinate() *VivaldiCoordinate {
	return vm.LocalCoord
}

// Vector 获取向量部分（兼容C++代码）
func (vm *VivaldiModel) Vector() []float64 {
	return vm.LocalCoord.Vector
}

// ==================== Geohash相关结构（Mercator专用）====================

// GeoPrefixNode Geohash前缀树节点
type GeoPrefixNode struct {
	Prefix   string                  // 前缀字符串
	NodeIDs  []int                   // 包含该前缀的节点ID列表
	Children map[rune]*GeoPrefixNode // 子节点映射
}

// NewGeoPrefixNode 创建新的前缀树节点
func NewGeoPrefixNode(prefix string) *GeoPrefixNode {
	return &GeoPrefixNode{
		Prefix:   prefix,
		NodeIDs:  make([]int, 0),
		Children: make(map[rune]*GeoPrefixNode),
	}
}

// KaryMessage K-ary树消息信息（Mercator内部传播控制）
type KaryMessage struct {
	RootNode int  // k-ary树的根节点ID
	IsKary   bool // 是否使用k-ary树传播
}

// NewKaryMessage 创建新的K-ary消息
func NewKaryMessage(rootNode int, isKary bool) *KaryMessage {
	return &KaryMessage{
		RootNode: rootNode,
		IsKary:   isKary,
	}
}
