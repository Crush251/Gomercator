# MERCATOR 广播算法模拟器 (Go 版本)

本项目是 MERCATOR 广播算法的 Go 语言实现，完整复现了 C++ 版本的核心功能，并支持多种广播算法的对比测试。

## 🎯 项目概览

MERCATOR 是一种基于 Geohash 的地理感知 P2P 广播算法，通过 K 桶结构和智能路由策略实现低延迟、高覆盖率的消息传播。

### 支持的算法

1. **MERCATOR** - 基于 Geohash 的地理感知广播（核心算法）
2. **Mercury** - 基于 Vivaldi 虚拟坐标和聚类的广播
3. **Random Flood** - 随机泛洪基准算法
4. **BlockP2P** - 基于地理聚类的分块 P2P
5. **Perigee UCB** - 基于观测的动态邻居选择（简化版）

---

## 📁 项目结构

```
Gomercator/
├── go.mod                    # Go 模块定义
├── main.go                   # 主程序入口
├── ARCHITECTURE.md           # 架构设计文档
├── PROGRESS.md              # 开发进度报告
├── README.md                # 本文件
│
├── handlware/               # 核心实现包
│   ├── model.go            # 基础数据结构
│   ├── utils.go            # 工具函数
│   ├── algorithm.go        # 算法接口
│   ├── queue.go            # 消息优先队列
│   ├── io.go               # 文件 I/O
│   ├── statistics.go       # 统计分析
│   ├── simulator.go        # 模拟器核心
│   ├── clustering.go       # K-means 聚类
│   ├── vivaldi.go          # Vivaldi 虚拟坐标
│   ├── geohash.go          # Geohash 编解码
│   │
│   └── algorithms/         # 算法实现
│       ├── random.go       # Random Flood
│       ├── blockp2p.go     # BlockP2P
│       ├── perigee.go      # Perigee UCB
│       ├── mercury.go      # Mercury
│       └── mercator.go     # MERCATOR
│
└── ../Geo.txt              # 节点地理坐标数据
```

---

## 🚀 快速开始

### 前置要求

- Go 1.22.0 或更高版本
- 8GB+ RAM（用于 8000 节点的大规模模拟）

### 编译和运行

```bash
cd Gomercator

# 编译
go build -o mercator_sim

# 运行
./mercator_sim

# 或直接运行
go run main.go
```

### 预期输出

程序会依次运行以下步骤：

1. **读取地理坐标** - 从 `Geo.txt` 读取最多 8000 个节点坐标
2. **运行 MERCATOR** - 构建 K 桶并模拟广播
3. **生成虚拟坐标** - 为 Mercury 算法生成 Vivaldi 坐标
4. **运行 Mercury** - 基于虚拟坐标的广播
5. **运行对比算法** - Random, BlockP2P, Perigee

### 输出文件

- `sim_output.csv` - 详细的模拟结果（延迟百分位、深度分布等）
- `fig.csv` - 简化的图表数据（用于绘图）

---

## 📊 核心功能特性

### 1. **事件驱动模拟**
- 基于优先队列的消息传播模拟
- 精确的延迟模型：距离延迟 + 数据传输延迟 + 处理延迟（高斯噪声）

### 2. **MERCATOR 算法核心**
- Geohash 编码（精度可配置）
- K 桶结构（K0 到 Kn 桶）
- K-ary 树传播（K0 桶节点数超过阈值时）
- 智能路由策略：由近到远逐层扩散

### 3. **攻击场景支持**
- 恶意节点（拒绝转发）
- 节点离开（接收但不转发）
- 谎报坐标攻击（MERCATOR 专用）

### 4. **统计分析**
- 延迟百分位（5% - 100%）
- 深度累积分布
- 带宽消耗（重复消息率）
- 簇统计（每个簇的平均延迟和深度）

---

## ⚙️ 配置参数

### MERCATOR 参数

