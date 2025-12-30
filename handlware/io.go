package handlware

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ==================== 输入函数 ====================

// ReadGeoCoordinates 从文件读取地理坐标
// 文件格式:
// 第一行: 节点数量 n
// 接下来n行: 每行两个浮点数，表示纬度和经度
func ReadGeoCoordinates(filename string) ([]LatLonCoordinate, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件 %s: %v", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// 读取第一行：节点数量
	if !scanner.Scan() {
		return nil, fmt.Errorf("文件为空")
	}

	n, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil {
		return nil, fmt.Errorf("无法解析节点数量: %v", err)
	}

	coords := make([]LatLonCoordinate, n)

	// 读取坐标
	for i := 0; i < n; i++ {
		if !scanner.Scan() {
			return nil, fmt.Errorf("坐标数据不完整，期望%d个节点，只读取到%d个", n, i)
		}

		line := strings.TrimSpace(scanner.Text())
		parts := strings.Fields(line)

		if len(parts) != 2 {
			return nil, fmt.Errorf("第%d行格式错误: %s", i+2, line)
		}

		lat, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return nil, fmt.Errorf("第%d行纬度解析失败: %v", i+2, err)
		}

		lon, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("第%d行经度解析失败: %v", i+2, err)
		}

		coords[i] = LatLonCoordinate{Lat: lat, Lon: lon}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件出错: %v", err)
	}

	return coords, nil
}

// ==================== 输出函数 ====================

// WriteSimulationResults 写入模拟结果到CSV文件
func WriteSimulationResults(filename string, result *TestResult, algoName string, n int, malNode float64) error {
	// 以追加模式打开文件
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %v", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// 写入算法名称
	fmt.Fprintf(writer, "%s\n", algoName)

	// 写入表头
	fmt.Fprintf(writer, "#node, mal node, Bandwidth, ")
	for p := 0.05; p <= 1.0; p += 0.05 {
		fmt.Fprintf(writer, "%.2f, ", p)
	}
	fmt.Fprintf(writer, "\n")

	// 写入数据
	fmt.Fprintf(writer, "%d, %.2f, %.2f, ", n, malNode, result.AvgBandwidth)
	for i := 0; i < len(result.Latency); i++ {
		fmt.Fprintf(writer, "%.2f, ", result.Latency[i])
	}
	fmt.Fprintf(writer, "\n")

	// 写入深度分布
	fmt.Fprintf(writer, "depth pdf\n")
	for i := 0; i < MaxDepth; i++ {
		fmt.Fprintf(writer, "%d, ", i)
	}
	fmt.Fprintf(writer, "\n")

	avgDepth := 0.0
	for i := 0; i < MaxDepth; i++ {
		fmt.Fprintf(writer, "%.4f, ", result.DepthCDF[i])
		avgDepth += result.DepthCDF[i] * float64(i)
	}
	fmt.Fprintf(writer, "\n")

	// 写入平均统计
	fmt.Fprintf(writer, "avg depth = %.2f\n", avgDepth)
	fmt.Fprintf(writer, "avg latency = %.2f\n", result.AvgLatency)

	// 写入簇统计
	fmt.Fprintf(writer, "cluster avg depth\n")
	for i := 0; i < K; i++ {
		fmt.Fprintf(writer, "%.2f, ", result.ClusterAvgDepth[i])
	}
	fmt.Fprintf(writer, "\n")

	fmt.Fprintf(writer, "cluster avg latency\n")
	for i := 0; i < K; i++ {
		fmt.Fprintf(writer, "%.2f, ", result.ClusterAvgLatency[i])
	}
	fmt.Fprintf(writer, "\n")

	// 写入每层平均距离
	fmt.Fprintf(writer, "avg distance by depth\n")
	for i := 0; i < MaxDepth; i++ {
		fmt.Fprintf(writer, "%.2f, ", result.AvgDist[i])
	}
	fmt.Fprintf(writer, "\n\n")

	return nil
}

