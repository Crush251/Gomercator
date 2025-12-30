package algorithms

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	hw "gomercator/handlware"
)

// ==================== 配置结构体 ====================

// RelayStrategyConfig 转发策略完整配置
type RelayStrategyConfig struct {
	// 冗余模型参数
	WMode            string  // "EWMA" 或 "SlidingWindow"（默认 EWMA）
	RhoE             float64 // 早到性 EWMA 系数（默认 0.1）
	RhoF             float64 // 转发可观测率 EWMA 系数（默认 0.05）
	Calibration      string  // "linear" 或 "sigmoid"
	LinearA          float64 // linear 校准参数 a
	LinearB          float64 // linear 校准参数 b
	SigmoidAlpha     float64 // sigmoid 校准参数 alpha
	SigmoidMu        float64 // sigmoid 校准参数 mu
	GammaSender      float64 // 来源邻居条件修正系数 γ（默认 0.2）
	DeltaObs         float64 // 可观测偏差修正系数 δ（默认 0.2）
	NeutralPrior     float64 // 中性先验（默认 0.5）
	FreshnessEnabled bool    // 新鲜度回拉开关（默认 false）
	TauFresh         float64 // 新鲜度时间常数（秒，默认 600）

	// 转发策略参数
	D                  int     // 全局扇出上限
	EtaRand            float64 // 随机兜底比例 η（默认 0.1）
	MinCrossPerCluster int     // 跨簇最小配额（默认 1）
	PreferCrossCluster bool    // 优先跨簇（默认 true）
	SelfClusterCap     int     // 本簇最多选多少（默认 D/2）
	TruncatePolicy     string  // "keep_random" 或 "global_sort_all"（默认 keep_random）

	// 拓扑变化适配参数
	TopologyAdaptEnabled         bool    // 拓扑适配开关（默认 false）
	ChurnWindowSec               float64 // churn 检测窗口（秒）
	ChurnThreshold               float64 // churn 阈值（例如 0.2）
	ClusterChangeTriggersRelearn bool    // 簇变化触发 relearn（默认 false）
	RelearnDurationSec           float64 // relearn 模式持续时间（秒）
	RhoMultiplierInRelearn       float64 // relearn 时 rho 倍数（例如 3.0）
	EtaRandInRelearn             float64 // relearn 时随机兜底比例（默认 0.2）
	ExpireEnabled                bool    // 过期淘汰开关（默认 true）
	ExpireSec                    float64 // 过期时间（秒，默认 1800）

	// 消息收集窗口
	ArrivalCollectionWindow float64 // 消息到达收集窗口（秒，默认 0.1）
}

// NewDefaultRelayStrategyConfig 创建默认配置
func NewDefaultRelayStrategyConfig() *RelayStrategyConfig {
	return &RelayStrategyConfig{
		WMode:                        "EWMA",
		RhoE:                         0.1,
		RhoF:                         0.05,
		Calibration:                  "linear",
		LinearA:                      0.5,
		LinearB:                      0.3,
		SigmoidAlpha:                 5.0,
		SigmoidMu:                    0.5,
		GammaSender:                  0.2,
		DeltaObs:                     0.2,
		NeutralPrior:                 0.5,
		FreshnessEnabled:             false,
		TauFresh:                     600.0,
		D:                            16,
		EtaRand:                      0.1,
		MinCrossPerCluster:           1,
		PreferCrossCluster:           true,
		SelfClusterCap:               8, // D/2
		TruncatePolicy:               "keep_random",
		TopologyAdaptEnabled:         false,
		ChurnWindowSec:               60.0,
		ChurnThreshold:               0.2,
		ClusterChangeTriggersRelearn: false,
		RelearnDurationSec:           300.0,
		RhoMultiplierInRelearn:       3.0,
		EtaRandInRelearn:             0.2,
		ExpireEnabled:                true,
		ExpireSec:                    1800.0,
		ArrivalCollectionWindow:      0.1,
	}
}

// ==================== 核心数据结构 ====================

// NeighborStats 单个邻居的统计信息
type NeighborStats struct {
	EBar         float64      // 早到性 EWMA [0,1]
	FObs         float64      // 可观测转发率 EWMA [0,1]
	LastUpdate   time.Time    // 最后更新时间
	MessageRanks []RankRecord // 历史消息排名记录（窗口模式）
}

// RankRecord 消息排名记录
type RankRecord struct {
	TxID        string
	Rank        int
	Score       float64
	ArrivalTime time.Time
}

// NodeRelayState 单个节点的转发状态
type NodeRelayState struct {
	NodeID         int
	ClusterID      int
	Peers          []int                  // 邻居集合
	Stats          map[int]*NeighborStats // 每个邻居的统计
	Config         *RelayStrategyConfig
	InRelearnMode  bool
	RelearnEndTime time.Time
	PeersHistory   [][]int // churn检测窗口（最近N个时间点的peers）
	LastClusterID  int     // 上次的clusterID（用于检测变化）
}

