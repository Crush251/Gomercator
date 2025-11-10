# MERCATOR Go 实现架构分析

## 一、共性模块提取（从 sim.cpp）

### 1. 基础数据结构模块 (model.go)

#### 1.1 坐标相关
```go
// 经纬度坐标
type LatLonCoordinate struct {
    Lat float64
    Lon float64
}

// 虚拟坐标（Vivaldi）
type VivaldiCoordinate struct {
    Vector []float64  // D维向量
    Height float64
    Error  float64
}
```

#### 1.2 消息结构
```go
type Message struct {
    Root     int     // 广播根节点
    Src      int     // 源节点
    Dst      int     // 目标节点
    Step     int     // 传播步数
    SendTime float64 // 发送时间
    RecvTime float64 // 接收时间
}
```

#### 1.3 图结构
```go
type Graph struct {
    N        int         // 节点数
    M        int         // 边数
    InBound  [][]int     // 入边列表
    OutBound [][]int     // 出边列表
}

// 方法：
// - AddEdge(u, v int) bool
// - DelEdge(u, v int)
// - Outbound(u int) []int
// - Inbound(u int) []int
```

#### 1.4 测试结果结构
```go
type TestResult struct {
    AvgBandwidth     float64      // 平均带宽消耗（重复率）
    AvgLatency       float64      // 平均延迟
    Latency          []float64    // 不同百分位的延迟 (5%, 10%, ..., 100%)
    DepthCDF         []float64    // 深度CDF
    AvgDist          []float64    // 每层平均距离
    ClusterAvgLatency []float64   // 每个簇的平均延迟
    ClusterAvgDepth   []float64   // 每个簇的平均深度
}
```

### 2. 工具函数模块 (utils.go)

#### 2.1 距离计算
```go
// 计算两点间的地理距离
func Distance(a, b LatLonCoordinate) float64

// 角度转弧度
func Rad(deg float64) float64

// 环状经度差值处理
func FitInRing(x float64) float64

// 角度检查（用于Mercator方向判断）
func AngleCheck(r, u, v LatLonCoordinate) bool
```

#### 2.2 随机数工具
```go
func RandomNum(n int) int
func RandomBetween01() float64
```

### 3. 聚类模块 (clustering.go)

#### 3.1 K-means聚类
```go
// 基于真实坐标的K-means
func KMeans(coords []LatLonCoordinate, k int, maxIter int) ClusterResult

// 基于虚拟坐标的K-means
func KMeansVirtual(vcoords []VivaldiCoordinate, k int, maxIter int) ClusterResult

type ClusterResult struct {
    ClusterID   []int      // 每个节点所属簇ID
    ClusterList [][]int    // 每个簇包含的节点列表
    ClusterCnt  []int      // 每个簇的节点数
}
```

### 4. Vivaldi虚拟坐标模块 (vivaldi.go)

#### 4.1 Vivaldi模型
```go
type VivaldiModel struct {
    NodeID       int
    Coord        VivaldiCoordinate
    PeerSet      []int
    HaveEnoughPeer bool
}

// 方法：
// - Observe(peer int, peerCoord VivaldiCoordinate, rtt float64)
// - UpdateCoordinate(...)
```

#### 4.2 生成虚拟坐标
```go
// 基于真实RTT生成虚拟坐标
func GenerateVirtualCoordinate(coords []LatLonCoordinate, rounds int) []VivaldiModel

// 随机生成虚拟坐标
func GenerateRandomVirtualCoordinate(n int) []VivaldiModel
```

### 5. 消息传播模拟器 (simulator.go)

#### 5.1 核心模拟函数
```go
// 单根节点模拟
func SingleRootSimulation(
    root int,
    reptTime int,
    malNode float64,
    algo Algorithm,
    coords []LatLonCoordinate,
    malFlags []bool,
) TestResult

// 多根节点模拟（随机选择20个根节点）
func Simulation(
    reptTime int,
    malNode float64,
    algo Algorithm,
    coords []LatLonCoordinate,
) TestResult
```

