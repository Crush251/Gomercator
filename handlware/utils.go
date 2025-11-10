package handlware

import (
	"math"
	"math/rand"
)

// ==================== 距离计算相关 ====================

// Rad 角度转弧度
func Rad(deg float64) float64 {
	return deg * Pi / 180.0
}

// Distance 计算两个经纬度坐标之间的地理距离（单位：ms延迟）
// 注意：C++代码中返回的是 distance/100000*2，表示延迟（ms）
func Distance(a, b LatLonCoordinate) float64 {
	// 如果两点非常接近，视为同一点
	if math.Abs(a.Lat-b.Lat) < 0.1 && math.Abs(a.Lon-b.Lon) < 0.1 {
		return 0
	}

	latA := Rad(a.Lat)
	lonA := Rad(a.Lon)
	latB := Rad(b.Lat)
	lonB := Rad(b.Lon)

	// Haversine公式
	c := math.Cos(latA)*math.Cos(latB)*math.Cos(lonA-lonB) + math.Sin(latA)*math.Sin(latB)

	// 防止浮点误差导致c超出[-1, 1]
	if c > 1.0 {
		c = 1.0
	} else if c < -1.0 {
		c = -1.0
	}

	dist := math.Acos(c) * EarthRadius

	// 转换为延迟（ms）：distance / 100000 * 2
	return dist / 100000.0 * 2.0
}

// DistanceEuclidean 计算欧几里得距离（用于虚拟坐标）
func DistanceEuclidean(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	sum := 0.0
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

// DistanceVivaldi 计算Vivaldi坐标之间的距离
func DistanceVivaldi(a, b *VivaldiCoordinate) float64 {
	euclidean := DistanceEuclidean(a.Vector, b.Vector)
	return euclidean + a.Height + b.Height
}

// FitInRing 将经度差值限制在[-180, 180]范围内
func FitInRing(x float64) float64 {
	if x < -180 {
		x += 360
	}
	if x > 180 {
		x -= 360
	}
	return x
}

// AngleCheck 检查向量 r->u 和 u->v 的夹角是否在[-90, 90]度内
// 用于Mercator的方向性检查
func AngleCheck(r, u, v LatLonCoordinate) bool {
	x1 := u.Lon - r.Lon
	y1 := u.Lat - r.Lat
	x2 := v.Lon - u.Lon
	y2 := v.Lat - u.Lat

	x1 = FitInRing(x1)
	x2 = FitInRing(x2)

	// 获取 (u-r) 的垂直向量
	x3 := y1
	y3 := -x1

	// 使用叉积检查角度
	crossProduct := x3*y2 - x2*y3
	return crossProduct > -1e-3
}

// ==================== 随机数相关 ====================

// RandomNum 生成[0, n)范围内的随机整数
func RandomNum(n int) int {
	if n <= 0 {
		return 0
	}
	return rand.Intn(n)
}

// RandomBetween01 生成[0, 1)范围内的随机浮点数
func RandomBetween01() float64 {
	return rand.Float64()
}

// RandomNormal 生成正态分布的随机数（均值mean，标准差std）
func RandomNormal(mean, std float64) float64 {
	return rand.NormFloat64()*std + mean
}

// ==================== 数组和切片工具 ====================

// Contains 检查切片中是否包含目标元素
func Contains(slice []int, target int) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

// RemoveElement 从切片中移除指定元素（快速删除，不保持顺序）
func RemoveElement(slice []int, target int) []int {
	for i, v := range slice {
		if v == target {
			slice[i] = slice[len(slice)-1]
			return slice[:len(slice)-1]
		}
	}
	return slice
}

// Min 返回两个整数的最小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个整数的最大值
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MinFloat64 返回两个浮点数的最小值
func MinFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// MaxFloat64 返回两个浮点数的最大值
func MaxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// Clamp 将值限制在[min, max]范围内
func Clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// ==================== 延迟计算相关 ====================

// CalculateTransmissionDelay 计算数据传输延迟（ms）
func CalculateTransmissionDelay(dataSize, bandwidth float64) float64 {
	// dataSize: Bytes
	// bandwidth: bps (bits per second)
	// 返回: ms
	return (dataSize * 8 / bandwidth) * 1000.0
}

// CalculatePropagationDelay 计算传播延迟（距离延迟 + 数据传输延迟）
func CalculatePropagationDelay(u, v int, coords []LatLonCoordinate, bandwidth, dataSize float64) float64 {
	distDelay := Distance(coords[u], coords[v]) * 3.0
	dataDelay := CalculateTransmissionDelay(dataSize, bandwidth)
	return distDelay + dataDelay
}

// CalculateProcessingDelay 计算节点处理延迟（固定延迟 + 随机高斯噪声）
func CalculateProcessingDelay() float64 {
	// 固定延迟250ms，模拟时拆分为: 200ms + Gaussian(50, 10)
	base := FixedDelay - 50.0
	noise := RandomNormal(50.0, 10.0)

	// 限制噪声在[0, 100]范围内
	noise = Clamp(noise, 0.0, 100.0)

	return base + noise
}

// ==================== 排序工具 ====================

// PairIntFloat 用于排序的(int, float64)对
type PairIntFloat struct {
	First  int
	Second float64
}

// PairFloatInt 用于排序的(float64, int)对
type PairFloatInt struct {
	First  float64
	Second int
}

// SortByFirst 按第一个元素排序PairFloatInt切片
type ByFirst []PairFloatInt

func (a ByFirst) Len() int           { return len(a) }
func (a ByFirst) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByFirst) Less(i, j int) bool { return a[i].First < a[j].First }

// ==================== 统计工具 ====================

// NthElement 快速选择第n小的元素（类似C++ nth_element）
// 修改slice，使得slice[n]是第n小的元素
func NthElement(slice []float64, n int) {
	if n < 0 || n >= len(slice) {
		return
	}
	quickSelect(slice, 0, len(slice)-1, n)
}

// quickSelect 快速选择算法实现
func quickSelect(slice []float64, left, right, k int) {
	if left >= right {
		return
	}

	pivotIndex := partition(slice, left, right)

	if pivotIndex == k {
		return
	} else if pivotIndex > k {
		quickSelect(slice, left, pivotIndex-1, k)
	} else {
		quickSelect(slice, pivotIndex+1, right, k)
	}
}

// partition 分区函数
func partition(slice []float64, left, right int) int {
	pivot := slice[right]
	i := left

	for j := left; j < right; j++ {
		if slice[j] <= pivot {
			slice[i], slice[j] = slice[j], slice[i]
			i++
		}
	}

	slice[i], slice[right] = slice[right], slice[i]
	return i
}