// TransactionMessage 交易消息
type TransactionMessage struct {
	TxID       string
	SourceNode int
	Timestamp  time.Time
	SeenBy     map[int]time.Time // 记录哪些节点何时收到
	Arrivals   map[int]time.Time // 从各邻居到达的时间（用于rank计算）
}

// NewTransactionMessage 创建新的交易消息
func NewTransactionMessage(txID string, sourceNode int) *TransactionMessage {
	return &TransactionMessage{
		TxID:       txID,
		SourceNode: sourceNode,
		Timestamp:  time.Now(),
		SeenBy:     make(map[int]time.Time),
		Arrivals:   make(map[int]time.Time),
	}
}

// NewNodeRelayState 创建新的节点转发状态
func NewNodeRelayState(nodeID int, clusterID int, peers []int, config *RelayStrategyConfig) *NodeRelayState {
	if config == nil {
		config = NewDefaultRelayStrategyConfig()
	}

	stats := make(map[int]*NeighborStats)
	for _, peerID := range peers {
		stats[peerID] = &NeighborStats{
			EBar:         config.NeutralPrior,
			FObs:         config.NeutralPrior,
			LastUpdate:   time.Now(),
			MessageRanks: make([]RankRecord, 0),
		}
	}

	return &NodeRelayState{
		NodeID:        nodeID,
		ClusterID:     clusterID,
		Peers:         peers,
		Stats:         stats,
		Config:        config,
		InRelearnMode: false,
		PeersHistory:  make([][]int, 0),
		LastClusterID: clusterID,
	}
}

// ==================== 辅助函数 ====================

// clipProbability 裁剪概率到 [0,1]
func clipProbability(p float64) float64 {
	if p < 0.0 {
		return 0.0
	}
	if p > 1.0 {
		return 1.0
	}
	return p
}

// calibrateLinear 线性校准
func calibrateLinear(eBar, a, b float64) float64 {
	return clipProbability(a*eBar + b)
}

// calibrateSigmoid Sigmoid 校准
func calibrateSigmoid(eBar, alpha, mu float64) float64 {
	expArg := alpha * (eBar - mu)
	return clipProbability(1.0 / (1.0 + math.Exp(-expArg)))
}

// selectRandomSubset 从集合中随机选择指定数量的元素
func selectRandomSubset(candidates []int, count int) []int {
	if count <= 0 {
		return []int{}
	}
	if count >= len(candidates) {
		return candidates
	}

	selected := make([]int, count)
	indices := rand.Perm(len(candidates))
	for i := 0; i < count; i++ {
		selected[i] = candidates[indices[i]]
	}
	return selected
}

// partitionByCluster 按簇分组邻居
func partitionByCluster(peers []int, clusterIDs map[int]int, selfClusterID int) (sameCluster []int, otherClusters map[int][]int) {
	otherClusters = make(map[int][]int)
	sameCluster = make([]int, 0)

	for _, peerID := range peers {
		peerClusterID, exists := clusterIDs[peerID]
		if !exists {
			peerClusterID = -1 // 未知簇
		}

		if peerClusterID == selfClusterID {
			sameCluster = append(sameCluster, peerID)
		} else {
			if otherClusters[peerClusterID] == nil {
				otherClusters[peerClusterID] = make([]int, 0)
			}
			otherClusters[peerClusterID] = append(otherClusters[peerClusterID], peerID)
		}
	}

	return sameCluster, otherClusters
}

// computeRanks 计算消息到达排名
func computeRanks(arrivals map[int]time.Time) map[int]int {
	// 按到达时间排序
	type arrivalPair struct {
		peerID int
		time   time.Time
	}

	pairs := make([]arrivalPair, 0, len(arrivals))
	for peerID, t := range arrivals {
		pairs = append(pairs, arrivalPair{peerID: peerID, time: t})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].time.Before(pairs[j].time)
	})

	// 分配排名（相同时间相同排名）
	ranks := make(map[int]int)
	currentRank := 1
	for i := 0; i < len(pairs); i++ {
		if i > 0 && pairs[i].time.Sub(pairs[i-1].time) > time.Millisecond*10 {
			// 时间差超过10ms，排名递增
			currentRank = i + 1
		}
		ranks[pairs[i].peerID] = currentRank
	}

	return ranks
}

// ==================== 模块 A: 聚类集成 ====================