#### 5.2 延迟模型
```go
const (
    FIXED_DELAY = 250.0  // 固定延迟（ms）
    R           = 6371000.0 // 地球半径（m）
)

// 计算传播延迟
func CalculateDelay(u, v int, coords []LatLonCoordinate, bandwidth, dataSize float64) float64 {
    // 距离延迟 + 数据传输延迟
    distDelay := Distance(coords[u], coords[v]) * 3
    dataLatency := (dataSize * 8 / bandwidth) * 1000
    return distDelay + dataLatency
}
```

#### 5.3 消息队列管理
```go
// 使用优先队列（按RecvTime排序）
type MessageQueue struct {
    messages []Message
}

// 方法：
// - Push(msg Message)
// - Pop() Message
// - Empty() bool
```

### 6. 统计分析模块 (statistics.go)

#### 6.1 Percentile计算
```go
// 计算延迟的不同百分位
func CalculatePercentiles(recvTimes []float64, percentiles []float64) []float64
```

#### 6.2 深度和带宽统计
```go
// 统计深度分布
func CalculateDepthCDF(depths []int, maxDepth int) []float64

// 计算带宽消耗（重复率）
func CalculateBandwidth(dupMsgCount, nodeCount int) float64
```

### 7. 攻击模拟模块 (attack.go)

#### 7.1 恶意节点
```go
type AttackConfig struct {
    MaliciousRatio    float64  // 恶意节点比例
    FakeCoordRatio    float64  // 谎报坐标节点比例
    NodeLeaveRatio    float64  // 节点离开比例
}

// 生成恶意节点标记
func GenerateMaliciousNodes(n int, ratio float64) []bool

// 生成节点离开标记（接收但不转发）
func GenerateLeaveNodes(n int, ratio float64) []bool
```

#### 7.2 谎报坐标攻击（Mercator专用）
```go
// 生成伪造坐标
func GenerateFakeCoordinates(coords []LatLonCoordinate, ratio float64, offsetDegree float64) ([]LatLonCoordinate, []bool)

type FakeCoordConfig struct {
    Ratio        float64  // 谎报比例
    OffsetDegree float64  // 偏移度数（±10, ±20, ±30, 或完全随机）
}
```

### 8. 算法接口模块 (algorithm.go)

#### 8.1 基础算法接口
```go
type Algorithm interface {
    // 响应消息，返回转发节点列表
    Respond(msg Message) []int
    
    // 设置广播根节点
    SetRoot(root int)
    
    // 获取算法名称
    GetAlgoName() string
    
    // 是否需要指定根节点重建
    NeedSpecifiedRoot() bool
}
```

### 9. 文件I/O模块 (io.go)

#### 9.1 输入
```go
// 读取地理坐标
func ReadGeoCoordinates(filename string) ([]LatLonCoordinate, error)
```

#### 9.2 输出
```go
// 输出模拟结果到CSV
func WriteSimulationResults(filename string, results []TestResult, algoName string)

// 输出图表数据
func WriteFigData(filename string, results []TestResult, algoName string)

// 输出树结构
func WriteTreeStructure(filename string, parents []int, root int)

// 输出Geohash对比（Mercator）
func WriteGeohashComparison(filename string, realHash, fakeHash []string, flags []bool)

// 输出K桶信息（Mercator）
func WriteKBuckets(filename string, kBuckets [][][]int, nodeGeohash []string)
```

### 10. Geohash模块 (geohash.go - Mercator专用)

#### 10.1 Geohash编码/解码
```go
const BASE32 = "0123456789bcdefghjkmnpqrstuvwxyz"

type Geohash struct {
    Precision int
}

// 方法：
// - Encode(lat, lon float64) string
// - Decode(hash string) (lat, lon float64)
// - ToBinary(hash string) string
// - GetNeighbors(hash string) []string
```

