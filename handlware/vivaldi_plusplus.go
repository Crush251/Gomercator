package handlware

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"
)

// ==================== Vivaldi++ 常量定义 ====================

const (
	// 默认配置参数
	DefaultDim                 = 3     // 默认维度
	DefaultRTTWindow           = 15    // RTT历史窗口大小 M
	DefaultCoordWindow         = 15    // 坐标历史窗口大小 W
	DefaultRMin                = 20    // 最小轮数（阶段切换条件1）
	DefaultESwitch             = 0.15  // 误差切换门限
	DefaultS                   = 7     // 持续轮数（误差需持续满足S轮）
	DefaultBMin                = 3     // 最小稳定节点数
	DefaultP                   = 0.03  // 震荡阈值 2%
	DefaultE0                  = 0.15  // 稳定节点误差上限
	DefaultTau                 = 0.1   // λ裕量（可设为0）
	DefaultEpsMin              = 0.1   // 权重下限
	DefaultGamma               = 0.4   // 不稳定节点权重
	DefaultFc                  = 100.0 // 冻结上限（最大位移）
	DefaultAlpha               = 2.00  // λ衰减参数alpha
	DefaultAnnealRate          = 0.30  // 退火衰减率
	DefaultAnnealPeriod        = 5     // 退火周期（每T轮衰减一次）
	FixedNeighborSetSize       = 128   // 固定邻居集合大小
	NeighborSampleSizePerRound = 16    // 每轮从固定邻居中采样的数量
)

// ==================== 配置结构体 ====================

// VivaldiPlusPlusConfig Vivaldi++ 配置参数
type VivaldiPlusPlusConfig struct {
	// 基础参数
	Dim int     // 虚拟坐标维度
	Cc  float64 // Vivaldi权重常数
	Ce  float64 // 误差权重

	// 窗口大小
	RTTWindow   int // RTT历史窗口M
	CoordWindow int // 坐标历史窗口W

	// 阶段切换参数
	RMin    int     // 最小轮数
	ESwitch float64 // 误差切换门限
	S       int     // 持续轮数
	BMin    int     // 最小稳定节点数

	// 稳定判定参数
	P  float64 // 震荡阈值（百分比）
	E0 float64 // 稳定节点误差上限

	// λ参数
	Tau    float64 // 裕量（可设为0）
	EpsMin float64 // 权重下限
	Alpha  float64 // λ衰减参数

	// 降权参数
	Gamma float64 // 不稳定节点权重

	// 退火参数
	AnnealRate   float64 // 退火衰减率
	AnnealPeriod int     // 退火周期

	// 冻结参数
	Fc float64 // 最大位移上限
}

// NewDefaultConfig 创建默认配置
func NewVivaldiPlusPlusConfig() *VivaldiPlusPlusConfig {
	return &VivaldiPlusPlusConfig{
		Dim:          DefaultDim,
		Cc:           VivaldiCc,
		Ce:           VivaldiCe,
		RTTWindow:    DefaultRTTWindow,
		CoordWindow:  DefaultCoordWindow,
		RMin:         DefaultRMin,
		ESwitch:      DefaultESwitch,
		S:            DefaultS,
		BMin:         DefaultBMin,
		P:            DefaultP,
		E0:           DefaultE0,
		Tau:          DefaultTau,
		EpsMin:       DefaultEpsMin,
		Alpha:        DefaultAlpha,
		Gamma:        DefaultGamma,
		AnnealRate:   DefaultAnnealRate,
		AnnealPeriod: DefaultAnnealPeriod,
		Fc:           DefaultFc,
	}
}

// ==================== RTTTracker: RTT历史跟踪器 ====================

// RTTTracker 维护每个邻居的RTT历史，用于计算中位数RTT
type RTTTracker struct {
	rttHist map[int][]float64
	maxSize int
}

// NewRTTTracker 创建新的RTT跟踪器
func NewRTTTracker(maxSize int) *RTTTracker {
	return &RTTTracker{
		rttHist: make(map[int][]float64),
		maxSize: maxSize,
	}
}

// AddRTT 添加RTT观测值
func (rt *RTTTracker) AddRTT(peerID int, rtt float64) {
	if _, exists := rt.rttHist[peerID]; !exists {
		rt.rttHist[peerID] = make([]float64, 0, rt.maxSize)
	}

	rt.rttHist[peerID] = append(rt.rttHist[peerID], rtt)

	// 保持窗口大小
	if len(rt.rttHist[peerID]) > rt.maxSize {
		rt.rttHist[peerID] = rt.rttHist[peerID][1:]
	}
}

