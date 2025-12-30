package handlware

import (
	"fmt"
	"math"
	"strings"
)

// ==================== Geohash常量 ====================

const (
	// Base32字符集（Geohash标准）
	Base32Charset = "0123456789bcdefghjkmnpqrstuvwxyz"
)

// ==================== Geohash编码/解码 ====================

// GeoRange 地理范围
type GeoRange struct {
	Min float64
	Max float64
}

// NewGeoRange 创建新的地理范围
func NewGeoRange(min, max float64) *GeoRange {
	return &GeoRange{Min: min, Max: max}
}

// Mid 计算范围中点
func (gr *GeoRange) Mid() float64 {
	return (gr.Min + gr.Max) / 2.0
}

// GeohashEncoder Geohash编码器
type GeohashEncoder struct {
	Precision int // 精度（字符数）
}

// NewGeohashEncoder 创建新的Geohash编码器
func NewGeohashEncoder(precision int) *GeohashEncoder {
	return &GeohashEncoder{Precision: precision}
}

// Encode 编码经纬度为Geohash字符串
// 参数:
//   - lat: 纬度 [-90, 90]
//   - lon: 经度 [-180, 180]
//
// 返回: Geohash字符串
func (ge *GeohashEncoder) Encode(lat, lon float64) string {
	if ge.Precision <= 0 {
		return ""
	}

	// 初始化范围
	latRange := NewGeoRange(-90.0, 90.0)
	lonRange := NewGeoRange(-180.0, 180.0)

	geohash := strings.Builder{}
	isEven := true // 偶数位编码经度，奇数位编码纬度
	bit := 0
	idx := 0

	for geohash.Len() < ge.Precision {
		if isEven {
			// 处理经度
			mid := lonRange.Mid()
			if lon >= mid {
				idx = idx*2 + 1
				lonRange.Min = mid
			} else {
				idx = idx * 2
				lonRange.Max = mid
			}
		} else {
			// 处理纬度
			mid := latRange.Mid()
			if lat >= mid {
				idx = idx*2 + 1
				latRange.Min = mid
			} else {
				idx = idx * 2
				latRange.Max = mid
			}
		}

		isEven = !isEven

		// 每5位生成一个字符
		bit++
		if bit == 5 {
			geohash.WriteByte(Base32Charset[idx])
			bit = 0
			idx = 0
		}
	}

	return geohash.String()
}

// Decode 解码Geohash字符串为经纬度
// 返回: (纬度, 经度)
func (ge *GeohashEncoder) Decode(geohash string) (float64, float64) {
	latRange := NewGeoRange(-90.0, 90.0)
	lonRange := NewGeoRange(-180.0, 180.0)
	isEven := true

	for _, ch := range geohash {
		idx := strings.IndexRune(Base32Charset, ch)
		if idx == -1 {
			continue
		}

		// 每个字符表示5位二进制
		for i := 4; i >= 0; i-- {
			bit := (idx >> uint(i)) & 1

			if isEven {
				// 经度
				mid := lonRange.Mid()
				if bit == 1 {
					lonRange.Min = mid
				} else {
					lonRange.Max = mid
				}
			} else {
				// 纬度
				mid := latRange.Mid()
				if bit == 1 {
					latRange.Min = mid
				} else {
					latRange.Max = mid
				}
			}

			isEven = !isEven
		}
	}

	return latRange.Mid(), lonRange.Mid()
}

// ==================== 二进制表示 ====================

// CharToBits 将Geohash字符转换为5位二进制字符串
func CharToBits(ch rune) string {
	idx := strings.IndexRune(Base32Charset, ch)
	if idx == -1 {
		idx = 0
	}

	bits := strings.Builder{}
	for i := 4; i >= 0; i-- {
		if (idx>>uint(i))&1 == 1 {
			bits.WriteRune('1')
		} else {
			bits.WriteRune('0')
		}
	}

	return bits.String()
}

// ToBinary 将Geohash字符串转换为二进制字符串
func ToBinary(geohash string) string {
	binary := strings.Builder{}
	for _, ch := range geohash {
		binary.WriteString(CharToBits(ch))
	}
	return binary.String()
}

// ==================== 邻居查找 ====================