在 `main.go` 中的 `runMercator` 函数修改：

```go
geoPrec := 4        // Geohash 精度 (2-8)
bucketSize := 6     // K 桶大小 (4-14)
k0Threshold := 9999 // K0 桶阈值（超过则用 K-ary 树）
karyFactor := 3     // K-ary 树分支因子 (2-4)
```

### 模拟器参数

```go
reptTime := 1                    // 重复测试次数
malNode := 0.0                   // 恶意节点比例
simConfig.Bandwidth = 33000000.0 // 带宽 (bps)
simConfig.DataSize = 300.0       // 数据包大小 (Bytes)
```

---

## 🧪 测试和验证

### 单元测试（待实现）

```bash
go test ./handlware/...
```

### 与 C++ 版本对比

运行 Go 版本和 C++ 版本，对比以下指标：
- 延迟百分位（95%）
- 平均带宽消耗
- 深度分布

预期差异：
- 延迟差异：< 5%（浮点精度差异）
- 带宽差异：< 1%（图构建随机性）

---

## 📈 性能优化建议

### 1. **并行化**
当前实现是串行的，可通过 goroutine 并行化：
- 多个根节点的测试可并行执行
- K 桶填充可并行化

### 2. **内存优化**
- 预分配切片容量（避免动态扩容）
- 使用对象池复用消息对象

### 3. **性能分析**

```bash
# CPU profiling
go run -cpuprofile=cpu.prof main.go
go tool pprof cpu.prof

# Memory profiling
go run -memprofile=mem.prof main.go
go tool pprof mem.prof
```

---

## 🔧 扩展功能

### 添加新算法

1. 在 `handlware/algorithms/` 创建新文件
2. 实现 `Algorithm` 接口：
   ```go
   type Algorithm interface {
       Respond(msg *Message) []int
       SetRoot(root int)
       GetAlgoName() string
       NeedSpecifiedRoot() bool
   }
   ```
3. 在 `main.go` 添加运行函数

### 支持配置文件

可以添加 YAML/JSON 配置文件支持：
```yaml
simulation:
  repeat_times: 1
  max_nodes: 8000

mercator:
  geo_precision: 4
  bucket_size: 6

attack:
  malicious_ratio: 0.0
  fake_coord_ratio: 0.0
```

---

## 📝 代码质量

### 代码规范
- 遵循 Go 官方代码规范
- 所有导出函数都有完整注释（中文）
- 关键逻辑有详细说明

### 错误处理
- 所有文件 I/O 操作都有错误检查
- 边界情况处理完善

### 可测试性
- 模块化设计，便于单元测试
- 接口抽象清晰

---

## 🐛 已知问题

### Perigee 算法
- 当前为简化版本，未实现完整的 warmup phase
- 完整实现需要观测数据管理和动态邻居重选

### 性能
- 大规模网络（8000+ 节点）的 K 桶填充较慢
- 后续可优化为并行化实现

---

## 📚 相关文档

- `ARCHITECTURE.md` - 详细的架构设计和模块说明
- `PROGRESS.md` - 开发进度和工作量统计
- C++ 版本 README - 原始实现说明

---

## 🤝 贡献指南

### 提交代码
1. Fork 项目
2. 创建功能分支
3. 提交代码并通过测试
4. 发起 Pull Request

### 报告问题
- 使用 GitHub Issues
- 提供详细的复现步骤
- 附上日志和错误信息

---

## 📄 许可证

本项目遵循原 C++ 版本的许可证。

---

## 👥 作者

- **C++ 版本**: [原始作者]
- **Go 移植**: Google 团队首席架构师（AI 辅助）

---

## 🎉 致谢

感谢原 C++ 版本的开发者，为 Go 版本的实现提供了清晰的参考。

---

## 📞 联系方式

如有问题或建议，请联系：
- Email: [你的邮箱]
- GitHub: [你的 GitHub]

---

最后更新：2025-01-08

