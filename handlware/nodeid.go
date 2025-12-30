package handlware

import (
	"crypto/rand"
	"math/big"
	"math/bits"
)

// ==================== NodeID128 和 XOR 距离计算 ====================

// NodeID128 代表 128-bit 节点 ID
type NodeID128 [16]byte

// GenerateRandomNodeID 生成随机 128-bit NodeID
// 使用加密安全的随机数生成器
func GenerateRandomNodeID() NodeID128 {
	var id NodeID128
	_, err := rand.Read(id[:])
	if err != nil {
		// 如果加密随机失败，回退到伪随机
		for i := 0; i < 16; i++ {
			id[i] = byte(RandomBetween01() * 256)
		}
	}
	return id
}

// XORDistance 计算两个 NodeID 的 XOR 距离
// 返回 a XOR b
func XORDistance(a, b NodeID128) NodeID128 {
	var result NodeID128
	for i := 0; i < 16; i++ {
		result[i] = a[i] ^ b[i]
	}
	return result
}

// BucketIndex 计算 XOR 距离对应的桶索引
// bucketIndex = floor(log2(dist))，返回 0..127
// 若 dist=0 返回 -1
//
// 实现原理：找到 XOR 结果中最高位的 1 的位置
// 例如：dist = 0x00...010...0，最高位 1 在第 k 位，则 bucketIndex = k
func BucketIndex(dist NodeID128) int {
	// 从高字节到低字节遍历
	for byteIdx := 0; byteIdx < 16; byteIdx++ {
		if dist[byteIdx] != 0 {
			// 找到第一个非 0 字节
			// 计算该字节中最高位 1 的位置
			leadingZeros := bits.LeadingZeros8(dist[byteIdx])
			bitPos := 7 - leadingZeros
			
			// 计算全局位索引
			// 字节 0 是最高字节（位 127-120）
			// 字节 15 是最低字节（位 7-0）
			return (15-byteIdx)*8 + bitPos
		}
	}
	
	// dist = 0，返回 -1
	return -1
}

// CompareNodeID 比较两个 NodeID128 的大小（无符号）
// 返回值：
//   -1: a < b
//    0: a == b
//   +1: a > b
func CompareNodeID(a, b NodeID128) int {
	for i := 0; i < 16; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// ==================== 辅助函数 ====================

// NodeIDToBigInt 将 NodeID128 转换为 big.Int（用于调试）
func NodeIDToBigInt(id NodeID128) *big.Int {
	return new(big.Int).SetBytes(id[:])
}

// NodeIDToHex 将 NodeID128 转换为十六进制字符串（用于调试）
func NodeIDToHex(id NodeID128) string {
	return NodeIDToBigInt(id).Text(16)
}

// IsZeroNodeID 检查 NodeID 是否为全 0
func IsZeroNodeID(id NodeID128) bool {
	for i := 0; i < 16; i++ {
		if id[i] != 0 {
			return false
		}
	}
	return true
}

// DistanceValue 计算两个 NodeID 的 XOR 距离并返回 big.Int（用于距离比较）
func DistanceValue(a, b NodeID128) *big.Int {
	dist := XORDistance(a, b)
	return NodeIDToBigInt(dist)
}