// GetNeighbors 获取Geohash的8个邻居（北、东北、东、东南、南、西南、西、西北）
func GetNeighbors(geohash string, encoder *GeohashEncoder) []string {
	if len(geohash) == 0 {
		return []string{}
	}

	neighbors := make([]string, 0, 8)

	// 方向偏移：北、东北、东、东南、南、西南、西、西北
	dx := []int{0, 1, 1, 1, 0, -1, -1, -1}
	dy := []int{1, 1, 0, -1, -1, -1, 0, 1}

	lat, lon := encoder.Decode(geohash)

	// 估算经纬度变化单位
	latUnit := 180.0 / math.Pow(2, float64(len(geohash))*2.5)
	lonUnit := 360.0 / math.Pow(2, float64(len(geohash))*2.5)

	for i := 0; i < 8; i++ {
		neighborLat := lat + float64(dy[i])*latUnit
		neighborLon := lon + float64(dx[i])*lonUnit

		// 处理边界情况
		neighborLat = math.Max(-90.0, math.Min(90.0, neighborLat))
		// 经度环绕处理
		neighborLon = math.Mod(math.Mod(neighborLon+540.0, 360.0)-180.0, 360.0)

		neighborHash := encoder.Encode(neighborLat, neighborLon)
		neighbors = append(neighbors, neighborHash)
	}

	return neighbors
}

// ==================== XOR距离计算 ====================

// XorDistance 计算两个二进制字符串的XOR距离
func XorDistance(binaryA, binaryB string) uint {
	distance := uint(0)
	maxLen := len(binaryA)
	if len(binaryB) < maxLen {
		maxLen = len(binaryB)
	}

	for i := 0; i < maxLen; i++ {
		if binaryA[i] != binaryB[i] {
			distance += 1 << uint(maxLen-i-1)
		}
	}

	return distance
}

// GetGeoBucketIndex 获取两个Geohash之间的桶索引
// 桶索引基于XOR距离的最高位
func GetGeoBucketIndex(hashA, hashB string, totalBits int) int {
	binaryA := ToBinary(hashA)
	binaryB := ToBinary(hashB)

	dist := XorDistance(binaryA, binaryB)
	if dist == 0 {
		return 0 // 相同的Geohash，桶0
	}

	// 找到最高位的位置
	idx := 0
	for dist > 0 {
		dist >>= 1
		idx++
	}

	return idx
}

// ==================== K-ary树辅助函数 ====================

// ComputeKaryChildren 计算K-ary树中节点的子节点索引
// 参数:
//   - nodeIdx: 当前节点在有序列表中的索引
//   - totalNodes: 总节点数
//   - k: 分支因子
//
// 返回: 子节点索引列表
func ComputeKaryChildren(nodeIdx, totalNodes, k int) []int {
	children := make([]int, 0, k)

	for i := 1; i <= k; i++ {
		childIdx := nodeIdx*k + i
		if childIdx < totalNodes {
			children = append(children, childIdx)
		}
	}

	return children
}

// ==================== 前缀树构建 ====================

// BuildPrefixTree 构建Geohash前缀树
// 参数:
//   - nodeGeohash: 所有节点的Geohash数组
//
// 返回: 前缀树根节点
func BuildPrefixTree(nodeGeohash []string) *GeoPrefixNode {
	root := NewGeoPrefixNode("")

	for i, hash := range nodeGeohash {
		curr := root
		prefix := strings.Builder{}

		// 将节点添加到所有相应的前缀节点
		for _, ch := range hash {
			prefix.WriteRune(ch)
			prefixStr := prefix.String()

			if _, exists := curr.Children[ch]; !exists {
				curr.Children[ch] = NewGeoPrefixNode(prefixStr)
			}

			curr = curr.Children[ch]
			curr.NodeIDs = append(curr.NodeIDs, i)
		}
	}

	return root
}

// FindNodesWithPrefix 查找具有特定前缀的所有节点
func FindNodesWithPrefix(root *GeoPrefixNode, prefix string) []int {
	curr := root

	for _, ch := range prefix {
		if _, exists := curr.Children[ch]; !exists {
			return []int{} // 前缀不存在
		}
		curr = curr.Children[ch]
	}

	return curr.NodeIDs
}

// ==================== K桶填充辅助 ====================

// InitializeKBuckets 初始化K桶结构
// 参数:
//   - n: 节点数
//   - totalBits: Geohash总位数
//
// 返回: K桶结构 [节点][桶ID][节点列表]
func InitializeKBuckets(n, totalBits int) [][][]int {
	kBuckets := make([][][]int, n)
	for i := 0; i < n; i++ {
		kBuckets[i] = make([][]int, totalBits+1)
		for j := 0; j <= totalBits; j++ {
			kBuckets[i][j] = make([]int, 0)
		}
	}
	return kBuckets
}