// WriteFigData 写入图表数据（简化版，用于绘图）
func WriteFigData(filename string, result *TestResult, algoName string) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %v", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	fmt.Fprintf(writer, "%s, ", algoName)
	//写入平均带宽
	fmt.Fprintf(writer, "%.2f, ", result.AvgBandwidth)
	for i := 0; i < len(result.Latency); i++ {
		fmt.Fprintf(writer, "%.2f, ", result.Latency[i])
	}
	fmt.Fprintf(writer, "\n")

	return nil
}

// WriteTreeStructure 写入树结构到文件
func WriteTreeStructure(filename string, n, root int, parents []int) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("无法创建文件 %s: %v", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// 第一行：节点数和根节点
	fmt.Fprintf(writer, "%d %d\n", n, root)

	// 后续行：每个节点的父节点
	for i := 0; i < n; i++ {
		fmt.Fprintf(writer, "%d\n", parents[i])
	}

	return nil
}

// WriteGeohashComparison 写入Geohash对比信息（Mercator专用）
func WriteGeohashComparison(filename string, n int, realHash, fakeHash []string, flags []bool) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("无法创建文件 %s: %v", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	fmt.Fprintf(writer, "节点ID,真实Geohash,伪造Geohash,是否伪造\n")

	// 写入数据
	for i := 0; i < n; i++ {
		isFake := 0
		if flags[i] {
			isFake = 1
		}
		fmt.Fprintf(writer, "%d,%s,%s,%d\n", i, realHash[i], fakeHash[i], isFake)
	}

	return nil
}

// WriteKBuckets 写入K桶信息（Mercator专用）
func WriteKBuckets(filename string, kBuckets [][][]int, nodeGeohash []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("无法创建文件 %s: %v", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	fmt.Fprintf(writer, "NodeID,Geohash,BucketID,BucketSize,BucketNodes\n")

	// 限制导出节点数
	maxNodes := len(kBuckets)
	fmt.Printf("导出前%d个节点的K桶信息到 %s...\n", maxNodes, filename)

	totalBuckets := 0
	emptyBuckets := 0

	for i := 0; i < maxNodes; i++ {
		for j := 0; j < len(kBuckets[i]); j++ {
			totalBuckets++
			if len(kBuckets[i][j]) == 0 {
				emptyBuckets++
				fmt.Fprintf(writer, "%d,%s,%d,0,\n", i, nodeGeohash[i], j)
			} else {
				fmt.Fprintf(writer, "%d,%s,%d,%d,", i, nodeGeohash[i], j, len(kBuckets[i][j]))
				for k := 0; k < len(kBuckets[i][j]); k++ {
					fmt.Fprintf(writer, "%d", kBuckets[i][j][k])
					if k < len(kBuckets[i][j])-1 {
						fmt.Fprintf(writer, "|")
					}
				}
				fmt.Fprintf(writer, "\n")
			}
		}

		if i%100 == 0 && i > 0 {
			fmt.Printf("已导出 %d/%d 节点 (%.1f%%)\n", i, maxNodes, float64(i)*100.0/float64(maxNodes))
		}
	}

	// 写入摘要
	fmt.Fprintf(writer, "\nSummary:\n")
	fmt.Fprintf(writer, "Total buckets analyzed: %d\n", totalBuckets)
	if totalBuckets > 0 {
		fmt.Fprintf(writer, "Empty buckets: %d (%.1f%%)\n", emptyBuckets, float64(emptyBuckets)*100.0/float64(totalBuckets))
	}

	fmt.Printf("K桶信息导出完成: %s\n", filename)
	return nil
}

// WriteMercatorResults 写入Mercator专用结果（包含参数信息）
func WriteMercatorResults(filename string, result *TestResult, n int, malNode, fakeCoordRatio float64,
	geoPrecision, bucketSize, k0Threshold, karyFactor int) error {

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %v", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// 写入算法名称和参数
	fmt.Fprintf(writer, "mercator\n")
	fmt.Fprintf(writer, "GEO_PRECISION=%d, BUCKET_SIZE=%d, K0_THRESHOLD=%d, KARY_FACTOR=%d, FAKE_COORD=%.1f%%\n",
		geoPrecision, bucketSize, k0Threshold, karyFactor, fakeCoordRatio*100)

	// 写入表头
	fmt.Fprintf(writer, "#node, mal node, fake coord, Bandwidth, ")
	for p := 0.05; p <= 1.0; p += 0.05 {
		fmt.Fprintf(writer, "%.2f, ", p)
	}
	fmt.Fprintf(writer, "\n")

	// 写入数据
	fmt.Fprintf(writer, "%d, %.2f, %.2f, %.2f, ", n, malNode, fakeCoordRatio, result.AvgBandwidth)
	for i := 0; i < len(result.Latency); i++ {
		fmt.Fprintf(writer, "%.2f, ", result.Latency[i])
	}
	fmt.Fprintf(writer, "\n\n")

	return nil
}

// WriteMercatorFigData 写入Mercator图表数据
func WriteMercatorFigData(filename string, result *TestResult, n int, malNode, fakeCoordRatio float64,
	geoPrecision, bucketSize, k0Threshold, karyFactor int) error {

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %v", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	fmt.Fprintf(writer, "mercator, %d, %.2f, %.2f, %.2f, GEO=%d, BKT=%d, K0T=%d, KARY=%d, ",
		n, malNode, fakeCoordRatio, result.AvgBandwidth, geoPrecision, bucketSize, k0Threshold, karyFactor)

	// 只输出前20个百分位（0.05-1.00）
	for i := 0; i < Min(20, len(result.Latency)); i++ {
		fmt.Fprintf(writer, "%.3f, ", result.Latency[i])
	}
	fmt.Fprintf(writer, "\n")

	return nil
}