#### 10.2 K桶和前缀树
```go
// K桶索引计算
func GetGeoBucketIndex(hashA, hashB string) int

// XOR距离计算
func XorDistance(binaryA, binaryB string) uint

// 前缀树节点
type GeoPrefixNode struct {
    Prefix   string
    NodeIds  []int
    Children map[rune]*GeoPrefixNode
}

// K-ary树子节点计算
func ComputeKaryChildren(nodeIdx, totalNodes, k int) []int
```

---

## 二、各算法实现模块

### 1. Random Flood (random.go)
```go
type RandomFlood struct {
    graph      *Graph
    treeRoot   int
    rootFanout int
    fanout     int
}
```

### 2. Block P2P (blockp2p.go)
```go
type BlockP2P struct {
    graph         *Graph
    clusterResult *ClusterResult
}
```

### 3. Perigee (perigee.go)
```go
type PerigeeUBC struct {
    graph          *Graph
    observations   [][][]float64  // 观测数据
    warmupRounds   int
    rootFanout     int
    fanout         int
}
```

### 4. Mercury (mercury.go)
```go
type Mercury struct {
    graph         *Graph
    clusterResult *ClusterResult
    vivaldiModels []VivaldiModel
    rootFanout    int
    secondFanout  int
    fanout        int
    enableNearest bool
}
```

### 5. Mercator (mercator.go)
```go
type Mercator struct {
    graph           *Graph
    nodeGeohash     []string
    kBuckets        [][][]int
    geohashGroups   map[string][]int
    prefixTree      *GeoPrefixNode
    treeRoot        int
    visited         [][]bool
    k0Threshold     int
    karyFactor      int
    bucketSize      int
}
```

---

## 三、目录结构建议

```
Gomercator/
├── go.mod
├── main.go                    # 主程序入口
├── handlware/                 # 核心实现包
│   ├── model.go              # 基础数据结构
│   ├── utils.go              # 工具函数
│   ├── clustering.go         # 聚类算法
│   ├── vivaldi.go            # Vivaldi虚拟坐标
│   ├── simulator.go          # 消息传播模拟器
│   ├── statistics.go         # 统计分析
│   ├── attack.go             # 攻击模拟
│   ├── algorithm.go          # 算法接口
│   ├── io.go                 # 文件I/O
│   ├── geohash.go           # Geohash（Mercator）
│   ├── network.go           # 图结构和网络构建
│   ├── propagtion.go        # 传播策略（拼写保持原样）
│   │
│   ├── algorithms/          # 各算法实现
│   │   ├── random.go       # Random Flood
│   │   ├── blockp2p.go     # Block P2P
│   │   ├── perigee.go      # Perigee
│   │   ├── mercury.go      # Mercury
│   │   └── mercator.go     # Mercator
│   │
│   └── tests/              # 单元测试
│       ├── utils_test.go
│       ├── clustering_test.go
│       └── ...
│
├── configs/                 # 配置文件
│   └── config.yaml
│
├── data/                    # 数据文件
│   └── Geo.txt
│
└── output/                  # 输出文件
    ├── sim_output.csv
    ├── fig.csv
    └── ...
```

---

## 四、常量定义

```go
const (
    K            = 8        // 聚类数量
    MAX_DEPTH    = 40       // 最大深度
    FIXED_DELAY  = 250.0    // 固定延迟（ms）
    ROOT_FANOUT  = 64       // 根节点扇出
    SECOND_FANOUT = 64      // 第二层扇出
    FANOUT       = 8        // 普通节点扇出
    INNER_DEG    = 4        // 簇内连接度
    MAX_OUTBOUND = 8        // 最大出度
    R            = 6371000.0 // 地球半径（m）
    
    // Mercator专用
    GEO_BITS_PER_CHAR = 5
    GEO_PRECISION     = 4
    K0_BUCKET_THRESHOLD = 15
    KARY_FACTOR       = 3
    MAX_BUCKET_SIZE   = 10
    
    // 数据传输
    BANDWIDTH_DEFAULT = 33000000.0 // 33 Mbps
    DATA_SIZE_SMALL   = 300.0      // 300 Bytes
    DATA_SIZE_LARGE   = 1048576.0  // 1 MB
)
```