// ComputeClusterAssignments 基于稳定节点计算簇分配
func ComputeClusterAssignments(states []*hw.VivaldiPlusPlusState, k int) map[int]int {
	n := len(states)
	clusterIDs := make(map[int]int)

	// 提取所有节点的坐标（从 states 中）
	allModels := make([]*hw.VivaldiModel, n)
	for i := 0; i < n; i++ {
		if states[i] != nil && states[i].Coord != nil {
			allModels[i] = &hw.VivaldiModel{
				NodeID:         i,
				LocalCoord:     states[i].Coord,
				RandomPeerSet:  make([]int, 0),
				HaveEnoughPeer: false,
			}
		} else {
			// 如果状态不存在，创建默认模型
			allModels[i] = &hw.VivaldiModel{
				NodeID:         i,
				LocalCoord:     hw.NewVivaldiCoordinate(3),
				RandomPeerSet:  make([]int, 0),
				HaveEnoughPeer: false,
			}
		}
	}

	// 调用 KMeansVirtual 进行聚类
	clusterResult := hw.KMeansVirtual(allModels, k, 100, time.Now().UnixNano())

	// 构建 nodeID -> clusterID 映射
	for i := 0; i < n; i++ {
		clusterIDs[i] = clusterResult.ClusterID[i]
	}

	return clusterIDs
}

// ==================== 模块 B: 冗余模型 ====================

// ComputeRelayProbability 计算转发概率 P_ij
func ComputeRelayProbability(
	stats *NeighborStats,
	senderStats *NeighborStats, // 消息来源邻居的统计（可为nil）
	config *RelayStrategyConfig,
	currentTime time.Time,
) float64 {
	// 1. 基础概率校准
	var pBase float64
	if config.Calibration == "sigmoid" {
		pBase = calibrateSigmoid(stats.EBar, config.SigmoidAlpha, config.SigmoidMu)
	} else {
		// 默认 linear
		pBase = calibrateLinear(stats.EBar, config.LinearA, config.LinearB)
	}

	// 2. 来源条件修正
	pCond := pBase
	if senderStats != nil {
		pCond = clipProbability(pBase + config.GammaSender*(senderStats.EBar-0.5))
	}

	// 3. 可观测偏差修正
	p := clipProbability(pCond + config.DeltaObs*(1.0-stats.FObs))

	// 4. 新鲜度回拉（可选）
	if config.FreshnessEnabled {
		timeSinceUpdate := currentTime.Sub(stats.LastUpdate).Seconds()
		if timeSinceUpdate > 0 {
			wFresh := math.Exp(-timeSinceUpdate / config.TauFresh)
			p = wFresh*p + (1.0-wFresh)*config.NeutralPrior
		}
	}

	return clipProbability(p)
}

// ==================== 模块 C: 转发选择 ====================

// SelectRelays 选择转发列表
func SelectRelays(
	state *NodeRelayState,
	msg *TransactionMessage,
	sourceNeighbor int,
	allClusterIDs map[int]int,
) []int {
	config := state.Config

	// 候选邻居（排除来源）
	candidates := make([]int, 0)
	for _, peerID := range state.Peers {
		if peerID != sourceNeighbor {
			candidates = append(candidates, peerID)
		}
	}

	if len(candidates) == 0 {
		return []int{}
	}

	// 获取来源邻居的统计（用于概率计算）
	senderStats := state.Stats[sourceNeighbor]

	// A) 计算所有候选邻居的概率 P_ij
	probMap := make(map[int]float64)
	for _, peerID := range candidates {
		stats := state.Stats[peerID]
		if stats == nil {
			// 初始化统计
			stats = &NeighborStats{
				EBar:       config.NeutralPrior,
				FObs:       config.NeutralPrior,
				LastUpdate: time.Now(),
			}
			state.Stats[peerID] = stats
		}
		probMap[peerID] = ComputeRelayProbability(stats, senderStats, config, time.Now())
	}

	// B) 随机兜底
	etaRand := config.EtaRand
	if state.InRelearnMode {
		etaRand = config.EtaRandInRelearn
	}
	dRand := int(math.Ceil(etaRand * float64(config.D)))
	if dRand > len(candidates) {
		dRand = len(candidates)
	}
	L_rand := selectRandomSubset(candidates, dRand)

	// C) 跨簇最小配额
	L_cross := make([]int, 0)
	sameCluster, otherClusters := partitionByCluster(candidates, allClusterIDs, state.ClusterID)

	// 对每个异簇，按 P 升序取 min_cross_per_cluster 个
	for _, clusterPeers := range otherClusters {
		// 按概率排序（升序，优先选概率小的，保证跨簇扩散）
		sort.Slice(clusterPeers, func(i, j int) bool {
			return probMap[clusterPeers[i]] < probMap[clusterPeers[j]]
		})

		count := config.MinCrossPerCluster
		if count > len(clusterPeers) {
			count = len(clusterPeers)
		}
		L_cross = append(L_cross, clusterPeers[:count]...)
	}

	// D) 分簇配额补齐
	L_relay := make([]int, 0)
	used := make(map[int]bool)
	for _, peerID := range L_rand {
		used[peerID] = true
	}
	for _, peerID := range L_cross {
		used[peerID] = true
	}

	D_remaining := config.D - len(L_rand) - len(L_cross)
	if D_remaining > 0 {
		// 优先异簇，再本簇
		remainingOther := make([]int, 0)
		for _, clusterPeers := range otherClusters {
			for _, peerID := range clusterPeers {
				if !used[peerID] {
					remainingOther = append(remainingOther, peerID)
				}
			}
		}

		// 按概率升序排序
		sort.Slice(remainingOther, func(i, j int) bool {
			return probMap[remainingOther[i]] < probMap[remainingOther[j]]
		})

		// 添加异簇节点
		for _, peerID := range remainingOther {
			if len(L_relay) >= D_remaining {
				break
			}
			L_relay = append(L_relay, peerID)
			used[peerID] = true
		}

		// 如果还有剩余，添加本簇节点（受 self_cluster_cap 约束）
		selfClusterCount := 0
		for _, peerID := range sameCluster {
			if !used[peerID] && selfClusterCount < config.SelfClusterCap {
				if len(L_relay) >= D_remaining {
					break
				}
				L_relay = append(L_relay, peerID)
				used[peerID] = true
				selfClusterCount++
			}
		}
	}

	// E) 最终裁剪到 D
	L_all := make([]int, 0)
	L_all = append(L_all, L_rand...)
	L_all = append(L_all, L_cross...)
	L_all = append(L_all, L_relay...)

	// 去重
	uniqueMap := make(map[int]bool)
	L_unique := make([]int, 0)
	for _, peerID := range L_all {
		if !uniqueMap[peerID] {
			L_unique = append(L_unique, peerID)
			uniqueMap[peerID] = true
		}
	}

	if len(L_unique) <= config.D {
		return L_unique
	}

	// 裁剪策略
	if config.TruncatePolicy == "keep_random" {
		// 优先保留随机兜底，再从其他按P升序取
		result := make([]int, 0)
		result = append(result, L_rand...)
		remaining := make([]int, 0)
		for _, peerID := range L_cross {
			if !hw.Contains(result, peerID) {
				remaining = append(remaining, peerID)
			}
		}
		for _, peerID := range L_relay {
			if !hw.Contains(result, peerID) {
				remaining = append(remaining, peerID)
			}
		}
		sort.Slice(remaining, func(i, j int) bool {
			return probMap[remaining[i]] < probMap[remaining[j]]
		})
		for _, peerID := range remaining {
			if len(result) >= config.D {
				break
			}
			result = append(result, peerID)
		}
		return result
	} else {
		// global_sort_all: 全部按P排序取前D个
		sort.Slice(L_unique, func(i, j int) bool {
			return probMap[L_unique[i]] < probMap[L_unique[j]]
		})
		if len(L_unique) > config.D {
			return L_unique[:config.D]
		}
		return L_unique
	}
}

