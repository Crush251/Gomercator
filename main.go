package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"gomercator/handlware"
	"gomercator/handlware/algorithms"
)

func main() {
	// 设置随机数种子
	rand.Seed(100)

	fmt.Println("========================================")
	fmt.Println("   MERCATOR 广播算法模拟器 (Go版本)")
	fmt.Println("========================================")
	fmt.Println()

	// 1. 读取地理坐标数据
	fmt.Println("步骤 1/5: 读取地理坐标数据...")
	coords, err := handlware.ReadGeoCoordinates("../Geo.txt")
	if err != nil {
		log.Fatalf("读取坐标文件失败: %v", err)
	}

	n := len(coords)
	if n > 8000 {
		n = 8000
		coords = coords[:n]
	}

	fmt.Printf("成功读取 %d 个节点的坐标\n\n", n)

	// 2. 配置模拟参数
	reptTime := 1
	malNode := 0.0
	attackConfig := handlware.NewAttackConfig()
	attackConfig.MaliciousRatio = malNode

	simConfig := handlware.NewSimulatorConfig()
	simConfig.Bandwidth = 33000000.0 // 33 Mbps
	simConfig.DataSize = 300.0       // 300 Bytes

	// 清空输出文件
	// os.Remove("sim_output.csv")
	// os.Remove("fig.csv")

	// 3. 运行 MERCATOR 算法
	fmt.Println("步骤 2/5: 运行 MERCATOR 算法...")
	fmt.Println("----------------------------------------")
	runMercator(n, coords, reptTime, attackConfig, simConfig)
	fmt.Println()

	// 4. 生成虚拟坐标和聚类（用于 Mercury 和 BlockP2P）
	fmt.Println("步骤 3/5: 生成虚拟坐标和聚类...")
	fmt.Println("----------------------------------------")

	fmt.Println("生成Vivaldi虚拟坐标...")
	vmodels := handlware.GenerateVirtualCoordinate(coords, 100, 3)

	fmt.Println("基于虚拟坐标进行K-means聚类...")
	clusterResult := handlware.KMeansVirtual(vmodels, 8, 100, 13)
	fmt.Println()

	// 5. 运行 Mercury 算法
	fmt.Println("步骤 4/5: 运行 MERCURY 算法...")
	fmt.Println("----------------------------------------")
	runMercury(n, coords, vmodels, clusterResult, reptTime, attackConfig, simConfig)
	fmt.Println()

	// 6. 运行其他对比算法
	fmt.Println("步骤 5/5: 运行对比算法...")
	fmt.Println("----------------------------------------")

	// Random Flood
	runRandom(n, coords, reptTime, attackConfig, simConfig)

	//普通聚类
	handlware.KMeans(coords, 8, 100, 13)
	// BlockP2P
	runBlockP2P(n, coords, clusterResult, reptTime, attackConfig, simConfig)

	// Perigee (简化版)
	runPerigee(n, coords, reptTime, attackConfig, simConfig)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("   所有模拟完成！")
	fmt.Println("   结果已保存到:")
	fmt.Println("   - sim_output.csv (详细结果)")
	fmt.Println("   - fig.csv (图表数据)")
	fmt.Println("   - kbuckets.csv (k桶信息)")
	fmt.Println("   - geohash_comparison.csv (Geohash对比信息)")
	fmt.Println("========================================")
	//等待查看结果
	fmt.Scanln()
}

// runMercator 运行MERCATOR算法
func runMercator(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Mercator参数
	geoPrec := 2
	bucketSize := 6
	k0Threshold := 9999 // 不使用k-ary树
	karyFactor := 3

	fmt.Printf("参数: GEO_PRECISION=%d, BUCKET_SIZE=%d, K0_THRESHOLD=%d, KARY_FACTOR=%d\n",
		geoPrec, bucketSize, k0Threshold, karyFactor)

	// 创建MERCATOR算法实例
	// 注意：这里真实坐标和显示坐标相同（无伪造）
	algo := algorithms.NewMercator(n, coords, coords, 0, geoPrec, bucketSize, k0Threshold, karyFactor)

	// 运行Mercator专用模拟（使用step==0时系数1的逻辑）
	//result := handlware.MercatorSimulation(reptTime, coords, attackConfig, algo, simConfig, nil)
	result := handlware.Simulation(reptTime, coords, attackConfig, algo, simConfig, nil)

	// 输出结果
	err := handlware.WriteSimulationResults("sim_output.csv", result, algo.GetAlgoName(), n, attackConfig.MaliciousRatio)
	if err != nil {
		log.Printf("写入结果失败: %v", err)
	}

	err = handlware.WriteFigData("fig.csv", result, algo.GetAlgoName())
	if err != nil {
		log.Printf("写入图表数据失败: %v", err)
	}
	//输出k桶信息
	err = handlware.WriteKBuckets("kbuckets.csv", algo.KBuckets, algo.NodeGeohash)
	if err != nil {
		log.Printf("写入k桶信息失败: %v", err)
	}
	// //输出前缀树信息
	// err = handlware.WriteGeohashComparison("geohash_comparison.csv", n, algo.NodeGeohash, algo.NodeGeohash, nil)
	// if err != nil {
	// 	log.Printf("写入Geohash对比信息失败: %v", err)
	// }

	elapsed := time.Since(startTime)
	fmt.Printf("MERCATOR 完成，耗时: %s\n", elapsed)
	fmt.Println("----------------------------------------")

}