// FillK0Bucket 填充K0桶（相同Geohash的节点）
// 参数:
//   - kBuckets: K桶结构
//   - geohashGroups: Geohash分组 map[geohash][]nodeID
func FillK0Bucket(kBuckets [][][]int, geohashGroups map[string][]int) int {
	pairCount := 0

	for _, group := range geohashGroups {
		if len(group) > 1 {
			// 将同一geohash组的所有节点互相添加到各自的K0桶中
			for i := 0; i < len(group); i++ {
				for j := 0; j < len(group); j++ {
					if i != j {
						kBuckets[group[i]][0] = append(kBuckets[group[i]][0], group[j])
						pairCount++
					}
				}
			}
		}
	}
	// //每个节点随机保留15个k0桶的连接
	// for i := range kBuckets {
	// 	k0Len := len(kBuckets[i][0])
	// 	if k0Len > 6 {
	// 		rand.Shuffle(k0Len, func(a, b int) {
	// 			kBuckets[i][0][a], kBuckets[i][0][b] = kBuckets[i][0][b], kBuckets[i][0][a]
	// 		})
	// 		kBuckets[i][0] = kBuckets[i][0][:6]
	// 	}
	// }
	return pairCount
}

// FillOtherKBucketsFixed 填充其他K桶（K1到Kn），逻辑与C++版本不一致
// 参数:
//   - kBuckets: K桶结构
//   - nodeGeohashBinary: 节点Geohash的二进制表示
//   - coords: 节点坐标（用于按距离排序）
//   - bucketSize: 每个桶的最大容量
//   - totalBits: Geohash总位数

func FillOtherKBucketsFixed(kBuckets [][][]int, nodeGeohashBinary []string, coords []LatLonCoordinate, bucketSize, totalBits int) int {
	n := len(nodeGeohashBinary)
	connections := 0

	for i := 0; i < n; i++ {
		binI := nodeGeohashBinary[i]

		// 对于每个节点，填充其K1到Kn桶
		for bucketIdx := 1; bucketIdx <= totalBits; bucketIdx++ {
			// 如果桶已满，跳过
			if len(kBuckets[i][bucketIdx]) >= bucketSize {
				continue
			}

			// 查找满足条件的节点
			candidates := make([]PairFloatInt, 0)

			for j := 0; j < n; j++ {
				if i == j {
					continue
				}

				binJ := nodeGeohashBinary[j]

				// 找出从左往右第一个不同的位
				diffPos := -1
				minLen := len(binI)
				if len(binJ) < minLen {
					minLen = len(binJ)
				}

				for b := 0; b < minLen; b++ {
					if binI[b] != binJ[b] {
						diffPos = b
						break
					}
				}

				if diffPos == -1 {
					continue // 相同的Geohash
				}

				// 计算桶索引：从最高位开始计数
				calcBucketIdx := totalBits - diffPos

				if calcBucketIdx == bucketIdx && len(kBuckets[i][bucketIdx]) < bucketSize {
					dist := Distance(coords[i], coords[j])
					candidates = append(candidates, PairFloatInt{First: dist, Second: j})
				}
			}

			// 按距离排序，选择最近的节点
			if len(candidates) > 0 {
				// 简单排序
				for a := 0; a < len(candidates)-1; a++ {
					for b := a + 1; b < len(candidates); b++ {
						if candidates[a].First > candidates[b].First {
							candidates[a], candidates[b] = candidates[b], candidates[a]
						}
					}
				}

				// 选择最近的bucketSize个节点
				for c := 0; c < len(candidates) && c < bucketSize && len(kBuckets[i][bucketIdx]) < bucketSize; c++ {
					kBuckets[i][bucketIdx] = append(kBuckets[i][bucketIdx], candidates[c].Second)
					connections++
				}
			}
		}
	}

	return connections
}

