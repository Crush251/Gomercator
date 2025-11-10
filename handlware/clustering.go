package handlware

import (
	"fmt"
	"math"
	"math/rand"
)

// ==================== K-means聚类（基于真实坐标）====================

// KMeans 基于经纬度坐标的K-means聚类
// 参数:
//   - coords: 节点坐标数组
//   - k: 簇数量
//   - maxIter: 最大迭代次数
//   - seed: 随机数种子
//
// 返回: 聚类结果
func KMeans(coords []LatLonCoordinate, k int, maxIter int, seed int64) *ClusterResult {
	rng := rand.New(rand.NewSource(seed))
	n := len(coords)

	if n == 0 || k <= 0 {
		return NewClusterResult(k, n)
	}

	result := NewClusterResult(k, n)

	// 初始化中心点（随机选择k个不重复的节点）
	centers := make([]LatLonCoordinate, k)
	tmpList := make([]int, 0, k)

	for i := 0; i < k; i++ {
		for {
			u := rng.Intn(n)
			if !Contains(tmpList, u) {
				centers[i] = coords[u]
				tmpList = append(tmpList, u)
				break
			}
		}
	}

	// K-means迭代
	for iter := 0; iter < maxIter; iter++ {
		// 1. 为每个节点分配最近的中心点
		for i := 0; i < n; i++ {
			minDist := math.MaxFloat64
			bestCluster := 0

			for j := 0; j < k; j++ {
				dist := Distance(centers[j], coords[i])
				if dist < minDist {
					minDist = dist
					bestCluster = j
				}
			}

			result.ClusterID[i] = bestCluster
		}

		// 2. 重新计算中心点
		avg := make([]LatLonCoordinate, k)
		result.ClusterCnt = make([]int, k) // 重置计数

		for i := 0; i < n; i++ {
			c := result.ClusterID[i]
			avg[c].Lon += coords[i].Lon
			avg[c].Lat += coords[i].Lat
			result.ClusterCnt[c]++
		}

		for i := 0; i < k; i++ {
			if result.ClusterCnt[i] > 0 {
				centers[i].Lon = avg[i].Lon / float64(result.ClusterCnt[i])
				centers[i].Lat = avg[i].Lat / float64(result.ClusterCnt[i])
			}
		}
	}

	// 3. 构建ClusterList
	result.ClusterList = make([][]int, k)
	for i := 0; i < k; i++ {
		result.ClusterList[i] = make([]int, 0)
	}

	for i := 0; i < n; i++ {
		c := result.ClusterID[i]
		result.ClusterList[c] = append(result.ClusterList[c], i)
	}

	// 打印聚类结果
	fmt.Printf("聚类结果（K=%d）:\n", k)
	for i := 0; i < k; i++ {
		fmt.Printf("簇 %d: %d 个节点\n", i, len(result.ClusterList[i]))
	}

	return result
}

// ==================== K-means聚类（基于虚拟坐标）====================

