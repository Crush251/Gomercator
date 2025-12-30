package handlware

import (
	"math"
	"sort"
)

// ==================== 延迟百分位计算 ====================

// CalculatePercentiles 计算延迟的百分位数
// recvTimes: 所有节点的接收时间（未覆盖的节点使用inf）
// 返回: 21个百分位的延迟值 [5%, 10%, ..., 100%]
func CalculatePercentiles(recvTimes []float64) []float64 {
	// 复制一份用于排序
	times := make([]float64, len(recvTimes))
	copy(times, recvTimes)
	sort.Float64s(times)

	percentiles := make([]float64, 21)
	cnt := 0

	for pct := 0.05; pct <= 1.0; pct += 0.05 {
		idx := int(float64(len(times)) * pct)
		if idx >= len(times) {
			idx = len(times) - 1
		}
		percentiles[cnt] = times[idx]
		cnt++
	}

	return percentiles
}

// ==================== 深度分布统计 ====================

// CalculateDepthCDF 计算深度累积分布函数
// depths: 每个节点的深度
// 返回: 每个深度级别的节点比例
func CalculateDepthCDF(depths []int) []float64 {
	cdf := make([]float64, MaxDepth)
	total := len(depths)

	if total == 0 {
		return cdf
	}

	// 统计每个深度的节点数
	for _, d := range depths {
		if d >= 0 && d < MaxDepth {
			cdf[d]++
		}
	}

	// 转换为比例
	for i := 0; i < MaxDepth; i++ {
		cdf[i] /= float64(total)
	}

	return cdf
}

// CalculateAvgDistByDepth 计算每层的平均距离延迟
// depths: 每个节点的深度
// dists: 每个节点的距离延迟（recv_time - send_time）
// 返回: 每个深度级别的平均距离延迟
func CalculateAvgDistByDepth(depths []int, dists []float64) []float64 {
	avgDist := make([]float64, MaxDepth)
	depthCnt := make([]int, MaxDepth)

	for i := 0; i < len(depths); i++ {
		d := depths[i]
		if d >= 0 && d < MaxDepth {
			avgDist[d] += dists[i]
			depthCnt[d]++
		}
	}

	// 计算平均值
	for i := 0; i < MaxDepth; i++ {
		if depthCnt[i] > 0 {
			avgDist[i] /= float64(depthCnt[i])
		}
	}

	return avgDist
}

// ==================== 带宽统计 ====================

// CalculateBandwidth 计算平均带宽消耗（重复消息率）
// dupMsgCount: 重复消息数量
// totalNodes: 总节点数
// 返回: 平均带宽 = (重复消息数 + 总节点数) / 总节点数
func CalculateBandwidth(dupMsgCount, totalNodes int) float64 {
	if totalNodes == 0 {
		return 0
	}
	return float64(dupMsgCount+totalNodes) / float64(totalNodes)
}

// ==================== 簇统计 ====================

// CalculateClusterStatistics 计算每个簇的统计信息
// clusterResult: 聚类结果
// depths: 每个节点的深度
// latencies: 每个节点的延迟
// recvFlags: 每个节点是否接收到消息
// 返回: (簇平均深度, 簇平均延迟)
func CalculateClusterStatistics(clusterResult *ClusterResult, depths []int, latencies []float64, recvFlags []bool) ([]float64, []float64) {
	clusterAvgDepth := make([]float64, K)
	clusterAvgLatency := make([]float64, K)
	clusterRecvCount := make([]int, K)

	for i := 0; i < len(depths); i++ {
		if recvFlags[i] {
			c := clusterResult.ClusterID[i]
			if c >= 0 && c < K {
				clusterAvgDepth[c] += float64(depths[i])
				clusterAvgLatency[c] += latencies[i]
				clusterRecvCount[c]++
			}
		}
	}

	// 计算平均值
	for c := 0; c < K; c++ {
		if clusterRecvCount[c] > 0 {
			clusterAvgDepth[c] /= float64(clusterRecvCount[c])
			clusterAvgLatency[c] /= float64(clusterRecvCount[c])
		}
	}

	return clusterAvgDepth, clusterAvgLatency
}

// ==================== 结果累加和平均 ====================

// AccumulateResults 累加两个测试结果
func AccumulateResults(dst, src *TestResult) {
	dst.AvgBandwidth += src.AvgBandwidth
	dst.AvgLatency += src.AvgLatency

	for i := 0; i < len(src.Latency); i++ {
		dst.Latency[i] += src.Latency[i]
	}

	for i := 0; i < MaxDepth; i++ {
		dst.DepthCDF[i] += src.DepthCDF[i]
		dst.AvgDist[i] += src.AvgDist[i]
	}

	for i := 0; i < K; i++ {
		dst.ClusterAvgDepth[i] += src.ClusterAvgDepth[i]
		dst.ClusterAvgLatency[i] += src.ClusterAvgLatency[i]
	}
}

// AverageResults 对测试结果求平均
func AverageResults(result *TestResult, count int) {
	const inf = 1e8

	if count == 0 {
		return
	}

	fcount := float64(count)
	result.AvgBandwidth /= fcount
	result.AvgLatency /= fcount

	// 延迟百分位需要特殊处理（剔除inf值）
	for i := 0; i < len(result.Latency); i++ {
		tmp := int(result.Latency[i] / inf)
		result.Latency[i] -= float64(tmp) * inf
		validCount := count - tmp

		if validCount == 0 {
			result.Latency[i] = 0
		} else {
			result.Latency[i] /= float64(validCount)
		}

		if result.Latency[i] < 0.1 {
			result.Latency[i] = inf
		}
	}

	// 深度CDF和平均距离
	for i := 0; i < MaxDepth; i++ {
		result.DepthCDF[i] /= fcount
		result.AvgDist[i] /= fcount
	}

	// 簇统计
	for i := 0; i < K; i++ {
		result.ClusterAvgDepth[i] /= fcount
		result.ClusterAvgLatency[i] /= fcount
	}
}

// ==================== Perigee专用统计 ====================

// PerigeeObservation Perigee观测数据
type PerigeeObservation struct {
	Observations []float64 // 时间差观测值
	Src          int       // 源节点
	Dst          int       // 目标节点
}

// NewPerigeeObservation 创建新的观测对象
func NewPerigeeObservation(src, dst int) *PerigeeObservation {
	return &PerigeeObservation{
		Observations: make([]float64, 0),
		Src:          src,
		Dst:          dst,
	}
}

// Add 添加观测值
func (po *PerigeeObservation) Add(t float64) {
	if t < 0 {
		// 异常情况：时间差为负
		return
	}
	po.Observations = append(po.Observations, t)
}

// GetLCBUCB 获取Lower Confidence Bound和Upper Confidence Bound
// 返回: (LCB, UCB)
func (po *PerigeeObservation) GetLCBUCB() (float64, float64) {
	length := len(po.Observations)
	if length == 0 {
		return 1e10, 1e10
	}

	// 计算第90百分位
	pos := int(float64(length) * 0.9)
	if pos >= length {
		pos = length - 1
	}

	// 使用快速选择算法
	obsCopy := make([]float64, length)
	copy(obsCopy, po.Observations)
	NthElement(obsCopy, pos)
	per90obs := obsCopy[pos]

	// 计算置信区间偏差
	bias := 125.0 * math.Sqrt(math.Log(float64(length))/(2.0*float64(length)))

	lcb := per90obs - bias
	ucb := per90obs + bias

	return lcb, ucb
}
