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
	// coords, err := handlware.ReadGeoCoordinates("./nodes_latlon.txt")
	coords, err := handlware.ReadGeoCoordinates("./Geo.txt")
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
	//reptTime := 1
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
	// fmt.Println("步骤 2/6: 运行 MERCATOR 算法...")
	// fmt.Println("----------------------------------------")
	// runMercator(n, coords, reptTime, attackConfig, simConfig)
	// fmt.Println()

	// // 3.1 运行 MERCATOR SAMPLED K0 算法
	// fmt.Println("步骤 2.1/6: 运行 MERCATOR SAMPLED K0 算法...")
	// fmt.Println("----------------------------------------")
	// runMercatorSampled(n, coords, reptTime, attackConfig, simConfig)
	// fmt.Println()

	// // 3.2 运行 MERCATOR-MERCURY 混合算法
	// fmt.Println("步骤 2.2/6: 运行 MERCATOR-MERCURY 算法...")
	// fmt.Println("----------------------------------------")
	// runMercatorMercury(n, coords, reptTime, attackConfig, simConfig)
	// fmt.Println()

	// 3.5 运行 MERCATOR ADAPTIVE 算法
	// fmt.Println("步骤 2.5/6: 运行 MERCATOR ADAPTIVE 算法...")
	// fmt.Println("----------------------------------------")
	// runMercatorAdaptive(n, coords, reptTime, attackConfig, simConfig)
	// fmt.Println()

	// 4. 生成虚拟坐标和聚类（用于 Mercury 和 BlockP2P）
	// fmt.Println("步骤 3/6: 生成虚拟坐标和聚类...")
	// fmt.Println("----------------------------------------")

	// ====== 对比测试：标准 Vivaldi vs Vivaldi++ ======
	fmt.Println("\n========================================")
	fmt.Println("  性能对比测试：标准 Vivaldi vs Vivaldi++")
	fmt.Println("========================================\n")

	// 1. 测试标准 Vivaldi
	fmt.Println("【测试 1/2】标准 Vivaldi")
	fmt.Println("----------------------------------------")
	vmodels := handlware.GenerateVirtualCoordinate(coords, 100, 3)
	fmt.Println()

	// 2. 测试 Vivaldi++（优化版）
	fmt.Println("【测试 2/2】Vivaldi++（优化版）")
	fmt.Println("----------------------------------------")
	config := handlware.NewVivaldiPlusPlusConfig()
	config.RandSeed = 100 // 使用相同种子保证公平对比
	vmodelsplusplus := handlware.GenerateVirtualCoordinatePlusPlus(coords, 100, config)
	fmt.Println()

	// ====== 可选：使用虚拟坐标进行传播测试 ======
	// 取消注释以下代码来测试传播性能

	// fmt.Println("\n【传播测试】使用标准 Vivaldi 坐标")
	// clusterResult := handlware.KMeansVirtual(vmodels, 8, 100, 13)
	// runMercury(n, coords, vmodels, clusterResult, 1, attackConfig, simConfig)
	// fmt.Println()

	// fmt.Println("【传播测试】使用 Vivaldi++ 坐标")
	// clusterResultplusplus := handlware.KMeansVirtual(vmodelsplusplus, 8, 100, 13)
	// runMercury(n, coords, vmodelsplusplus, clusterResultplusplus, 1, attackConfig, simConfig)

	// fmt.Println()
	// // 5.1 运行 Mercury_Local 算法
	// fmt.Println("步骤 4.1/6: 运行 MERCURY_LOCAL 算法...")
	// fmt.Println("----------------------------------------")
	// runMercuryLocal(n, coords, reptTime, attackConfig, simConfig)
	// fmt.Println()

	//自动参数调节（可选）
	fmt.Println("自动参数调节...")
	result, err := handlware.AutoTuneParameters(coords, 100, "vivaldi_plusplus_params.json")
	if err != nil {
		log.Fatalf("自动参数调节失败: %v", err)
	}
	fmt.Println("自动参数调节完成")
	fmt.Println("最优参数:")
	fmt.Println(result.Config)
	fmt.Println("最优误差分布:")
	fmt.Println(result.ErrorDist)
	fmt.Println()

	// 运行 Vivaldi++ 传播策略
	// fmt.Println("步骤 6/6: 运行 Vivaldi++ 传播策略...")
	// fmt.Println("----------------------------------------")
	// runVivaldiPlusPlusRelay(n, coords, 1, attackConfig, simConfig)
	// fmt.Println()

	// fmt.Println()
	// // 5.2 运行 Kadcast 算法
	// fmt.Println("步骤 4.2/6: 运行 KADCAST 算法...")
	// fmt.Println("----------------------------------------")
	// runKadcast(n, coords, reptTime, attackConfig, simConfig)
	// fmt.Println()

	// // 5.3 运行 ETH 算法
	// fmt.Println("步骤 4.3/6: 运行 ETH 算法...")
	// fmt.Println("----------------------------------------")
	// runETH(n, coords, reptTime, attackConfig, simConfig)
	// fmt.Println()

	// // 6. 运行其他对比算法
	// fmt.Println("步骤 5/6: 运行对比算法...")
	// fmt.Println("----------------------------------------")

	// // Random Flood
	// runRandom(n, coords, reptTime, attackConfig, simConfig)

	// // BlockP2P
	// runBlockP2P(n, coords, clusterResult, reptTime, attackConfig, simConfig)

	// // Perigee (简化版)
	// runPerigee(n, coords, reptTime, attackConfig, simConfig)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("   所有模拟完成！")
	fmt.Println("   结果已保存到:")
	fmt.Println("   - sim_output.csv (详细结果)")
	fmt.Println("   - fig.csv (图表数据)")
	fmt.Println("========================================")
}