// runMercury 运行MERCURY算法
func runMercury(n int, coords []handlware.LatLonCoordinate, vmodels []*handlware.VivaldiModel,
	clusterResult *handlware.ClusterResult, reptTime int, attackConfig *handlware.AttackConfig,
	simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Mercury参数
	rootFanout := 128
	secondFanout := 8
	fanout := 8
	innerDeg := 4
	enableNearest := true

	fmt.Printf("参数: ROOT_FANOUT=%d, FANOUT=%d, INNER_DEG=%d, ENABLE_NEAREST=%v\n",
		rootFanout, fanout, innerDeg, enableNearest)

	// 创建Mercury算法实例
	algo := algorithms.NewMercury(n, coords, vmodels, clusterResult, 0,
		rootFanout, secondFanout, fanout, innerDeg, enableNearest)

	// 运行模拟
	result := handlware.Simulation(reptTime, coords, attackConfig, algo, simConfig, clusterResult)

	// 输出结果
	err := handlware.WriteSimulationResults("sim_output.csv", result, algo.GetAlgoName(), n, attackConfig.MaliciousRatio)
	if err != nil {
		log.Printf("写入结果失败: %v", err)
	}

	err = handlware.WriteFigData("fig.csv", result, algo.GetAlgoName())
	if err != nil {
		log.Printf("写入图表数据失败: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("MERCURY 完成，耗时: %s\n", elapsed)
}

// runRandom 运行Random Flood算法
func runRandom(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	fmt.Println("\n运行 RANDOM FLOOD 算法...")
	startTime := time.Now()

	// 创建Random Flood算法实例
	algo := algorithms.NewRandomFlood(n, coords, 0, 8, 8)

	// 运行模拟
	result := handlware.Simulation(reptTime, coords, attackConfig, algo, simConfig, nil)

	// 输出结果
	err := handlware.WriteSimulationResults("sim_output.csv", result, algo.GetAlgoName(), n, attackConfig.MaliciousRatio)
	if err != nil {
		log.Printf("写入结果失败: %v", err)
	}

	err = handlware.WriteFigData("fig.csv", result, algo.GetAlgoName())
	if err != nil {
		log.Printf("写入图表数据失败: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("RANDOM FLOOD 完成，耗时: %s\n", elapsed)
}

// runBlockP2P 运行BlockP2P算法
func runBlockP2P(n int, coords []handlware.LatLonCoordinate, clusterResult *handlware.ClusterResult,
	reptTime int, attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	fmt.Println("\n运行 BLOCKP2P 算法...")
	startTime := time.Now()

	// 创建BlockP2P算法实例
	algo := algorithms.NewBlockP2P(n, coords, clusterResult, 0, 8)

	// 运行模拟
	result := handlware.Simulation(reptTime, coords, attackConfig, algo, simConfig, clusterResult)

	// 输出结果
	err := handlware.WriteSimulationResults("sim_output.csv", result, algo.GetAlgoName(), n, attackConfig.MaliciousRatio)
	if err != nil {
		log.Printf("写入结果失败: %v", err)
	}

	err = handlware.WriteFigData("fig.csv", result, algo.GetAlgoName())
	if err != nil {
		log.Printf("写入图表数据失败: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("BLOCKP2P 完成，耗时: %s\n", elapsed)
}

// runPerigee 运行Perigee算法（简化版）
func runPerigee(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	fmt.Println("\n运行 PERIGEE UCB 算法 (简化版)...")
	startTime := time.Now()

	// 创建Perigee算法实例
	algo := algorithms.NewPerigeeUCB(n, coords, 0, 6, 6, 8)

	// 运行模拟
	result := handlware.Simulation(reptTime, coords, attackConfig, algo, simConfig, nil)

	// 输出结果
	err := handlware.WriteSimulationResults("sim_output.csv", result, algo.GetAlgoName(), n, attackConfig.MaliciousRatio)
	if err != nil {
		log.Printf("写入结果失败: %v", err)
	}

	err = handlware.WriteFigData("fig.csv", result, algo.GetAlgoName())
	if err != nil {
		log.Printf("写入图表数据失败: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("PERIGEE UCB 完成，耗时: %s\n", elapsed)
}