// ==================== 模块 D: 统计更新 ====================

// UpdateNeighborStats 更新邻居统计
func UpdateNeighborStats(
	state *NodeRelayState,
	msg *TransactionMessage,
	arrivals map[int]time.Time,
) {
	config := state.Config

	// 计算排名
	ranks := computeRanks(arrivals)
	if len(ranks) == 0 {
		return
	}

	// 计算归一化早到分数并更新 EWMA
	for peerID, rank := range ranks {
		stats := state.Stats[peerID]
		if stats == nil {
			stats = &NeighborStats{
				EBar:       config.NeutralPrior,
				FObs:       config.NeutralPrior,
				LastUpdate: time.Now(),
			}
			state.Stats[peerID] = stats
		}

		// 归一化早到分数
		k := len(ranks)
		var score float64
		if k > 1 {
			score = 1.0 - float64(rank-1)/float64(k-1)
		} else {
			score = 1.0
		}

		// EWMA 更新
		rho := config.RhoE
		if state.InRelearnMode {
			rho *= config.RhoMultiplierInRelearn
		}
		stats.EBar = rho*score + (1.0-rho)*stats.EBar
		stats.EBar = clipProbability(stats.EBar)

		// 记录排名（用于窗口模式，如果启用）
		if config.WMode == "SlidingWindow" {
			stats.MessageRanks = append(stats.MessageRanks, RankRecord{
				TxID:        msg.TxID,
				Rank:        rank,
				Score:       score,
				ArrivalTime: time.Now(),
			})
			// 保持窗口大小（例如最近100条）
			if len(stats.MessageRanks) > 100 {
				stats.MessageRanks = stats.MessageRanks[1:]
			}
		}

		stats.LastUpdate = time.Now()
	}

	// 更新可观测转发率（对来源邻居）
	// 这里简化：假设消息来源是第一个到达的邻居
	if len(arrivals) > 0 {
		// 找到最早到达的邻居（可能是来源）
		earliestPeer := -1
		earliestTime := time.Now()
		for peerID, t := range arrivals {
			if t.Before(earliestTime) {
				earliestTime = t
				earliestPeer = peerID
			}
		}

		if earliestPeer >= 0 {
			stats := state.Stats[earliestPeer]
			if stats != nil {
				rho := config.RhoF
				if state.InRelearnMode {
					rho *= config.RhoMultiplierInRelearn
				}
				// 来源邻居：y=1
				stats.FObs = rho*1.0 + (1.0-rho)*stats.FObs
				stats.FObs = clipProbability(stats.FObs)
			}

			// 其他邻居：慢衰减（可选）
			for peerID, stats := range state.Stats {
				if peerID != earliestPeer {
					rho := config.RhoF * 0.1 // 更慢的衰减
					stats.FObs = rho*0.0 + (1.0-rho)*stats.FObs
				}
			}
		}
	}
}