// runMercator 运行MERCATOR算法
func runMercator(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Mercator参数--循环多组参数组合
	geoPrecVals := []int{1, 2, 3, 4, 5, 6, 7}
	bucketSizeVals := []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	k0ThresholdVals := []int{1}
	karyFactorVals := []int{2, 3, 4}

	for _, geoPrec := range geoPrecVals {
		for _, bucketSize := range bucketSizeVals {
			for _, k0Threshold := range k0ThresholdVals {
				for _, karyFactor := range karyFactorVals {

					fmt.Printf("参数: GEO_PRECISION=%d, BUCKET_SIZE=%d, K0_THRESHOLD=%d, KARY_FACTOR=%d\n",
						geoPrec, bucketSize, k0Threshold, karyFactor)

					// 创建MERCATOR算法实例
					// 注意：这里真实坐标和显示坐标相同（无伪造）
					algo := algorithms.NewMercator(n, coords, coords, 0, geoPrec, bucketSize, k0Threshold, karyFactor)

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
					fmt.Printf("完成参数: GEO_PRECISION=%d, BUCKET_SIZE=%d, K0_THRESHOLD=%d, KARY_FACTOR=%d\n",
						geoPrec, bucketSize, k0Threshold, karyFactor)
					fmt.Println("----------------------------------------")
				}
			}
		}
	}

	// //测试k0gossip策略
	// algoGossip := algorithms.NewMercatorGossip(algo, 8)
	// resultgossip := handlware.Simulation(reptTime, coords, attackConfig, algoGossip, simConfig, nil)
	// // 输出结果
	// err = handlware.WriteSimulationResults("sim_output.csv", resultgossip, algo.GetAlgoName(), n, attackConfig.MaliciousRatio)
	// if err != nil {
	// 	log.Printf("写入结果失败: %v", err)
	// }

	// err = handlware.WriteFigData("fig.csv", resultgossip, algo.GetAlgoName())
	// if err != nil {
	// 	log.Printf("写入图表数据失败: %v", err)
	// }
	// //输出k桶信息
	// err = handlware.WriteKBuckets("kbuckets.csv", algo.KBuckets, algo.NodeGeohash)
	// if err != nil {
	// 	log.Printf("写入k桶信息失败: %v", err)
	// }
	// //输出前缀树信息
	// err = handlware.WriteGeohashComparison("geohash_comparison.csv", n, algo.NodeGeohash, algo.NodeGeohash, nil)
	// if err != nil {
	// 	log.Printf("写入Geohash对比信息失败: %v", err)
	// }

	elapsed := time.Since(startTime)
	fmt.Printf("MERCATOR 完成，耗时: %s\n", elapsed)
	fmt.Println("----------------------------------------")
}

// runMercatorAdaptive 运行MERCATOR ADAPTIVE算法
func runMercatorAdaptive(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Mercator Adaptive参数
	initPrec := 1      // 初始精度
	maxPrec := 6       // 最大精度
	k0Threshold := 100 // k0桶阈值
	bucketSize := 6
	karyFactor := 3

	fmt.Printf("参数: INIT_PRECISION=%d, MAX_PRECISION=%d, K0_THRESHOLD=%d, BUCKET_SIZE=%d\n",
		initPrec, maxPrec, k0Threshold, bucketSize)

	// 创建MERCATOR ADAPTIVE算法实例
	algo := algorithms.NewMercatorAdaptive(n, coords, coords, 0, initPrec, maxPrec, k0Threshold, bucketSize, karyFactor)

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
	fmt.Printf("MERCATOR ADAPTIVE 完成，耗时: %s\n", elapsed)
	fmt.Println("----------------------------------------")
}

// runMercatorSampled 运行MERCATOR SAMPLED K0算法
func runMercatorSampled(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Mercator Sampled参数
	geoPrec := 3
	bucketSize := 6
	k0Threshold := 9999
	karyFactor := 3
	k0SampleSize := 10 // K0桶采样大小

	fmt.Printf("参数: GEO_PRECISION=%d, K0_SAMPLE_SIZE=%d\n", geoPrec, k0SampleSize)

	// 创建MERCATOR SAMPLED算法实例
	algo := algorithms.NewMercatorSampled(n, coords, coords, 0, geoPrec, bucketSize, k0Threshold, karyFactor, k0SampleSize)

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
	fmt.Printf("MERCATOR SAMPLED 完成，耗时: %s\n", elapsed)
	fmt.Println("----------------------------------------")
}