// FillOtherKBuckets 按 C++ 版本（含其缺陷）完全复刻的实现
// - 第一处写入：把候选节点写入其“真实桶 calcBucketIdx”
// - 第二处写入：将所有候选（跨桶聚合后排序）再写入“当前外层枚举桶 bucketIdx”
// 注意：本实现故意保留 C++ 里的错桶/重复/超容等问题
func FillOtherKBuckets(
	kBuckets [][][]int,
	nodeGeohashBinary []string,
	coords []LatLonCoordinate,
	bucketSize, totalBits int,
) int {
	n := len(nodeGeohashBinary)
	connections := 0

	for i := 0; i < n; i++ {
		binI := nodeGeohashBinary[i]

		// 外层：枚举所有桶（1..totalBits）
		for bucketIdx := 1; bucketIdx <= totalBits; bucketIdx++ {

			// 若“当前外层桶”已满，直接跳过（注意：只检查外层桶）
			if len(kBuckets[i][bucketIdx]) >= bucketSize {
				continue
			}

			// 跨真实桶聚合的候选（准备在第二阶段写入外层桶）
			type cand struct {
				dist float64
				id   int
			}
			candidates := make([]cand, 0)

			// 内层：扫描所有节点，决定其“真实桶 calcBucketIdx”
			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				binJ := nodeGeohashBinary[j]

				// 找从左到右首个不同位
				diffPos := -1
				minLen := len(binI)
				if len(binJ) < minLen {
					minLen = len(binJ)
				}
				for b := 0; b < minLen; b++ {
					if binI[b] != binJ[b] {
						diffPos = b
						break
					}
				}
				if diffPos == -1 {
					// geohash 完全相同（K0），这里 C++ 逻辑不走“其他桶”分支，跳过
					continue
				}

				// 计算“真实桶号”
				calcBucketIdx := totalBits - diffPos

				// 与 C++ 一致：只要“真实桶”在范围内且该桶未满，就：
				// 1) 记录候选（用于二次写入外层桶）
				// 2) 立即写入“真实桶”（第一次写入）
				if calcBucketIdx >= 1 && calcBucketIdx <= totalBits &&
					len(kBuckets[i][calcBucketIdx]) < bucketSize {

					dist := Distance(coords[i], coords[j])
					candidates = append(candidates, cand{dist: dist, id: j})

					// 第一次写入：写到“真实桶 calcBucketIdx”
					kBuckets[i][calcBucketIdx] = append(kBuckets[i][calcBucketIdx], j)
					connections++
				}
			}

			// 将跨桶聚合的候选按距离升序排序
			for a := 0; a < len(candidates)-1; a++ {
				for b := a + 1; b < len(candidates); b++ {
					if candidates[a].dist > candidates[b].dist {
						candidates[a], candidates[b] = candidates[b], candidates[a]
					}
				}
			}

			// 第二次写入：把 Top-K 候选“再”写入到**当前外层桶 bucketIdx**
			// 注意：故意不做去重/不做容量检查（复刻 C++ 的错误行为）
			limit := bucketSize
			if limit > len(candidates) {
				limit = len(candidates)
			}
			for c := 0; c < limit; c++ {
				kBuckets[i][bucketIdx] = append(kBuckets[i][bucketIdx], candidates[c].id)
				connections++
			}
		}
		// >>> 新增：节点 i 的所有桶做一次稳定去重（不裁容量，不改变分桶逻辑）
		for b := 0; b <= totalBits; b++ {
			kBuckets[i][b] = DedupIntsStable(kBuckets[i][b])
		}
	}
	return connections
}

// 相关工具
// DedupIntsStable 原地稳定去重（保留首次出现的顺序）
func DedupIntsStable(xs []int) []int {
	if len(xs) <= 1 {
		return xs
	}
	seen := make(map[int]struct{}, len(xs))
	out := xs[:0]
	for _, v := range xs {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// FirstDiffBitPos 全二进制串的首个不同位位置（从0开始，-1表示完全相同）
func FirstDiffBitPos(aBin, bBin string) int {
	minLen := len(aBin)
	if len(bBin) < minLen {
		minLen = len(bBin)
	}
	for i := 0; i < minLen; i++ {
		if aBin[i] != bBin[i] {
			return i
		}
	}
	return -1
}

// ==================== 调试和导出 ====================

// PrintGeohashInfo 打印Geohash信息（调试用）
func PrintGeohashInfo(geohash string, encoder *GeohashEncoder) {
	lat, lon := encoder.Decode(geohash)
	binary := ToBinary(geohash)

	fmt.Printf("Geohash: %s\n", geohash)
	fmt.Printf("  位置: (%.4f, %.4f)\n", lat, lon)
	fmt.Printf("  二进制: %s\n", binary)
	fmt.Printf("  长度: %d字符 = %d位\n", len(geohash), len(binary))
}

// VerifyGeohashEncoding 验证Geohash编码的正确性
func VerifyGeohashEncoding(encoder *GeohashEncoder, lat, lon float64) bool {
	hash := encoder.Encode(lat, lon)
	decodedLat, decodedLon := encoder.Decode(hash)

	// 计算误差（取决于精度）
	maxError := 180.0 / math.Pow(2, float64(encoder.Precision)*2.5)

	latError := math.Abs(lat - decodedLat)
	lonError := math.Abs(lon - decodedLon)

	if latError > maxError || lonError > maxError {
		fmt.Printf("编码验证失败: (%.4f, %.4f) -> %s -> (%.4f, %.4f)\n",
			lat, lon, hash, decodedLat, decodedLon)
		return false
	}

	return true
}