// GetMedianRTT 获取指定邻居的中位数RTT
func (rt *RTTTracker) GetMedianRTT(peerID int) float64 {
	hist, exists := rt.rttHist[peerID]
	if !exists || len(hist) == 0 {
		return 0.0
	}

	// 复制切片避免修改原数据
	sorted := make([]float64, len(hist))
	copy(sorted, hist)
	sort.Float64s(sorted)

	if len(sorted)%2 == 0 {
		return (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	return sorted[len(sorted)/2]
}

// ==================== NeighborHistory: 邻居坐标历史跟踪器 ====================

// NeighborHistory 维护邻居坐标历史，计算震荡指标和稳定性
type NeighborHistory struct {
	coordHist  map[int][]*VivaldiCoordinate // 坐标历史（深拷贝）
	deltaHist  map[int][]float64            // 移动幅度历史
	lastError  map[int]float64              // 上次误差
	osc        map[int]float64              // 震荡指标（delta中位数）
	stable     map[int]bool                 // 是否稳定
	wNode      map[int]float64              // 节点级权重
	windowSize int                          // 历史窗口大小
}

// NewNeighborHistory 创建新的邻居历史跟踪器
func NewNeighborHistory(windowSize int) *NeighborHistory {
	return &NeighborHistory{
		coordHist:  make(map[int][]*VivaldiCoordinate),
		deltaHist:  make(map[int][]float64),
		lastError:  make(map[int]float64),
		osc:        make(map[int]float64),
		stable:     make(map[int]bool),
		wNode:      make(map[int]float64),
		windowSize: windowSize,
	}
}

// copyCoordinate 深拷贝坐标
func copyCoordinate(coord *VivaldiCoordinate) *VivaldiCoordinate {
	if coord == nil {
		return nil
	}
	newCoord := &VivaldiCoordinate{
		Vector: make([]float64, len(coord.Vector)),
		Height: coord.Height,
		Error:  coord.Error,
	}
	copy(newCoord.Vector, coord.Vector)
	return newCoord
}

// Update 更新邻居坐标历史
func (nh *NeighborHistory) Update(peerID int, coord *VivaldiCoordinate) {
	// 深拷贝坐标
	coordCopy := copyCoordinate(coord)

	// 更新坐标历史
	if _, exists := nh.coordHist[peerID]; !exists {
		nh.coordHist[peerID] = make([]*VivaldiCoordinate, 0, nh.windowSize)
		nh.deltaHist[peerID] = make([]float64, 0, nh.windowSize)
		nh.wNode[peerID] = 1.0 // 初始权重为1
	}

	// 计算移动幅度（如果有历史记录）
	if len(nh.coordHist[peerID]) > 0 {
		prevCoord := nh.coordHist[peerID][len(nh.coordHist[peerID])-1]
		delta := DistanceVivaldi(prevCoord, coordCopy)
		nh.deltaHist[peerID] = append(nh.deltaHist[peerID], delta)
		if len(nh.deltaHist[peerID]) > nh.windowSize {
			nh.deltaHist[peerID] = nh.deltaHist[peerID][1:]
		}
	}

	// 更新坐标历史
	nh.coordHist[peerID] = append(nh.coordHist[peerID], coordCopy)
	if len(nh.coordHist[peerID]) > nh.windowSize {
		// 释放旧坐标
		nh.coordHist[peerID] = nh.coordHist[peerID][1:]
	}

	// 更新误差
	if coord != nil {
		nh.lastError[peerID] = coord.Error
	}
}

// ComputeStability 计算邻居的稳定性
func (nh *NeighborHistory) ComputeStability(peerID int, config *VivaldiPlusPlusConfig) {
	// 计算震荡指标（delta的中位数）
	if len(nh.deltaHist[peerID]) > 0 {
		deltaCopy := make([]float64, len(nh.deltaHist[peerID]))
		copy(deltaCopy, nh.deltaHist[peerID])
		sort.Float64s(deltaCopy)

		if len(deltaCopy)%2 == 0 {
			nh.osc[peerID] = (deltaCopy[len(deltaCopy)/2-1] + deltaCopy[len(deltaCopy)/2]) / 2
		} else {
			nh.osc[peerID] = deltaCopy[len(deltaCopy)/2]
		}
	} else {
		nh.osc[peerID] = 0.0
	}

	// 获取当前坐标的范数
	var coordNorm float64 = 1.0
	if len(nh.coordHist[peerID]) > 0 {
		lastCoord := nh.coordHist[peerID][len(nh.coordHist[peerID])-1]
		coordNorm = 0.0
		for _, v := range lastCoord.Vector {
			coordNorm += v * v
		}
		coordNorm = math.Sqrt(coordNorm) + lastCoord.Height
		if coordNorm < 1.0 {
			coordNorm = 1.0
		}
	}

	// 稳定判定：相对震荡小 且 误差小
	relativeOsc := nh.osc[peerID] / coordNorm
	errorOK := nh.lastError[peerID] < config.E0

	nh.stable[peerID] = relativeOsc < config.P && errorOK

	// 更新节点权重
	if nh.stable[peerID] {
		nh.wNode[peerID] = 1.0
	} else {
		nh.wNode[peerID] = config.Gamma
	}
}

// GetStableNodes 获取所有稳定节点ID列表
func (nh *NeighborHistory) GetStableNodes() []int {
	stableNodes := make([]int, 0)
	for peerID, isStable := range nh.stable {
		if isStable {
			stableNodes = append(stableNodes, peerID)
		}
	}
	return stableNodes
}

// ==================== StableSetManager: 稳定节点集合管理 ====================

// StableSetManager 管理稳定节点集合，选择参考点
type StableSetManager struct {
	stableSet []int
}

// NewStableSetManager 创建新的稳定集合管理器
func NewStableSetManager() *StableSetManager {
	return &StableSetManager{
		stableSet: make([]int, 0),
	}
}

// RefreshStableSet 刷新稳定节点集合
func (ssm *StableSetManager) RefreshStableSet(history *NeighborHistory) {
	ssm.stableSet = history.GetStableNodes()
}

// SelectReferencePoint 选择参考点b（震荡最小的稳定节点）
func (ssm *StableSetManager) SelectReferencePoint(history *NeighborHistory) int {
	if len(ssm.stableSet) == 0 {
		return -1 // 回退：稳定集合为空
	}

	// 找到震荡最小的稳定节点
	minOsc := math.MaxFloat64
	bestPeer := -1

	for _, peerID := range ssm.stableSet {
		osc := history.osc[peerID]
		if osc < minOsc {
			minOsc = osc
			bestPeer = peerID
		}
	}

	// 如果所有节点震荡都很大，随机选一个
	if bestPeer == -1 && len(ssm.stableSet) > 0 {
		bestPeer = ssm.stableSet[rand.Intn(len(ssm.stableSet))]
	}

	return bestPeer
}

// GetStableSetSize 获取稳定集合大小
func (ssm *StableSetManager) GetStableSetSize() int {
	return len(ssm.stableSet)
}

// ==================== LambdaChecker: λ三角校验与违例降权 ====================

// ComputeLambda 计算λ值（三角不等式检验）
// λ = t_ij / (t_ib + t_bj)
func ComputeLambda(tij, tib, tbj float64) float64 {
	denom := tib + tbj
	if denom < 1e-6 {
		return 1.0 // 避免除零
	}
	return tij / denom
}

// ComputeWTIV 计算TIV权重 w_tiv（连续衰减）
// 若 λ <= 1 + τ：w_tiv = 1（不违例）
// 若 λ > 1 + τ：w_tiv = max(epsMin, 1/(1 + alpha*(lambda-1-tau)))
func ComputeWTIV(lambda, tau, epsMin, alpha float64) float64 {
	if lambda <= 1.0+tau {
		return 1.0
	}

	// 连续衰减
	wTIV := 1.0 / (1.0 + alpha*(lambda-1.0-tau))
	if wTIV < epsMin {
		wTIV = epsMin
	}
	return wTIV
}

// ==================== VivaldiPlusPlusState: 节点状态 ====================

// VivaldiPlusPlusState 单个节点的完整状态
type VivaldiPlusPlusState struct {
	NodeID             int                // 节点ID
	Phase              string             // "EARLY" 或 "LATE"
	Coord              *VivaldiCoordinate // 当前坐标
	RTTTracker         *RTTTracker        // RTT历史跟踪器
	NeighborHistory    *NeighborHistory   // 邻居历史跟踪器
	StableSetManager   *StableSetManager  // 稳定集合管理器
	PhaseStableCounter int                // 连续满足切换条件的计数
	CurrentCc          float64            // 当前步长
	CurrentCe          float64            // 当前误差权重
	SwitchRound        int                // 切换到Late阶段的轮数（-1表示未切换）
	FixedNeighbors     []int              // 固定邻居集合（128个）
}

// NewVivaldiPlusPlusState 创建新的节点状态
func NewVivaldiPlusPlusState(nodeID int, dim int, config *VivaldiPlusPlusConfig) *VivaldiPlusPlusState {
	// 初始化坐标
	coord := NewVivaldiCoordinate(dim)
	coord.Error = VivaldiInitError

	// 随机初始化坐标
	for d := 0; d < dim; d++ {
		coord.Vector[d] = RandomBetween01() * 1000
	}
	coord.Height = RandomBetween01() * 100

	return &VivaldiPlusPlusState{
		NodeID:             nodeID,
		Phase:              "EARLY",
		Coord:              coord,
		RTTTracker:         NewRTTTracker(config.RTTWindow),
		NeighborHistory:    NewNeighborHistory(config.CoordWindow),
		StableSetManager:   NewStableSetManager(),
		PhaseStableCounter: 0,
		CurrentCc:          config.Cc,
		CurrentCe:          config.Ce,
		SwitchRound:        -1,
		FixedNeighbors:     make([]int, 0, FixedNeighborSetSize), // 初始化为空，由外部填充
	}
}

// ==================== PhaseController: 阶段控制与切换 ====================

// ShouldSwitchToLate 判断是否应该切换到Late阶段
func ShouldSwitchToLate(state *VivaldiPlusPlusState, round int, config *VivaldiPlusPlusConfig) bool {
	// 如果已经是Late阶段，不再切换
	if state.Phase == "LATE" {
		return false
	}

	// 条件1：最小轮数
	if round < config.RMin {
		return false
	}

	// 条件2：误差持续满足 或 稳定节点数足够
	errorOK := state.Coord.Error < config.ESwitch
	stableSetSize := state.StableSetManager.GetStableSetSize()
	stableSetOK := stableSetSize >= config.BMin

	if errorOK {
		state.PhaseStableCounter++
	} else {
		state.PhaseStableCounter = 0
	}

	// 切换条件：误差持续S轮 或 稳定节点数足够
	shouldSwitch := (state.PhaseStableCounter >= config.S) || stableSetOK

	if shouldSwitch {
		state.Phase = "LATE"
		state.SwitchRound = round
		// 切换后降低步长（退火初始化）
		state.CurrentCc = config.Cc * 0.5
		state.CurrentCe = config.Ce * 0.9
		return true
	}

	return false
}

// ==================== 退火与冻结机制 ====================

// ApplyAnnealing 应用退火（减小步长）
func ApplyAnnealing(state *VivaldiPlusPlusState, round int, config *VivaldiPlusPlusConfig) {
	if state.Phase != "LATE" {
		return
	}

	// 每T轮衰减一次
	if state.SwitchRound >= 0 && (round-state.SwitchRound)%config.AnnealPeriod == 0 {
		state.CurrentCc *= config.AnnealRate
		if state.CurrentCc < 0.01 {
			state.CurrentCc = 0.01 // 防止过小
		}
	}
}

// ApplyFreeze 应用冻结（限制最大位移）
func ApplyFreeze(deltaX []float64, fc float64) []float64 {
	// 计算位移范数
	norm := 0.0
	for _, dx := range deltaX {
		norm += dx * dx
	}
	norm = math.Sqrt(norm)

	// 如果超过上限，按比例缩放
	if norm > fc {
		scale := fc / norm
		for i := range deltaX {
			deltaX[i] *= scale
		}
	}

	return deltaX
}

// ==================== ObservePlusPlus: 统一更新逻辑 ====================

// ObservePlusPlus Vivaldi++ 统一更新函数
func ObservePlusPlus(state *VivaldiPlusPlusState, peerID int, peerCoord *VivaldiCoordinate,
	rtt float64, round int, config *VivaldiPlusPlusConfig, coords []LatLonCoordinate) {

	// 添加RTT历史
	state.RTTTracker.AddRTT(peerID, rtt)

	// 更新邻居坐标历史
	state.NeighborHistory.Update(peerID, peerCoord)

	// 计算稳定性（Late阶段或准备切换时）
	if state.Phase == "LATE" || round >= config.RMin-5 {
		state.NeighborHistory.ComputeStability(peerID, config)
	}

	// 获取中位数RTT（用于更新）
	medianRTT := state.RTTTracker.GetMedianRTT(peerID)
	if medianRTT < 1e-6 {
		medianRTT = rtt // 如果历史不足，使用当前RTT
	}

	// 预测距离
	predictedRTT := DistanceVivaldi(state.Coord, peerCoord)

	// 计算相对误差
	relativeError := math.Abs(predictedRTT-medianRTT) / medianRTT
	if medianRTT < 1e-6 {
		relativeError = 0
	}

	// 基础权重（Vivaldi原始权重）
	localError := state.Coord.Error
	peerError := peerCoord.Error
	wBase := localError / (localError + peerError)
	if wBase > 1.0 {
		wBase = 1.0
	}
	if wBase < 0.0 {
		wBase = 0.0
	}

	// 节点级权重
	wNode := state.NeighborHistory.wNode[peerID]
	if wNode == 0 {
		wNode = 1.0 // 默认权重
	}

	// TIV权重（Late阶段启用）
	wTIV := 1.0
	if state.Phase == "LATE" {
		// 选择参考点
		refPoint := state.StableSetManager.SelectReferencePoint(state.NeighborHistory)

		if refPoint >= 0 {
			// 获取三个RTT的中位数
			tij := medianRTT
			tib := state.RTTTracker.GetMedianRTT(refPoint)
			tbj := 0.0 // 需要通过坐标计算或缓存（这里先用坐标距离近似）

			// 如果i和b之间有RTT历史，使用历史；否则用坐标距离
			if tib < 1e-6 {
				// 需要获取b的坐标（这里简化处理，实际应该从状态中获取）
				// 注意：这里需要访问其他节点的坐标，暂时用坐标距离近似
				tib = predictedRTT * 0.8 // 简化：假设b是稳定节点，距离相近
			}

			// 计算b到j的RTT（通过坐标距离近似，实际应该缓存）
			// 这里简化：假设可以通过坐标计算
			// 实际实现中，应该维护一个全局的RTT缓存或通过其他方式获取
			// 为了简化，我们使用坐标距离作为近似
			if len(coords) > refPoint && len(coords) > peerID {
				// 使用真实地理距离作为近似
				tbj = Distance(coords[refPoint], coords[peerID]) + FixedDelay
			} else {
				tbj = predictedRTT * 0.8 // 备用近似
			}

			// 计算λ
			lambda := ComputeLambda(tij, tib, tbj)
			wTIV = ComputeWTIV(lambda, config.Tau, config.EpsMin, config.Alpha)
		}
		// 如果refPoint < 0，wTIV保持为1.0（回退策略）
	}

	// 合成总权重
	W := wBase * wNode * wTIV

	// 更新误差估计
	state.Coord.Error = config.Ce*W*relativeError + (1-config.Ce*W)*localError
	if state.Coord.Error < VivaldiMinError {
		state.Coord.Error = VivaldiMinError
	}

	// 计算力（弹簧更新）
	force := state.CurrentCc * W * (medianRTT - predictedRTT)

	// 计算坐标更新向量
	deltaX := make([]float64, len(state.Coord.Vector))
	if predictedRTT > 1e-6 {
		for i := 0; i < len(state.Coord.Vector); i++ {
			direction := state.Coord.Vector[i] - peerCoord.Vector[i]
			deltaX[i] = force * direction / predictedRTT
		}
	}

	// 高度更新
	heightDelta := 0.0
	heightDiff := state.Coord.Height - peerCoord.Height
	if math.Abs(heightDiff) > 1e-6 {
		heightDelta = force * heightDiff / math.Abs(heightDiff)
	}

	// Late阶段应用冻结
	if state.Phase == "LATE" {
		deltaX = ApplyFreeze(deltaX, config.Fc)
		// 高度也限制
		if math.Abs(heightDelta) > config.Fc {
			heightDelta = math.Copysign(config.Fc, heightDelta)
		}
	}

	// 更新坐标
	for i := 0; i < len(state.Coord.Vector); i++ {
		state.Coord.Vector[i] += deltaX[i]
	}
	state.Coord.Height += heightDelta

	// 确保高度非负
	if state.Coord.Height < 0 {
		state.Coord.Height = 0
	}
}

// ==================== 主流程函数 ====================

// GenerateVirtualCoordinatePlusPlus Vivaldi++ 主流程函数
func GenerateVirtualCoordinatePlusPlus(coords []LatLonCoordinate, rounds int, config *VivaldiPlusPlusConfig) []*VivaldiModel {
	n := len(coords)
	if config == nil {
		config = NewVivaldiPlusPlusConfig()
	}

	fmt.Printf("开始生成Vivaldi++虚拟坐标（%d轮，%d维）...\n", rounds, config.Dim)
	fmt.Printf("配置: R_min=%d, e_switch=%.2f, S=%d, B_min=%d, τ=%.3f\n",
		config.RMin, config.ESwitch, config.S, config.BMin, config.Tau)

	// 初始化所有节点的状态
	states := make([]*VivaldiPlusPlusState, n)
	for i := 0; i < n; i++ {
		states[i] = NewVivaldiPlusPlusState(i, config.Dim, config)
	}

	// 为每个节点分配固定的邻居集合（128个）
	fmt.Printf("为每个节点分配固定邻居集合（每节点%d个固定邻居，每轮采样%d个）...\n",
		FixedNeighborSetSize, NeighborSampleSizePerRound)
	for i := 0; i < n; i++ {
		// 随机选择128个固定邻居（不包括自己）
		candidates := make([]int, 0, n-1)
		for j := 0; j < n; j++ {
			if j != i {
				candidates = append(candidates, j)
			}
		}

		// 随机打乱
		rand.Shuffle(len(candidates), func(a, b int) {
			candidates[a], candidates[b] = candidates[b], candidates[a]
		})

		// 取前128个（如果节点总数不足128，则取所有）
		neighborCount := FixedNeighborSetSize
		if neighborCount > len(candidates) {
			neighborCount = len(candidates)
		}
		states[i].FixedNeighbors = candidates[:neighborCount]
	}

	// 验证固定邻居集合
	totalNeighbors := 0
	for i := 0; i < n; i++ {
		totalNeighbors += len(states[i].FixedNeighbors)
	}
	avgNeighbors := float64(totalNeighbors) / float64(n)
	fmt.Printf("固定邻居集合初始化完成，平均每节点 %.1f 个固定邻居\n", avgNeighbors)

	// 统计信息
	switchRounds := make([]int, 0)
	stableSetSizes := make([]int, 0)
	lambdaViolations := 0
	totalUpdates := 0
	wTIVSum := 0.0
	wTIVCount := 0

	// 迭代更新坐标
	for round := 0; round < rounds; round++ {
		if round%10 == 0 {
			fmt.Printf("  轮次 %d/%d\n", round, rounds)
		}

		// 对每个节点进行处理
		for i := 0; i < n; i++ {
			state := states[i]

			// 检查阶段切换
			if ShouldSwitchToLate(state, round, config) {
				switchRounds = append(switchRounds, round)
				//fmt.Printf("  节点 %d 在第 %d 轮切换到Late阶段\n", i, round)
			}

			// 刷新稳定集合（Late阶段每3-5轮刷新一次，优化性能）
			if state.Phase == "LATE" && round%3 == 0 {
				state.StableSetManager.RefreshStableSet(state.NeighborHistory)
			}

			// 选择邻居策略：从固定邻居集合中采样
			var selectedNeighbors []int
			if state.Phase == "EARLY" {
				// Early阶段：从固定邻居中随机选择16个
				selectedNeighbors = make([]int, 0, NeighborSampleSizePerRound)

				// 随机打乱固定邻居列表
				shuffled := make([]int, len(state.FixedNeighbors))
				copy(shuffled, state.FixedNeighbors)
				rand.Shuffle(len(shuffled), func(a, b int) {
					shuffled[a], shuffled[b] = shuffled[b], shuffled[a]
				})

				// 取前16个
				sampleSize := NeighborSampleSizePerRound
				if sampleSize > len(shuffled) {
					sampleSize = len(shuffled)
				}
				selectedNeighbors = shuffled[:sampleSize]
			} else {
				// Late阶段：优先选择稳定节点（从固定邻居集合中筛选）
				stableSet := state.StableSetManager.stableSet
				selectedNeighbors = make([]int, 0, NeighborSampleSizePerRound)

				// 找出固定邻居中的稳定节点
				stableNeighbors := make([]int, 0)
				for _, peerID := range state.FixedNeighbors {
					if Contains(stableSet, peerID) {
						stableNeighbors = append(stableNeighbors, peerID)
					}
				}

				// 先选稳定节点
				rand.Shuffle(len(stableNeighbors), func(a, b int) {
					stableNeighbors[a], stableNeighbors[b] = stableNeighbors[b], stableNeighbors[a]
				})
				for _, peerID := range stableNeighbors {
					if len(selectedNeighbors) < NeighborSampleSizePerRound {
						selectedNeighbors = append(selectedNeighbors, peerID)
					}
				}

				// 如果不够，从固定邻居中随机补充
				if len(selectedNeighbors) < NeighborSampleSizePerRound {
					candidates := make([]int, 0)
					for _, peerID := range state.FixedNeighbors {
						if !Contains(selectedNeighbors, peerID) {
							candidates = append(candidates, peerID)
						}
					}
					rand.Shuffle(len(candidates), func(a, b int) {
						candidates[a], candidates[b] = candidates[b], candidates[a]
					})
					for _, peerID := range candidates {
						if len(selectedNeighbors) < NeighborSampleSizePerRound {
							selectedNeighbors = append(selectedNeighbors, peerID)
						} else {
							break
						}
					}
				}
			}

			// 对每个邻居进行观测和更新
			for _, j := range selectedNeighbors {
				// 计算真实RTT（基于地理距离）
				rtt := Distance(coords[i], coords[j]) + FixedDelay

				// 记录λ违例统计（Late阶段）
				if state.Phase == "LATE" {
					refPoint := state.StableSetManager.SelectReferencePoint(state.NeighborHistory)
					if refPoint >= 0 {
						tij := state.RTTTracker.GetMedianRTT(j)
						if tij < 1e-6 {
							tij = rtt
						}
						tib := state.RTTTracker.GetMedianRTT(refPoint)
						if tib < 1e-6 {
							tib = Distance(coords[i], coords[refPoint]) + FixedDelay
						}
						tbj := Distance(coords[refPoint], coords[j]) + FixedDelay

						lambda := ComputeLambda(tij, tib, tbj)
						wTIV := ComputeWTIV(lambda, config.Tau, config.EpsMin, config.Alpha)
						if lambda > 1.0+config.Tau {
							lambdaViolations++
						}
						wTIVSum += wTIV
						wTIVCount++
					}
				}

				// 调用更新函数
				ObservePlusPlus(state, j, states[j].Coord, rtt, round, config, coords)
				totalUpdates++
			}

			// 应用退火（Late阶段）
			ApplyAnnealing(state, round, config)

			// 记录稳定集合大小（每10轮一次）
			if round%10 == 0 {
				stableSetSizes = append(stableSetSizes, state.StableSetManager.GetStableSetSize())
			}
		}
	}

	// 转换为VivaldiModel数组（用于兼容）
	models := make([]*VivaldiModel, n)
	for i := 0; i < n; i++ {
		models[i] = &VivaldiModel{
			NodeID:         i,
			LocalCoord:     states[i].Coord,
			RandomPeerSet:  make([]int, 0),
			HaveEnoughPeer: false,
		}
	}

	// ==================== 输出验证指标 ====================

	fmt.Println("\n========== Vivaldi++ 验证指标 ==========")

	// 1. 误差分布统计
	errorCount := make(map[string]int)
	errors := make([]float64, n)
	for i := 0; i < n; i++ {
		err := models[i].LocalCoord.Error
		errors[i] = err

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

	// 计算误差统计
	sort.Float64s(errors)
	avgError := 0.0
	for _, e := range errors {
		avgError += e
	}
	avgError /= float64(n)
	medianError := errors[n/2]
	p95Error := errors[int(float64(n)*0.95)]

	fmt.Printf("误差统计:\n")
	fmt.Printf("  平均: %.4f\n", avgError)
	fmt.Printf("  中位数: %.4f\n", medianError)
	fmt.Printf("  95分位: %.4f\n", p95Error)
	fmt.Println("误差分布:")
	fmt.Printf("  <0.1: %d (%.1f%%)\n", errorCount["<0.1"], float64(errorCount["<0.1"])*100/float64(n))
	fmt.Printf("  0.1-0.2: %d (%.1f%%)\n", errorCount["0.1-0.2"], float64(errorCount["0.1-0.2"])*100/float64(n))
	fmt.Printf("  0.2-0.4: %d (%.1f%%)\n", errorCount["0.2-0.4"], float64(errorCount["0.2-0.4"])*100/float64(n))
	fmt.Printf("  0.4-0.6: %d (%.1f%%)\n", errorCount["0.4-0.6"], float64(errorCount["0.4-0.6"])*100/float64(n))
	fmt.Printf("  >=0.6: %d (%.1f%%)\n", errorCount[">=0.6"], float64(errorCount[">=0.6"])*100/float64(n))

	// // 2. 阶段切换统计
	// if len(switchRounds) > 0 {
	// 	avgSwitchRound := 0.0
	// 	for _, r := range switchRounds {
	// 		avgSwitchRound += float64(r)
	// 	}
	// 	avgSwitchRound /= float64(len(switchRounds))
	// 	fmt.Printf("\n阶段切换统计:\n")
	// 	fmt.Printf("  切换节点数: %d/%d\n", len(switchRounds), n)
	// 	fmt.Printf("  平均切换轮数: %.1f\n", avgSwitchRound)
	// 	fmt.Printf("  最早切换: %d\n", switchRounds[0])
	// 	if len(switchRounds) > 1 {
	// 		fmt.Printf("  最晚切换: %d\n", switchRounds[len(switchRounds)-1])
	// 	}
	// } else {
	// 	fmt.Printf("\n阶段切换统计: 无节点切换到Late阶段\n")
	// }

	// // 3. 稳定节点集合大小趋势
	// if len(stableSetSizes) > 0 {
	// 	fmt.Printf("\n稳定节点集合大小趋势:\n")
	// 	for i, size := range stableSetSizes {
	// 		if i%5 == 0 { // 每50轮显示一次
	// 			fmt.Printf("  轮次 %d: %d 个稳定节点\n", i*10, size)
	// 		}
	// 	}
	// }

	// 4. λ违例统计
	if wTIVCount > 0 {
		avgWTIV := wTIVSum / float64(wTIVCount)
		violationRate := float64(lambdaViolations) / float64(wTIVCount) * 100
		fmt.Printf("\nλ三角校验统计:\n")
		fmt.Printf("  总更新次数: %d\n", totalUpdates)
		fmt.Printf("  λ检测次数: %d\n", wTIVCount)
		fmt.Printf("  违例触发比例: %.2f%%\n", violationRate)
		fmt.Printf("  平均w_tiv: %.4f\n", avgWTIV)
	}

	// 5. 预测误差评估（采样）
	sampleSize := Min(1000, n*n/10)
	if sampleSize < 10 {
		sampleSize = 10
	}
	relativeErrors := make([]float64, 0, sampleSize)
	for s := 0; s < sampleSize; s++ {
		i := rand.Intn(n)
		j := rand.Intn(n)
		if i == j {
			continue
		}

		// 真实RTT
		realRTT := Distance(coords[i], coords[j]) + FixedDelay
		// 预测RTT
		predictedRTT := DistanceVivaldi(models[i].LocalCoord, models[j].LocalCoord)

		// 相对误差
		if realRTT > 1e-6 {
			relErr := math.Abs(predictedRTT-realRTT) / realRTT
			relativeErrors = append(relativeErrors, relErr)
		}
	}

	if len(relativeErrors) > 0 {
		sort.Float64s(relativeErrors)
		avgPredError := 0.0
		for _, e := range relativeErrors {
			avgPredError += e
		}
		avgPredError /= float64(len(relativeErrors))
		medianPredError := relativeErrors[len(relativeErrors)/2]
		p95PredError := relativeErrors[int(float64(len(relativeErrors))*0.95)]

		fmt.Printf("\n预测误差评估（采样%d对）:\n", len(relativeErrors))
		fmt.Printf("  平均相对误差: %.2f%%\n", avgPredError*100)
		fmt.Printf("  中位数相对误差: %.2f%%\n", medianPredError*100)
		fmt.Printf("  95分位相对误差: %.2f%%\n", p95PredError*100)
	}

	fmt.Println("\nVivaldi++ 虚拟坐标生成完成！")
	return models
}

// ==================== 误差分布评估 ====================

// ErrorDistribution 误差分布统计结果
type ErrorDistribution struct {
	ErrorCount         map[string]int // 误差分布计数
	AvgError           float64        // 平均误差
	MedianError        float64        // 中位数误差
	P95Error           float64        // 95分位误差
	LowErrorCount      int            // 误差<0.1的节点数（目标最大化）
	LowErrorRate       float64        // 误差<0.1的比例
	HighErrorCount     int            // 误差>=0.2的节点数
	HighErrorRate      float64        // 误差>=0.2的比例
	VeryHighErrorCount int            // 误差>=0.4的节点数（目标最小化）
	VeryHighErrorRate  float64        // 误差>=0.4的比例
}

// EvaluateErrorDistribution 评估误差分布
func EvaluateErrorDistribution(models []*VivaldiModel) *ErrorDistribution {
	n := len(models)
	errorCount := make(map[string]int)
	errors := make([]float64, n)

	for i := 0; i < n; i++ {
		err := models[i].LocalCoord.Error
		errors[i] = err

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

	// 计算统计值
	sort.Float64s(errors)
	avgError := 0.0
	for _, e := range errors {
		avgError += e
	}
	avgError /= float64(n)
	medianError := errors[n/2]
	p95Error := errors[int(float64(n)*0.95)]

	// 计算各类误差节点数
	lowErrorCount := errorCount["<0.1"]
	highErrorCount := errorCount["0.2-0.4"] + errorCount["0.4-0.6"] + errorCount[">=0.6"]
	veryHighErrorCount := errorCount["0.4-0.6"] + errorCount[">=0.6"]

	return &ErrorDistribution{
		ErrorCount:         errorCount,
		AvgError:           avgError,
		MedianError:        medianError,
		P95Error:           p95Error,
		LowErrorCount:      lowErrorCount,
		LowErrorRate:       float64(lowErrorCount) / float64(n),
		HighErrorCount:     highErrorCount,
		HighErrorRate:      float64(highErrorCount) / float64(n),
		VeryHighErrorCount: veryHighErrorCount,
		VeryHighErrorRate:  float64(veryHighErrorCount) / float64(n),
	}
}

// ==================== 参数自动调节 ====================

// ParameterSearchResult 参数搜索结果
type ParameterSearchResult struct {
	Config    *VivaldiPlusPlusConfig
	ErrorDist *ErrorDistribution
	Score     float64 // 评分（越小越好，主要看HighErrorCount）
}

// AutoTuneParameters 自动调节参数，寻找最优配置
// 目标：最小化平均误差、最大化低误差节点数、最小化0.4以上误差的节点数
func AutoTuneParameters(coords []LatLonCoordinate, rounds int, outputFile string) (*ParameterSearchResult, error) {
	fmt.Println("========== 开始自动参数调节 ==========")
	fmt.Printf("测试参数组合，目标：最小化平均误差、最大化低误差节点数(<0.1)、最小化极高误差节点数(>=0.4)\n\n")

	// 定义参数搜索空间（关键参数）
	rttWindows := []int{10, 15}
	coordWindows := []int{10, 15}
	rMins := []int{15, 20, 25, 30}
	eSwitches := []float64{0.05, 0.1, 0.15, 0.2, 0.25, 0.3}
	sValues := []int{3, 5, 7}
	bMins := []int{3, 5, 7, 10}
	pValues := []float64{0.01, 0.02, 0.03}
	e0Values := []float64{0.15, 0.2, 0.25}
	tauValues := []float64{0, 0.05, 0.1}
	epsMins := []float64{0.15, 0.2, 0.25}
	gammas := []float64{0.1, 0.2, 0.3, 0.4}
	fcs := []float64{60.0, 80.0, 100.0, 120, 140}
	alphas := []float64{0.5, 1.0, 1.5, 2.0, 2.5}
	annealRates := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}
	annealPeriods := []int{3, 5, 7}

	// 为了控制搜索空间，使用部分参数组合（可以逐步细化）
	// 先测试关键参数
	totalCombinations := len(rttWindows) * len(coordWindows) * len(rMins) * len(eSwitches) *
		len(sValues) * len(bMins) * len(pValues) * len(e0Values) * len(tauValues) *
		len(epsMins) * len(gammas) * len(fcs) * len(alphas) * len(annealRates) * len(annealPeriods)

	fmt.Printf("参数搜索空间: %d 种组合（实际会采样测试）\n", totalCombinations)
	fmt.Println("开始测试...")

	// 维护两个最优结果（初始化 ErrorDist 避免空指针）
	bestAvgErrorResult := &ParameterSearchResult{
		Score: math.MaxFloat64,
		ErrorDist: &ErrorDistribution{
			AvgError:           math.MaxFloat64,
			LowErrorCount:      0,
			VeryHighErrorCount: math.MaxInt32,
		},
	}
	bestLowErrorResult := &ParameterSearchResult{
		Score: math.MaxFloat64,
		ErrorDist: &ErrorDistribution{
			AvgError:           math.MaxFloat64,
			LowErrorCount:      0,
			VeryHighErrorCount: math.MaxInt32,
		},
	}

	// 记录所有测试结果
	allResults := make([]*ParameterSearchResult, 0)

	// 由于组合数太大，采用采样策略：随机采样 + 网格搜索关键参数
	testCount := 0
	maxTests := 1000 // 最多测试200个组合

	// 1. 先测试默认参数附近的组合（局部搜索）
	fmt.Println("阶段1: 测试默认参数附近的组合...")
	for _, rttW := range rttWindows {
		for _, coordW := range coordWindows {
			for _, rMin := range rMins {
				for _, eSwitch := range eSwitches {
					config := NewVivaldiPlusPlusConfig()
					config.RTTWindow = rttW
					config.CoordWindow = coordW
					config.RMin = rMin
					config.ESwitch = eSwitch

					result := testConfig(coords, rounds, config, testCount+1)
					if result != nil {
						allResults = append(allResults, result)

						// 更新平均误差最优结果
						if result.ErrorDist.AvgError < bestAvgErrorResult.ErrorDist.AvgError {
							bestAvgErrorResult = result
							fmt.Printf("  找到平均误差更优参数 (测试%d): 平均误差=%.4f, 低误差节点=%d(%.1f%%), 极高误差节点=%d(%.1f%%)\n",
								testCount+1, result.ErrorDist.AvgError,
								result.ErrorDist.LowErrorCount, result.ErrorDist.LowErrorRate*100,
								result.ErrorDist.VeryHighErrorCount, result.ErrorDist.VeryHighErrorRate*100)
							fmt.Println("  参数配置:")
							printConfig(result.Config)
						}

						// 更新低误差节点最多时平均误差最优结果
						if result.ErrorDist.LowErrorCount > bestLowErrorResult.ErrorDist.LowErrorCount ||
							(result.ErrorDist.LowErrorCount == bestLowErrorResult.ErrorDist.LowErrorCount &&
								result.ErrorDist.AvgError < bestLowErrorResult.ErrorDist.AvgError) {
							bestLowErrorResult = result
							fmt.Printf("  找到低误差节点更优参数 (测试%d): 低误差节点=%d(%.1f%%), 平均误差=%.4f, 极高误差节点=%d(%.1f%%)\n",
								testCount+1, result.ErrorDist.LowErrorCount, result.ErrorDist.LowErrorRate*100,
								result.ErrorDist.AvgError,
								result.ErrorDist.VeryHighErrorCount, result.ErrorDist.VeryHighErrorRate*100)
							fmt.Println("  参数配置:")
							printConfig(result.Config)
						}
					}
					testCount++
					if testCount >= maxTests/2 {
						goto phase2
					}
				}
			}
		}
	}

phase2:
	// 2. 随机采样其他参数组合
	fmt.Printf("\n阶段2: 随机采样测试（已测试%d个，继续测试到%d个）...\n", testCount, maxTests)
	rand.Seed(time.Now().UnixNano())

	for testCount < maxTests {
		config := NewVivaldiPlusPlusConfig()
		config.RTTWindow = rttWindows[rand.Intn(len(rttWindows))]
		config.CoordWindow = coordWindows[rand.Intn(len(coordWindows))]
		config.RMin = rMins[rand.Intn(len(rMins))]
		config.ESwitch = eSwitches[rand.Intn(len(eSwitches))]
		config.S = sValues[rand.Intn(len(sValues))]
		config.BMin = bMins[rand.Intn(len(bMins))]
		config.P = pValues[rand.Intn(len(pValues))]
		config.E0 = e0Values[rand.Intn(len(e0Values))]
		config.Tau = tauValues[rand.Intn(len(tauValues))]
		config.EpsMin = epsMins[rand.Intn(len(epsMins))]
		config.Gamma = gammas[rand.Intn(len(gammas))]
		config.Fc = fcs[rand.Intn(len(fcs))]
		config.Alpha = alphas[rand.Intn(len(alphas))]
		config.AnnealRate = annealRates[rand.Intn(len(annealRates))]
		config.AnnealPeriod = annealPeriods[rand.Intn(len(annealPeriods))]

		result := testConfig(coords, rounds, config, testCount+1)
		if result != nil {
			allResults = append(allResults, result)

			// 更新平均误差最优结果
			if result.ErrorDist.AvgError < bestAvgErrorResult.ErrorDist.AvgError {
				bestAvgErrorResult = result
				fmt.Printf("  找到平均误差更优参数 (测试%d): 平均误差=%.4f, 低误差节点=%d(%.1f%%), 极高误差节点=%d(%.1f%%)\n",
					testCount+1, result.ErrorDist.AvgError,
					result.ErrorDist.LowErrorCount, result.ErrorDist.LowErrorRate*100,
					result.ErrorDist.VeryHighErrorCount, result.ErrorDist.VeryHighErrorRate*100)
			}

			// 更新低误差节点最多时平均误差最优结果
			if result.ErrorDist.LowErrorCount > bestLowErrorResult.ErrorDist.LowErrorCount ||
				(result.ErrorDist.LowErrorCount == bestLowErrorResult.ErrorDist.LowErrorCount &&
					result.ErrorDist.AvgError < bestLowErrorResult.ErrorDist.AvgError) {
				bestLowErrorResult = result
				fmt.Printf("  找到低误差节点更优参数 (测试%d): 低误差节点=%d(%.1f%%), 平均误差=%.4f, 极高误差节点=%d(%.1f%%)\n",
					testCount+1, result.ErrorDist.LowErrorCount, result.ErrorDist.LowErrorRate*100,
					result.ErrorDist.AvgError,
					result.ErrorDist.VeryHighErrorCount, result.ErrorDist.VeryHighErrorRate*100)
			}
		}
		testCount++

		if testCount%20 == 0 {
			fmt.Printf("  已测试 %d/%d 个组合\n", testCount, maxTests)
			fmt.Printf("    当前平均误差最优: %.4f (低误差节点=%d, 极高误差节点=%d)\n",
				bestAvgErrorResult.ErrorDist.AvgError,
				bestAvgErrorResult.ErrorDist.LowErrorCount,
				bestAvgErrorResult.ErrorDist.VeryHighErrorCount)
			fmt.Printf("    当前低误差节点最多: %d(%.1f%%) (平均误差=%.4f, 极高误差节点=%d)\n",
				bestLowErrorResult.ErrorDist.LowErrorCount, bestLowErrorResult.ErrorDist.LowErrorRate*100,
				bestLowErrorResult.ErrorDist.AvgError,
				bestLowErrorResult.ErrorDist.VeryHighErrorCount)
		}
	}

	// 输出最优结果
	fmt.Println("\n========== 参数调节完成 ==========")
	fmt.Printf("总测试组合数: %d\n\n", testCount)

	// 输出平均误差最优结果
	fmt.Println("【结果1】平均误差最优配置:")
	fmt.Printf("  平均误差: %.4f\n", bestAvgErrorResult.ErrorDist.AvgError)
	fmt.Printf("  低误差(<0.1)节点: %d (%.1f%%)\n",
		bestAvgErrorResult.ErrorDist.LowErrorCount, bestAvgErrorResult.ErrorDist.LowErrorRate*100)
	fmt.Printf("  极高误差(>=0.4)节点: %d (%.1f%%)\n",
		bestAvgErrorResult.ErrorDist.VeryHighErrorCount, bestAvgErrorResult.ErrorDist.VeryHighErrorRate*100)
	fmt.Println("  参数配置:")
	printConfig(bestAvgErrorResult.Config)
	fmt.Println("\n  完整误差分布:")
	printErrorDistribution(bestAvgErrorResult.ErrorDist)

	// 输出低误差节点最多时平均误差最优结果
	fmt.Println("\n【结果2】低误差节点最多时平均误差最优配置:")
	fmt.Printf("  低误差(<0.1)节点: %d (%.1f%%)\n",
		bestLowErrorResult.ErrorDist.LowErrorCount, bestLowErrorResult.ErrorDist.LowErrorRate*100)
	fmt.Printf("  平均误差: %.4f\n", bestLowErrorResult.ErrorDist.AvgError)
	fmt.Printf("  极高误差(>=0.4)节点: %d (%.1f%%)\n",
		bestLowErrorResult.ErrorDist.VeryHighErrorCount, bestLowErrorResult.ErrorDist.VeryHighErrorRate*100)
	fmt.Println("  参数配置:")
	printConfig(bestLowErrorResult.Config)
	fmt.Println("\n  完整误差分布:")
	printErrorDistribution(bestLowErrorResult.ErrorDist)

	// 保存两个最优参数到不同文件
	if outputFile == "" {
		outputFile = "vivaldi_plusplus_optimal_params.txt"
	}

	// 保存平均误差最优结果
	avgErrorFile := outputFile
	err := saveOptimalParams(avgErrorFile, bestAvgErrorResult)
	if err != nil {
		return nil, fmt.Errorf("保存平均误差最优参数失败: %v", err)
	}
	fmt.Printf("\n平均误差最优参数已保存到: %s\n", avgErrorFile)

	// 保存低误差节点最多时的最优结果
	lowErrorFile := "vivaldi_plusplus_low_error_params.json"
	err = saveOptimalParams(lowErrorFile, bestLowErrorResult)
	if err != nil {
		return nil, fmt.Errorf("保存低误差节点最优参数失败: %v", err)
	}
	fmt.Printf("低误差节点最优参数已保存到: %s\n", lowErrorFile)

	// 返回平均误差最优结果作为主要结果
	return bestAvgErrorResult, nil
}

// testConfig 测试单个配置
func testConfig(coords []LatLonCoordinate, rounds int, config *VivaldiPlusPlusConfig, testID int) *ParameterSearchResult {
	// 静默运行（不输出详细信息）
	// 可以通过重定向输出或修改函数来实现，这里简化处理
	models := GenerateVirtualCoordinatePlusPlusSilent(coords, rounds, config)
	if models == nil {
		return nil
	}

	errorDist := EvaluateErrorDistribution(models)

	// 评分函数（越小越好）：
	// 1. 平均误差（权重10）- 主要目标
	// 2. 低误差率（<0.1）奖励（权重-5）- 越多越好
	// 3. 极高误差节点（>=0.4）惩罚（权重100）- 应该很少
	// 4. 中高误差节点（>=0.2）惩罚（权重20）- 作为辅助
	score := errorDist.AvgError*10.0 -
		errorDist.LowErrorRate*5.0 +
		float64(errorDist.VeryHighErrorCount)*100.0/float64(len(coords)) +
		float64(errorDist.HighErrorCount)*20.0/float64(len(coords))

	return &ParameterSearchResult{
		Config:    config,
		ErrorDist: errorDist,
		Score:     score,
	}
}

// GenerateVirtualCoordinatePlusPlusSilent 静默版本（不输出详细信息）
func GenerateVirtualCoordinatePlusPlusSilent(coords []LatLonCoordinate, rounds int, config *VivaldiPlusPlusConfig) []*VivaldiModel {
	n := len(coords)
	if config == nil {
		config = NewVivaldiPlusPlusConfig()
	}

	// 初始化所有节点的状态
	states := make([]*VivaldiPlusPlusState, n)
	for i := 0; i < n; i++ {
		states[i] = NewVivaldiPlusPlusState(i, config.Dim, config)
	}

	// 为每个节点分配固定的邻居集合（128个）
	for i := 0; i < n; i++ {
		// 随机选择128个固定邻居（不包括自己）
		candidates := make([]int, 0, n-1)
		for j := 0; j < n; j++ {
			if j != i {
				candidates = append(candidates, j)
			}
		}

		// 随机打乱
		rand.Shuffle(len(candidates), func(a, b int) {
			candidates[a], candidates[b] = candidates[b], candidates[a]
		})

		// 取前128个（如果节点总数不足128，则取所有）
		neighborCount := FixedNeighborSetSize
		if neighborCount > len(candidates) {
			neighborCount = len(candidates)
		}
		states[i].FixedNeighbors = candidates[:neighborCount]
	}

	// 迭代更新坐标（静默模式）
	for round := 0; round < rounds; round++ {
		for i := 0; i < n; i++ {
			state := states[i]

			// 检查阶段切换
			ShouldSwitchToLate(state, round, config)

			// 刷新稳定集合
			if state.Phase == "LATE" && round%3 == 0 {
				state.StableSetManager.RefreshStableSet(state.NeighborHistory)
			}

			// 选择邻居策略：从固定邻居集合中采样
			var selectedNeighbors []int
			if state.Phase == "EARLY" {
				// Early阶段：从固定邻居中随机选择16个
				selectedNeighbors = make([]int, 0, NeighborSampleSizePerRound)

				// 随机打乱固定邻居列表
				shuffled := make([]int, len(state.FixedNeighbors))
				copy(shuffled, state.FixedNeighbors)
				rand.Shuffle(len(shuffled), func(a, b int) {
					shuffled[a], shuffled[b] = shuffled[b], shuffled[a]
				})

				// 取前16个
				sampleSize := NeighborSampleSizePerRound
				if sampleSize > len(shuffled) {
					sampleSize = len(shuffled)
				}
				selectedNeighbors = shuffled[:sampleSize]
			} else {
				// Late阶段：优先选择稳定节点（从固定邻居集合中筛选）
				stableSet := state.StableSetManager.stableSet
				selectedNeighbors = make([]int, 0, NeighborSampleSizePerRound)

				// 找出固定邻居中的稳定节点
				stableNeighbors := make([]int, 0)
				for _, peerID := range state.FixedNeighbors {
					if Contains(stableSet, peerID) {
						stableNeighbors = append(stableNeighbors, peerID)
					}
				}

				// 先选稳定节点
				rand.Shuffle(len(stableNeighbors), func(a, b int) {
					stableNeighbors[a], stableNeighbors[b] = stableNeighbors[b], stableNeighbors[a]
				})
				for _, peerID := range stableNeighbors {
					if len(selectedNeighbors) < NeighborSampleSizePerRound {
						selectedNeighbors = append(selectedNeighbors, peerID)
					}
				}

				// 如果不够，从固定邻居中随机补充
				if len(selectedNeighbors) < NeighborSampleSizePerRound {
					candidates := make([]int, 0)
					for _, peerID := range state.FixedNeighbors {
						if !Contains(selectedNeighbors, peerID) {
							candidates = append(candidates, peerID)
						}
					}
					rand.Shuffle(len(candidates), func(a, b int) {
						candidates[a], candidates[b] = candidates[b], candidates[a]
					})
					for _, peerID := range candidates {
						if len(selectedNeighbors) < NeighborSampleSizePerRound {
							selectedNeighbors = append(selectedNeighbors, peerID)
						} else {
							break
						}
					}
				}
			}

			// 对每个邻居进行观测和更新
			for _, j := range selectedNeighbors {
				rtt := Distance(coords[i], coords[j]) + FixedDelay
				ObservePlusPlus(state, j, states[j].Coord, rtt, round, config, coords)
			}

			// 应用退火
			ApplyAnnealing(state, round, config)
		}
	}

	// 转换为VivaldiModel数组
	models := make([]*VivaldiModel, n)
	for i := 0; i < n; i++ {
		models[i] = &VivaldiModel{
			NodeID:         i,
			LocalCoord:     states[i].Coord,
			RandomPeerSet:  make([]int, 0),
			HaveEnoughPeer: false,
		}
	}

	return models
}

// printConfig 打印配置参数
func printConfig(config *VivaldiPlusPlusConfig) {
	fmt.Printf("  Dim: %d\n", config.Dim)
	fmt.Printf("  RTTWindow: %d\n", config.RTTWindow)
	fmt.Printf("  CoordWindow: %d\n", config.CoordWindow)
	fmt.Printf("  RMin: %d\n", config.RMin)
	fmt.Printf("  ESwitch: %.3f\n", config.ESwitch)
	fmt.Printf("  S: %d\n", config.S)
	fmt.Printf("  BMin: %d\n", config.BMin)
	fmt.Printf("  P: %.3f\n", config.P)
	fmt.Printf("  E0: %.3f\n", config.E0)
	fmt.Printf("  Tau: %.3f\n", config.Tau)
	fmt.Printf("  EpsMin: %.3f\n", config.EpsMin)
	fmt.Printf("  Gamma: %.3f\n", config.Gamma)
	fmt.Printf("  Fc: %.1f\n", config.Fc)
	fmt.Printf("  Alpha: %.3f\n", config.Alpha)
	fmt.Printf("  AnnealRate: %.3f\n", config.AnnealRate)
	fmt.Printf("  AnnealPeriod: %d\n", config.AnnealPeriod)
}

// printErrorDistribution 打印误差分布
func printErrorDistribution(dist *ErrorDistribution) {
	total := dist.ErrorCount["<0.1"] + dist.ErrorCount["0.1-0.2"] +
		dist.ErrorCount["0.2-0.4"] + dist.ErrorCount["0.4-0.6"] +
		dist.ErrorCount[">=0.6"]

	fmt.Printf("  平均误差: %.4f\n", dist.AvgError)
	fmt.Printf("  中位数误差: %.4f\n", dist.MedianError)
	fmt.Printf("  95分位误差: %.4f\n", dist.P95Error)
	fmt.Printf("  误差分布:\n")
	fmt.Printf("    <0.1: %d (%.1f%%) ✓ [优秀]\n", dist.ErrorCount["<0.1"], float64(dist.ErrorCount["<0.1"])*100/float64(total))
	fmt.Printf("    0.1-0.2: %d (%.1f%%) [良好]\n", dist.ErrorCount["0.1-0.2"], float64(dist.ErrorCount["0.1-0.2"])*100/float64(total))
	fmt.Printf("    0.2-0.4: %d (%.1f%%) [一般]\n", dist.ErrorCount["0.2-0.4"], float64(dist.ErrorCount["0.2-0.4"])*100/float64(total))
	fmt.Printf("    0.4-0.6: %d (%.1f%%) ✗ [较差]\n", dist.ErrorCount["0.4-0.6"], float64(dist.ErrorCount["0.4-0.6"])*100/float64(total))
	fmt.Printf("    >=0.6: %d (%.1f%%) ✗✗ [极差]\n", dist.ErrorCount[">=0.6"], float64(dist.ErrorCount[">=0.6"])*100/float64(total))
	fmt.Printf("  关键指标:\n")
	fmt.Printf("    低误差(<0.1)节点: %d (%.1f%%) [目标: 最大化]\n", dist.LowErrorCount, dist.LowErrorRate*100)
	fmt.Printf("    误差>=0.2节点: %d (%.1f%%)\n", dist.HighErrorCount, dist.HighErrorRate*100)
	fmt.Printf("    极高误差(>=0.4)节点: %d (%.1f%%) [目标: 最小化]\n", dist.VeryHighErrorCount, dist.VeryHighErrorRate*100)
}

// saveOptimalParams 保存最优参数到文件
func saveOptimalParams(filename string, result *ParameterSearchResult) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "========== Vivaldi++ 最优参数配置 ==========\n")
	fmt.Fprintf(file, "生成时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	fmt.Fprintf(file, "========== 参数配置 ==========\n")
	fmt.Fprintf(file, "Dim: %d\n", result.Config.Dim)
	fmt.Fprintf(file, "Cc: %.3f\n", result.Config.Cc)
	fmt.Fprintf(file, "Ce: %.3f\n", result.Config.Ce)
	fmt.Fprintf(file, "RTTWindow: %d\n", result.Config.RTTWindow)
	fmt.Fprintf(file, "CoordWindow: %d\n", result.Config.CoordWindow)
	fmt.Fprintf(file, "RMin: %d\n", result.Config.RMin)
	fmt.Fprintf(file, "ESwitch: %.3f\n", result.Config.ESwitch)
	fmt.Fprintf(file, "S: %d\n", result.Config.S)
	fmt.Fprintf(file, "BMin: %d\n", result.Config.BMin)
	fmt.Fprintf(file, "P: %.3f\n", result.Config.P)
	fmt.Fprintf(file, "E0: %.3f\n", result.Config.E0)
	fmt.Fprintf(file, "Tau: %.3f\n", result.Config.Tau)
	fmt.Fprintf(file, "EpsMin: %.3f\n", result.Config.EpsMin)
	fmt.Fprintf(file, "Gamma: %.3f\n", result.Config.Gamma)
	fmt.Fprintf(file, "Fc: %.1f\n", result.Config.Fc)
	fmt.Fprintf(file, "Alpha: %.3f\n", result.Config.Alpha)
	fmt.Fprintf(file, "AnnealRate: %.3f\n", result.Config.AnnealRate)
	fmt.Fprintf(file, "AnnealPeriod: %d\n", result.Config.AnnealPeriod)

	fmt.Fprintf(file, "\n========== 误差分布结果 ==========\n")
	fmt.Fprintf(file, "平均误差: %.4f\n", result.ErrorDist.AvgError)
	fmt.Fprintf(file, "中位数误差: %.4f\n", result.ErrorDist.MedianError)
	fmt.Fprintf(file, "95分位误差: %.4f\n", result.ErrorDist.P95Error)
	fmt.Fprintf(file, "\n误差分布:\n")
	total := result.ErrorDist.ErrorCount["<0.1"] + result.ErrorDist.ErrorCount["0.1-0.2"] +
		result.ErrorDist.ErrorCount["0.2-0.4"] + result.ErrorDist.ErrorCount["0.4-0.6"] +
		result.ErrorDist.ErrorCount[">=0.6"]
	fmt.Fprintf(file, "  <0.1: %d (%.1f%%)\n", result.ErrorDist.ErrorCount["<0.1"],
		float64(result.ErrorDist.ErrorCount["<0.1"])*100/float64(total))
	fmt.Fprintf(file, "  0.1-0.2: %d (%.1f%%)\n", result.ErrorDist.ErrorCount["0.1-0.2"],
		float64(result.ErrorDist.ErrorCount["0.1-0.2"])*100/float64(total))
	fmt.Fprintf(file, "  0.2-0.4: %d (%.1f%%)\n", result.ErrorDist.ErrorCount["0.2-0.4"],
		float64(result.ErrorDist.ErrorCount["0.2-0.4"])*100/float64(total))
	fmt.Fprintf(file, "  0.4-0.6: %d (%.1f%%)\n", result.ErrorDist.ErrorCount["0.4-0.6"],
		float64(result.ErrorDist.ErrorCount["0.4-0.6"])*100/float64(total))
	fmt.Fprintf(file, "  >=0.6: %d (%.1f%%)\n", result.ErrorDist.ErrorCount[">=0.6"],
		float64(result.ErrorDist.ErrorCount[">=0.6"])*100/float64(total))
	fmt.Fprintf(file, "\n误差>=0.2节点数: %d (%.2f%%)\n", result.ErrorDist.HighErrorCount, result.ErrorDist.HighErrorRate*100)
	fmt.Fprintf(file, "评分: %.4f (越小越好)\n", result.Score)

	fmt.Fprintf(file, "\n========== Go代码配置 ==========\n")
	fmt.Fprintf(file, "const (\n")
	fmt.Fprintf(file, "  DefaultDim          = %d\n", result.Config.Dim)
	fmt.Fprintf(file, "  DefaultRTTWindow    = %d\n", result.Config.RTTWindow)
	fmt.Fprintf(file, "  DefaultCoordWindow  = %d\n", result.Config.CoordWindow)
	fmt.Fprintf(file, "  DefaultRMin         = %d\n", result.Config.RMin)
	fmt.Fprintf(file, "  DefaultESwitch      = %.3f\n", result.Config.ESwitch)
	fmt.Fprintf(file, "  DefaultS            = %d\n", result.Config.S)
	fmt.Fprintf(file, "  DefaultBMin         = %d\n", result.Config.BMin)
	fmt.Fprintf(file, "  DefaultP            = %.3f\n", result.Config.P)
	fmt.Fprintf(file, "  DefaultE0           = %.3f\n", result.Config.E0)
	fmt.Fprintf(file, "  DefaultTau          = %.3f\n", result.Config.Tau)
	fmt.Fprintf(file, "  DefaultEpsMin       = %.3f\n", result.Config.EpsMin)
	fmt.Fprintf(file, "  DefaultGamma        = %.3f\n", result.Config.Gamma)
	fmt.Fprintf(file, "  DefaultFc           = %.1f\n", result.Config.Fc)
	fmt.Fprintf(file, "  DefaultAlpha        = %.3f\n", result.Config.Alpha)
	fmt.Fprintf(file, "  DefaultAnnealRate   = %.3f\n", result.Config.AnnealRate)
	fmt.Fprintf(file, "  DefaultAnnealPeriod = %d\n", result.Config.AnnealPeriod)
	fmt.Fprintf(file, ")\n")

	return nil
}