// ==================== 模块 E: 拓扑适配 ====================

// CheckAndUpdateTopology 检查并更新拓扑适配
func CheckAndUpdateTopology(state *NodeRelayState, currentTime time.Time, currentClusterID int) bool {
	config := state.Config
	if !config.TopologyAdaptEnabled {
		return false
	}

	changed := false

	// 检查 relearn 模式是否过期
	if state.InRelearnMode && currentTime.After(state.RelearnEndTime) {
		ExitRelearnMode(state)
		changed = true
	}

	// Churn 检测
	if len(state.PeersHistory) > 0 {
		// 计算最近窗口内的变化
		oldPeers := state.PeersHistory[0]
		currentPeers := state.Peers
		churn := computeChurnRate(oldPeers, currentPeers)
		if churn > config.ChurnThreshold {
			EnterRelearnMode(state, time.Duration(config.RelearnDurationSec)*time.Second)
			changed = true
		}
	}

	// 簇变化检测
	if config.ClusterChangeTriggersRelearn && currentClusterID != state.LastClusterID {
		EnterRelearnMode(state, time.Duration(config.RelearnDurationSec)*time.Second)
		state.LastClusterID = currentClusterID
		changed = true
	}

	// 过期处理
	if config.ExpireEnabled {
		for _, stats := range state.Stats {
			timeSinceUpdate := currentTime.Sub(stats.LastUpdate).Seconds()
			if timeSinceUpdate > config.ExpireSec {
				// 回拉到中性先验
				stats.EBar = config.NeutralPrior
				stats.FObs = config.NeutralPrior
			}
		}
	}

	return changed
}

// computeChurnRate 计算 churn 率
func computeChurnRate(oldPeers, newPeers []int) float64 {
	if len(oldPeers) == 0 {
		return 0.0
	}

	oldSet := make(map[int]bool)
	for _, p := range oldPeers {
		oldSet[p] = true
	}

	newSet := make(map[int]bool)
	for _, p := range newPeers {
		newSet[p] = true
	}

	// 计算对称差集大小
	diffCount := 0
	for p := range oldSet {
		if !newSet[p] {
			diffCount++
		}
	}
	for p := range newSet {
		if !oldSet[p] {
			diffCount++
		}
	}

	return float64(diffCount) / float64(len(oldPeers))
}

// EnterRelearnMode 进入 relearn 模式
func EnterRelearnMode(state *NodeRelayState, duration time.Duration) {
	state.InRelearnMode = true
	state.RelearnEndTime = time.Now().Add(duration)

	// 向中性先验轻度回拉（可选）
	kappa := 0.1 // 回拉系数
	for _, stats := range state.Stats {
		stats.EBar = (1.0-kappa)*stats.EBar + kappa*state.Config.NeutralPrior
		stats.FObs = (1.0-kappa)*stats.FObs + kappa*state.Config.NeutralPrior
	}
}

// ExitRelearnMode 退出 relearn 模式
func ExitRelearnMode(state *NodeRelayState) {
	state.InRelearnMode = false
}

// ==================== 模块 F: 预热系统 ====================

// WarmupSimulation 预热仿真（100轮 × 200交易）
func WarmupSimulation(
	coords []hw.LatLonCoordinate,
	states []*hw.VivaldiPlusPlusState,
	clusterIDs map[int]int,
	config *RelayStrategyConfig,
	rounds int,
	txPerRound int,
) []*NodeRelayState {
	n := len(coords)
	fmt.Printf("开始预热仿真：%d轮 × %d交易/轮\n", rounds, txPerRound)

	// 初始化所有节点的转发状态
	relayStates := make([]*NodeRelayState, n)
	for i := 0; i < n; i++ {
		// 构建邻居列表（从 Vivaldi++ 状态中获取）
		peers := make([]int, 0)
		// 简化：使用随机邻居（实际应该从稳定集合或网络拓扑获取）
		for j := 0; j < 20; j++ {
			peerID := rand.Intn(n)
			if peerID != i && !hw.Contains(peers, peerID) {
				peers = append(peers, peerID)
			}
		}

		clusterID := clusterIDs[i]
		relayStates[i] = NewNodeRelayState(i, clusterID, peers, config)
	}

	// 预热循环
	txCounter := 0
	for round := 0; round < rounds; round++ {
		if round%10 == 0 {
			fmt.Printf("  预热轮次 %d/%d\n", round, rounds)
		}

		for tx := 0; tx < txPerRound; tx++ {
			// 随机选择源节点
			sourceNode := rand.Intn(n)

			// 创建交易消息
			txID := fmt.Sprintf("warmup_tx_%d_%d", round, tx)
			msg := NewTransactionMessage(txID, sourceNode)

			// 模拟消息传播（事件驱动）
			simulateMessagePropagation(relayStates, msg, coords, clusterIDs, config)

			txCounter++
		}
	}

	fmt.Printf("预热完成，共处理 %d 笔交易\n", txCounter)
	return relayStates
}