// KMeansVirtual 基于Vivaldi虚拟坐标的K-means聚类
// 参数:
//   - vmodels: Vivaldi模型数组
//   - k: 簇数量
//   - maxIter: 最大迭代次数
//   - seed: 随机数种子
//
// 返回: 聚类结果
func KMeansVirtual(vmodels []*VivaldiModel, k int, maxIter int, seed int64) *ClusterResult {
	rng := rand.New(rand.NewSource(seed))
	n := len(vmodels)

	if n == 0 || k <= 0 {
		return NewClusterResult(k, n)
	}

	result := NewClusterResult(k, n)

	// 获取虚拟坐标的维度
	dim := len(vmodels[0].LocalCoord.Vector)

	// 初始化中心点（随机选择k个不重复的节点）
	centers := make([][]float64, k)
	tmpList := make([]int, 0, k)

	for i := 0; i < k; i++ {
		for {
			u := rng.Intn(n)
			if !Contains(tmpList, u) {
				centers[i] = make([]float64, dim)
				copy(centers[i], vmodels[u].Vector())
				tmpList = append(tmpList, u)
				break
			}
		}
	}

	// K-means迭代
	for iter := 0; iter < maxIter; iter++ {
		// 1. 为每个节点分配最近的中心点
		for i := 0; i < n; i++ {
			minDist := math.MaxFloat64
			bestCluster := 0

			for j := 0; j < k; j++ {
				dist := DistanceEuclidean(centers[j], vmodels[i].Vector())
				if dist < minDist {
					minDist = dist
					bestCluster = j
				}
			}

			result.ClusterID[i] = bestCluster
		}

		// 2. 重新计算中心点
		avg := make([][]float64, k)
		for i := 0; i < k; i++ {
			avg[i] = make([]float64, dim)
		}
		result.ClusterCnt = make([]int, k) // 重置计数

		for i := 0; i < n; i++ {
			c := result.ClusterID[i]
			vec := vmodels[i].Vector()
			for d := 0; d < dim; d++ {
				avg[c][d] += vec[d]
			}
			result.ClusterCnt[c]++
		}

		for i := 0; i < k; i++ {
			if result.ClusterCnt[i] > 0 {
				for d := 0; d < dim; d++ {
					centers[i][d] = avg[i][d] / float64(result.ClusterCnt[i])
				}
			}
		}
	}

	// 3. 构建ClusterList
	result.ClusterList = make([][]int, k)
	for i := 0; i < k; i++ {
		result.ClusterList[i] = make([]int, 0)
	}

	for i := 0; i < n; i++ {
		c := result.ClusterID[i]
		result.ClusterList[c] = append(result.ClusterList[c], i)
	}

	// 打印聚类结果
	fmt.Printf("聚类结果（基于虚拟坐标，K=%d）:\n", k)
	for i := 0; i < k; i++ {
		fmt.Printf("簇 %d: %d 个节点\n", i, len(result.ClusterList[i]))
	}

	return result
}

// ==================== 辅助函数 ====================

// ComputeClusterInertia 计算聚类惯性（簇内平方和）
// 用于评估聚类质量
func ComputeClusterInertia(coords []LatLonCoordinate, result *ClusterResult) float64 {
	inertia := 0.0
	k := result.K

	// 计算每个簇的中心
	centers := make([]LatLonCoordinate, k)
	for i := 0; i < k; i++ {
		if result.ClusterCnt[i] > 0 {
			sumLat := 0.0
			sumLon := 0.0
			for _, nodeID := range result.ClusterList[i] {
				sumLat += coords[nodeID].Lat
				sumLon += coords[nodeID].Lon
			}
			centers[i].Lat = sumLat / float64(result.ClusterCnt[i])
			centers[i].Lon = sumLon / float64(result.ClusterCnt[i])
		}
	}

	// 计算所有节点到其簇中心的距离平方和
	for i := 0; i < len(coords); i++ {
		c := result.ClusterID[i]
		dist := Distance(coords[i], centers[c])
		inertia += dist * dist
	}

	return inertia
}

// ComputeClusterInertiaVirtual 计算虚拟坐标聚类的惯性
func ComputeClusterInertiaVirtual(vmodels []*VivaldiModel, result *ClusterResult) float64 {
	inertia := 0.0
	k := result.K
	dim := len(vmodels[0].LocalCoord.Vector)

	// 计算每个簇的中心
	centers := make([][]float64, k)
	for i := 0; i < k; i++ {
		centers[i] = make([]float64, dim)
		if result.ClusterCnt[i] > 0 {
			for _, nodeID := range result.ClusterList[i] {
				vec := vmodels[nodeID].Vector()
				for d := 0; d < dim; d++ {
					centers[i][d] += vec[d]
				}
			}
			for d := 0; d < dim; d++ {
				centers[i][d] /= float64(result.ClusterCnt[i])
			}
		}
	}

	// 计算所有节点到其簇中心的距离平方和
	for i := 0; i < len(vmodels); i++ {
		c := result.ClusterID[i]
		dist := DistanceEuclidean(vmodels[i].Vector(), centers[c])
		inertia += dist * dist
	}

	return inertia
}

// FindOptimalK 使用肘部法则寻找最优K值（实验性功能）
// 返回每个K值对应的惯性
func FindOptimalK(coords []LatLonCoordinate, maxK int, maxIter int, seed int64) []float64 {
	inertias := make([]float64, maxK+1)

	fmt.Println("寻找最优K值...")
	for k := 1; k <= maxK; k++ {
		result := KMeans(coords, k, maxIter, seed)
		inertias[k] = ComputeClusterInertia(coords, result)
		fmt.Printf("K=%d, 惯性=%.2f\n", k, inertias[k])
	}

	return inertias
}