// WriteSuccessChildrenCSV 写入成功转发子节点信息到CSV文件
func WriteSuccessChildrenCSV(path string, root int, success [][]int) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	//f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// 标题：Root,Src,NumChildren,Children(comma)
	fmt.Fprintln(f, "Root,Src,NumChildren,Children")
	for src := range success {
		kids := success[src]
		if len(kids) == 0 {
			continue
		}
		fmt.Fprintf(f, "%d,%d,%d,", root, src, len(kids))
		for i, v := range kids {
			if i > 0 {
				fmt.Fprint(f, "|")
			} // 用 | 分隔子节点，和你们的导出风格一致
			fmt.Fprint(f, v)
		}
		fmt.Fprintln(f)
	}
	return nil
}

// WriteXorAnchorRecords 将XOR锚点记录写入CSV文件
// 参数:
//   - filename: 输出文件名
//   - records: XOR锚点记录切片
//
// 返回: 错误信息（如果有）
func WriteXorAnchorRecords(filename string, records []XorAnchorRecord) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 写入CSV头
	header := "NodeID,CharPos,UChar,VChar,XorValue,AddedNodeID,BucketID,UGeohash,VGeohash,Description\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("写入表头失败: %v", err)
	}

	// 写入每条记录
	for _, record := range records {
		// 生成描述信息
		description := fmt.Sprintf("字符位%d: '%c' XOR '%c' = %d, 放入桶%d",
			record.CharPos,
			record.UChar,
			record.VChar,
			record.XorValue,
			record.BucketID)

		row := fmt.Sprintf("%d,%d,%c,%c,%d,%d,%d,%s,%s,%s\n",
			record.NodeID,
			record.CharPos,
			record.UChar,
			record.VChar,
			record.XorValue,
			record.AddedNodeID,
			record.BucketID,
			record.UGeohash,
			record.VGeohash,
			description)

		if _, err := file.WriteString(row); err != nil {
			return fmt.Errorf("写入记录失败: %v", err)
		}
	}

	fmt.Printf("✓ XOR锚点记录已保存到 %s，共 %d 条记录\n", filename, len(records))
	return nil
}

// XorAnchorRecord 记录XOR锚点添加的详细信息
type XorAnchorRecord struct {
	NodeID      int    // 当前节点ID
	CharPos     int    // 字符位置（0-based）
	UChar       byte   // 当前节点的字符
	VChar       byte   // 被添加节点的字符
	XorValue    int    // XOR值（5/10/15）
	AddedNodeID int    // 被添加的节点ID
	BucketID    int    // 放入的K桶编号
	UGeohash    string // 当前节点的完整Geohash
	VGeohash    string // 被添加节点的完整Geohash
}