// simulateMessagePropagation 模拟单条消息的传播
func simulateMessagePropagation(
	relayStates []*NodeRelayState,
	msg *TransactionMessage,
	coords []hw.LatLonCoordinate,
	clusterIDs map[int]int,
	config *RelayStrategyConfig,
) {
	n := len(relayStates)
	//msgQueue := handlware.NewPriorityQueue()

	// 初始化：源节点收到消息
	sourceNode := msg.SourceNode
	msg.SeenBy[sourceNode] = time.Now()
	msg.Arrivals[sourceNode] = time.Now()

	// 源节点选择转发列表
	relayList := SelectRelays(relayStates[sourceNode], msg, -1, clusterIDs)
	for _, peerID := range relayList {
		// 计算传播延迟
		delay := hw.Distance(coords[sourceNode], coords[peerID]) + hw.FixedDelay
		arrivalTime := time.Now().Add(time.Duration(delay) * time.Millisecond)
		msg.Arrivals[peerID] = arrivalTime
		// 这里简化：直接记录到达时间，实际应该用事件队列
	}

	// 事件驱动传播（简化版）
	processed := make(map[int]bool)
	processed[sourceNode] = true

	// 收集到达时间窗口内的所有到达
	collectionWindow := time.Duration(config.ArrivalCollectionWindow * float64(time.Second))
	_ = collectionWindow // 用于后续扩展
	windowEnd := time.Now().Add(collectionWindow)

	// 模拟传播（简化：直接处理所有转发）
	for _, relayNodeID := range relayList {
		if processed[relayNodeID] {
			continue
		}

		// 节点收到消息
		msg.SeenBy[relayNodeID] = time.Now()
		processed[relayNodeID] = true

		// 更新统计（收集窗口内的到达）
		arrivals := make(map[int]time.Time)
		for peerID, t := range msg.Arrivals {
			if t.Before(windowEnd) {
				arrivals[peerID] = t
			}
		}
		UpdateNeighborStats(relayStates[relayNodeID], msg, arrivals)

		// 选择转发列表
		sourceNeighbor := sourceNode // 简化：假设来自源节点
		newRelayList := SelectRelays(relayStates[relayNodeID], msg, sourceNeighbor, clusterIDs)

		// 继续传播（限制深度避免无限循环）
		if len(msg.SeenBy) < n/10 { // 限制传播范围
			for _, nextPeerID := range newRelayList {
				if !processed[nextPeerID] {
					delay := hw.Distance(coords[relayNodeID], coords[nextPeerID]) + hw.FixedDelay
					arrivalTime := time.Now().Add(time.Duration(delay) * time.Millisecond)
					msg.Arrivals[nextPeerID] = arrivalTime
				}
			}
		}
	}
}

// ==================== 模块 G: 主仿真系统 ====================

// RelaySimulationResult 仿真结果
type RelaySimulationResult struct {
	// 覆盖率指标
	AvgCoverage float64
	Coverage95  float64

	// 冗余指标
	AvgRedundancy  float64 // 平均每个节点收到重复消息次数
	RedundancyRate float64 // 冗余消息占比

	// 延迟指标
	AvgLatency float64
	Latency95  float64

	// 转发策略指标
	AvgRelaySize     float64 // 平均转发列表大小
	CrossClusterRate float64 // 跨簇转发比例
	RandomPickRate   float64 // 随机兜底命中率

	// 概率分布指标
	ProbMean   float64
	ProbMedian float64
	ProbP95    float64

	// 拓扑适配指标（可选）
	RelearnTriggers int
	ChurnDetections int
}