---

## 五、实现优先级

### Phase 1: 基础设施（共性模块）
1. ✅ model.go - 基础数据结构
2. ✅ utils.go - 工具函数
3. ✅ network.go - 图结构
4. ✅ io.go - 文件I/O
5. ✅ simulator.go - 模拟器框架
6. ✅ statistics.go - 统计分析

### Phase 2: 核心算法支撑
7. ✅ clustering.go - K-means聚类
8. ✅ vivaldi.go - 虚拟坐标
9. ✅ geohash.go - Geohash（Mercator）
10. ✅ attack.go - 攻击模拟

### Phase 3: 算法实现
11. ✅ algorithms/random.go
12. ✅ algorithms/blockp2p.go
13. ✅ algorithms/perigee.go
14. ✅ algorithms/mercury.go
15. ✅ algorithms/mercator.go

### Phase 4: 集成和测试
16. ✅ main.go - 主程序
17. ✅ 参数扫描功能
18. ✅ 单元测试
19. ✅ 性能优化

---

## 六、关键差异点（C++ vs Go）

### 1. 内存管理
- C++: 手动管理，使用指针和new/delete
- Go: 自动GC，使用切片和引用

### 2. 模板 vs 泛型
- C++: 模板参数（如 `mercator<128, 8, 8, 10, 3>`）
- Go: 使用结构体字段配置，Go 1.18+支持泛型但较简单

### 3. 继承 vs 接口
- C++: 虚函数和继承（`basic_algo`基类）
- Go: 接口和组合（`Algorithm`接口）

### 4. 优先队列
- C++: `std::priority_queue`
- Go: 使用 `container/heap` 包

### 5. 随机数
- C++: `mt19937`, `std::normal_distribution`
- Go: `math/rand` 或 `math/rand/v2`

---

## 七、测试策略

### 1. 单元测试
- 距离计算精度测试
- K-means收敛性测试
- Geohash编码/解码测试

### 2. 集成测试
- 小规模网络（100节点）
- 中等规模网络（1000节点）
- 大规模网络（8000节点）

### 3. 性能基准
- 与C++版本结果对比
- 延迟分布对比
- 带宽消耗对比

### 4. 攻击场景测试
- 恶意节点测试
- 节点离开测试
- 谎报坐标测试

---

## 八、配置化建议

```yaml
# config.yaml
simulation:
  repeat_times: 1
  max_nodes: 8000
  test_nodes: 20

network:
  bandwidth: 33000000  # bps
  data_size: 300       # bytes
  fixed_delay: 250     # ms

clustering:
  k: 8
  max_iter: 100

attack:
  malicious_ratio: 0.0
  fake_coord_ratio: 0.0
  node_leave_ratio: 0.0

mercator:
  geo_precision: 4
  bucket_size: 10
  k0_threshold: 15
  kary_factor: 3

output:
  sim_output: "mercator_sim_output.csv"
  fig_output: "fig.csv"
  tree_struct: "tree_struct.txt"
```

---

## 九、注意事项

1. **浮点数精度**：Go和C++浮点运算可能有微小差异
2. **随机数种子**：确保相同种子产生相同结果
3. **并发安全**：Go可以利用goroutine并行化，但需注意数据竞争
4. **错误处理**：Go强制错误处理，C++使用异常
5. **性能优化**：关键路径可能需要profile优化

---

## 十、下一步计划

1. **先实现共性模块**（Phase 1）
2. **逐个实现算法**（从简单到复杂）：Random → BlockP2P → Perigee → Mercury → Mercator
3. **对比验证**：每个算法实现后与C++版本对比结果
4. **优化和重构**：根据测试结果优化性能
5. **文档和示例**：补充使用文档和示例代码