// runMercatorMercury 运行MERCATOR-MERCURY混合算法
func runMercatorMercury(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Mercator-Mercury参数
	geoPrec := 3
	bucketSize := 6
	k0Threshold := 9999
	karyFactor := 3
	k0SampleSize := 10 // K0桶采样大小
	hubFanout := 8     // Hub转发扇出

	fmt.Printf("参数: GEO_PRECISION=%d, K0_SAMPLE_SIZE=%d, HUB_FANOUT=%d\n",
		geoPrec, k0SampleSize, hubFanout)

	// 创建MERCATOR-MERCURY算法实例
	algo := algorithms.NewMercatorMercury(n, coords, coords, 0, geoPrec, bucketSize, k0Threshold, karyFactor, k0SampleSize, hubFanout)

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
	fmt.Printf("MERCATOR-MERCURY 完成，耗时: %s\n", elapsed)
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

// runMercuryLocal 运行MERCURY_LOCAL算法
func runMercuryLocal(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Mercury_Local参数
	neighborCount := 128 // 每个节点选择的邻居数量
	k := 8               // 局部聚类K值
	vivaldiRounds := 100 // Vivaldi更新轮数
	rootFanout := 128    // 根节点扇出度
	secondFanout := 8    // 第二层扇出度
	fanout := 8          // 普通节点扇出度
	innerDeg := 4        // 簇内连接度
	enableNearest := true

	fmt.Printf("参数: NEIGHBOR_COUNT=%d, K=%d, VIVALDI_ROUNDS=%d, ROOT_FANOUT=%d, FANOUT=%d, INNER_DEG=%d, ENABLE_NEAREST=%v\n",
		neighborCount, k, vivaldiRounds, rootFanout, fanout, innerDeg, enableNearest)

	// 创建MercuryLocal算法实例
	algo := algorithms.NewMercuryLocal(n, coords, 0, neighborCount, k, vivaldiRounds,
		rootFanout, secondFanout, fanout, innerDeg, enableNearest)

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
	fmt.Printf("MERCURY_LOCAL 完成，耗时: %s\n", elapsed)
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

// runPerigee 运行Perigee算法
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

// runKadcast 运行Kadcast算法
func runKadcast(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Kadcast参数配置
	config := handlware.KBucketConfig{
		K:       8,   // 每桶最大节点数
		Fanout:  6,   // 转发扇出 F
		NumBits: 128, // NodeID 位数
	}

	fmt.Printf("参数: K=%d, Fanout=%d, NumBits=%d\n", config.K, config.Fanout, config.NumBits)

	// 创建Kadcast算法实例
	algo := algorithms.NewKadcast(n, coords, config)

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
	fmt.Printf("KADCAST 完成，耗时: %s\n", elapsed)
	fmt.Println("----------------------------------------")
}

// runETH 运行ETH算法
func runETH(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// ETH参数配置
	config := handlware.KBucketConfig{
		K:       8,   // 每桶最大节点数
		Fanout:  2,   // 转发扇出 F
		NumBits: 128, // NodeID 位数
	}

	fmt.Printf("参数: K=%d, Fanout=%d, NumBits=%d\n", config.K, config.Fanout, config.NumBits)

	// 创建ETH算法实例
	algo := algorithms.NewETH(n, coords, config)

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
	fmt.Printf("ETH 完成，耗时: %s\n", elapsed)
	fmt.Println("----------------------------------------")
}

// runVivaldiPlusPlusRelay 运行 Vivaldi++ 传播策略算法
func runVivaldiPlusPlusRelay(n int, coords []handlware.LatLonCoordinate, reptTime int,
	attackConfig *handlware.AttackConfig, simConfig *handlware.SimulatorConfig) {

	startTime := time.Now()

	// Vivaldi++ 参数
	vivaldiConfig := handlware.NewVivaldiPlusPlusConfig()
	relayConfig := algorithms.NewDefaultRelayStrategyConfig()
	warmupRounds := 100
	txPerRound := 200

	fmt.Printf("参数: Vivaldi++ rounds=100, Warmup=%d轮×%d交易, D=%d, eta_rand=%.2f\n",
		warmupRounds, txPerRound, relayConfig.D, relayConfig.EtaRand)

	// 创建 Vivaldi++ 传播策略算法实例
	algo := algorithms.NewVivaldiPlusPlusRelay(n, coords, vivaldiConfig, relayConfig, warmupRounds, txPerRound)

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
	fmt.Printf("Vivaldi++ Relay 完成，耗时: %s\n", elapsed)
	fmt.Println("----------------------------------------")
}