// SimulateVivaldiPlusPlusRelay 完整的仿真入口
func SimulateVivaldiPlusPlusRelay(
	coords []hw.LatLonCoordinate,
	rounds int,
	vivaldiConfig *hw.VivaldiPlusPlusConfig,
	relayConfig *RelayStrategyConfig,
	warmupRounds int,
	txPerRound int,
) *RelaySimulationResult {
	fmt.Println("========== Vivaldi++ 传播策略仿真 ==========")

	// 1. 生成 Vivaldi++ 坐标
	fmt.Println("步骤 1/5: 生成 Vivaldi++ 坐标...")
	models := hw.GenerateVirtualCoordinatePlusPlus(coords, rounds, vivaldiConfig)
	states := make([]*hw.VivaldiPlusPlusState, len(models))
	// 注意：这里需要从 models 重建 states，简化处理
	// 实际应该保存 GenerateVirtualCoordinatePlusPlus 返回的 states
	fmt.Println("坐标生成完成")

	// 2. 提取稳定节点并聚类
	fmt.Println("步骤 2/5: 提取稳定节点并聚类...")
	// 简化：直接使用所有节点进行聚类
	k := 8 // 默认簇数
	clusterIDs := ComputeClusterAssignments(states, k)
	fmt.Printf("聚类完成，共 %d 个簇\n", k)

	// 3. 预热阶段
	fmt.Println("步骤 3/5: 预热阶段...")
	relayStates := WarmupSimulation(coords, states, clusterIDs, relayConfig, warmupRounds, txPerRound)
	fmt.Println("预热完成")

	// 4. 正式仿真阶段（简化：这里只做统计收集）
	fmt.Println("步骤 4/5: 正式仿真阶段...")
	result := collectSimulationMetrics(relayStates, clusterIDs)
	fmt.Println("仿真完成")

	// 5. 输出结果
	fmt.Println("步骤 5/5: 输出结果...")
	printSimulationResult(result)

	return result
}

// collectSimulationMetrics 收集仿真指标
func collectSimulationMetrics(relayStates []*NodeRelayState, clusterIDs map[int]int) *RelaySimulationResult {
	result := &RelaySimulationResult{}

	// 收集概率分布
	probs := make([]float64, 0)
	totalRelaySize := 0
	totalRelays := 0
	crossClusterCount := 0
	totalRelayCount := 0

	for _, state := range relayStates {
		for peerID, stats := range state.Stats {
			prob := ComputeRelayProbability(stats, nil, state.Config, time.Now())
			probs = append(probs, prob)

			// 检查是否跨簇
			peerClusterID, exists := clusterIDs[peerID]
			if exists && peerClusterID != state.ClusterID {
				crossClusterCount++
			}
			totalRelayCount++
		}

		// 平均转发列表大小（使用邻居数作为近似）
		totalRelaySize += len(state.Peers)
		totalRelays++
	}

	// 计算统计值
	if len(probs) > 0 {
		sort.Float64s(probs)
		sum := 0.0
		for _, p := range probs {
			sum += p
		}
		result.ProbMean = sum / float64(len(probs))
		result.ProbMedian = probs[len(probs)/2]
		result.ProbP95 = probs[int(float64(len(probs))*0.95)]
	}

	if totalRelays > 0 {
		result.AvgRelaySize = float64(totalRelaySize) / float64(totalRelays)
	}

	if totalRelayCount > 0 {
		result.CrossClusterRate = float64(crossClusterCount) / float64(totalRelayCount)
	}

	return result
}

// printSimulationResult 打印仿真结果
func printSimulationResult(result *RelaySimulationResult) {
	fmt.Println("\n========== 仿真结果 ==========")
	fmt.Printf("概率分布:\n")
	fmt.Printf("  均值: %.4f\n", result.ProbMean)
	fmt.Printf("  中位数: %.4f\n", result.ProbMedian)
	fmt.Printf("  95分位: %.4f\n", result.ProbP95)
	fmt.Printf("平均转发列表大小: %.2f\n", result.AvgRelaySize)
	fmt.Printf("跨簇转发比例: %.2f%%\n", result.CrossClusterRate*100)
}

// ==================== Algorithm 接口实现 ====================

// VivaldiPlusPlusRelay Vivaldi++ 传播策略算法实现
type VivaldiPlusPlusRelay struct {
	hw.BaseAlgorithm
	Coords         []hw.LatLonCoordinate
	VivaldiStates  []*hw.VivaldiPlusPlusState
	RelayStates    []*NodeRelayState
	ClusterIDs     map[int]int
	Config         *RelayStrategyConfig
	VivaldiConfig  *hw.VivaldiPlusPlusConfig
	MessageHistory map[int]map[string]time.Time // 节点ID -> (TxID -> 首次到达时间)
	ArrivalHistory map[string]map[int]time.Time // TxID -> (节点ID -> 到达时间)
}

// NewVivaldiPlusPlusRelay 创建新的 Vivaldi++ 传播策略算法实例
func NewVivaldiPlusPlusRelay(
	n int,
	coords []hw.LatLonCoordinate,
	vivaldiConfig *hw.VivaldiPlusPlusConfig,
	relayConfig *RelayStrategyConfig,
	warmupRounds int,
	txPerRound int,
) *VivaldiPlusPlusRelay {
	if vivaldiConfig == nil {
		vivaldiConfig = hw.NewVivaldiPlusPlusConfig()
	}
	if relayConfig == nil {
		relayConfig = NewDefaultRelayStrategyConfig()
	}

	// 生成 Vivaldi++ 坐标
	fmt.Println("生成 Vivaldi++ 坐标...")
	models := hw.GenerateVirtualCoordinatePlusPlus(coords, 100, vivaldiConfig)

	// 从 models 重建 states（简化处理）
	// 注意：这里无法获取完整的 VivaldiPlusPlusState，只能使用坐标
	// 实际应用中应该修改 GenerateVirtualCoordinatePlusPlus 返回完整 states
	states := make([]*hw.VivaldiPlusPlusState, n)
	for i := 0; i < n; i++ {
		// 创建最小状态（只包含坐标）
		// 注意：VivaldiPlusPlusState 的字段可能未导出，这里简化处理
		states[i] = &hw.VivaldiPlusPlusState{
			NodeID: i,
			Coord:  models[i].LocalCoord,
		}
	}

	// 聚类
	fmt.Println("进行 K-means 聚类...")
	k := 8
	clusterIDs := ComputeClusterAssignments(states, k)

	// 预热
	fmt.Printf("预热阶段：%d轮 × %d交易/轮...\n", warmupRounds, txPerRound)
	relayStates := WarmupSimulation(coords, states, clusterIDs, relayConfig, warmupRounds, txPerRound)
	fmt.Println("预热完成")

	// 构建网络图（用于兼容 Algorithm 接口）
	graph := hw.NewGraph(n)
	for i := 0; i < n; i++ {
		for _, peerID := range relayStates[i].Peers {
			graph.AddEdge(i, peerID)
		}
	}

	return &VivaldiPlusPlusRelay{
		BaseAlgorithm: hw.BaseAlgorithm{
			Name:          "Vivaldi++ Relay",
			SpecifiedRoot: false,
			Graph:         graph,
			Coords:        coords,
			Root:          0,
		},
		Coords:         coords,
		VivaldiStates:  states,
		RelayStates:    relayStates,
		ClusterIDs:     clusterIDs,
		Config:         relayConfig,
		VivaldiConfig:  vivaldiConfig,
		MessageHistory: make(map[int]map[string]time.Time),
		ArrivalHistory: make(map[string]map[int]time.Time),
	}
}

// Respond 实现 Algorithm 接口
func (v *VivaldiPlusPlusRelay) Respond(msg *hw.Message) []int {
	nodeID := msg.Dst
	sourceNode := msg.Src

	// 获取节点状态
	if nodeID >= len(v.RelayStates) {
		return []int{}
	}
	state := v.RelayStates[nodeID]

	// 创建交易消息（使用消息的 Root 和 Step 作为 TxID）
	txID := fmt.Sprintf("msg_%d_%d", msg.Root, msg.Step)

	// 检查是否已见过此消息（使用模拟时间）
	// 注意：在仿真中，RecvTime 是模拟时间（毫秒），需要转换为 time.Time
	// 简化处理：使用 RecvTime 作为相对时间戳
	recvTime := time.Unix(0, int64(msg.RecvTime*1e6)) // 将毫秒转换为纳秒

	if v.MessageHistory[nodeID] == nil {
		v.MessageHistory[nodeID] = make(map[string]time.Time)
	}
	if firstSeen, seen := v.MessageHistory[nodeID][txID]; seen {
		// 重复消息，不转发
		// 但如果这是更早的到达，更新到达时间（用于 rank 计算）
		if recvTime.Before(firstSeen) {
			v.MessageHistory[nodeID][txID] = recvTime
			if v.ArrivalHistory[txID] != nil {
				v.ArrivalHistory[txID][nodeID] = recvTime
			}
		}
		return []int{}
	}

	// 记录首次到达时间
	v.MessageHistory[nodeID][txID] = recvTime

	// 记录到达历史（用于 rank 计算）
	if v.ArrivalHistory[txID] == nil {
		v.ArrivalHistory[txID] = make(map[int]time.Time)
	}
	v.ArrivalHistory[txID][nodeID] = recvTime

	// 创建交易消息对象
	txMsg := &TransactionMessage{
		TxID:       txID,
		SourceNode: msg.Root,
		Timestamp:  recvTime,
		SeenBy:     make(map[int]time.Time),
		Arrivals:   v.ArrivalHistory[txID],
	}

	// 更新统计（收集窗口内的到达）
	collectionWindow := time.Duration(v.Config.ArrivalCollectionWindow * float64(time.Second))
	windowStart := recvTime.Add(-collectionWindow)
	arrivals := make(map[int]time.Time)
	for peerID, t := range v.ArrivalHistory[txID] {
		if t.After(windowStart) && t.Before(recvTime) || t.Equal(recvTime) {
			arrivals[peerID] = t
		}
	}
	if len(arrivals) > 0 {
		UpdateNeighborStats(state, txMsg, arrivals)
	}

	// 检查拓扑适配
	CheckAndUpdateTopology(state, recvTime, state.ClusterID)

	// 选择转发列表
	relayList := SelectRelays(state, txMsg, sourceNode, v.ClusterIDs)

	return relayList
}

// GetAlgoName 返回算法名称
func (v *VivaldiPlusPlusRelay) GetAlgoName() string {
	return "Vivaldi++ Relay"
}
