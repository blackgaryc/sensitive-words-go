# sensitive-words

`sensitive-words` 是一个本地离线可运行的 Go 敏感词检测包，适合在服务端、任务程序、审核工具或内部系统中直接嵌入使用。

它面向“本地词库 + 文本判定”场景，支持：

- 从本地目录或文件离线加载敏感词
- 运行时动态增加一个或多个敏感词
- 查询当前内存中的敏感词数量
- 判断文本是否包含敏感词
- 返回命中的词、命中方式、位置、来源和分类

当前实现基于 `Aho-Corasick` 多模式匹配自动机，并在匹配前增加归一化处理，以兼顾性能和基础反规避能力。

## 目录

- [设计目标](#设计目标)
- [核心特性](#核心特性)
- [算法说明](#算法说明)
- [匹配策略](#匹配策略)
- [安装与环境](#安装与环境)
- [快速开始](#快速开始)
- [API 说明](#api-说明)
- [返回结果说明](#返回结果说明)
- [默认行为](#默认行为)
- [并发与更新策略](#并发与更新策略)
- [性能建议](#性能建议)
- [适用边界](#适用边界)

## 设计目标

本项目的目标不是构建一个完整的内容审核平台，而是提供一层稳定、简单、可嵌入的本地敏感词检测能力：

- 以离线词库为基础，不依赖远程服务
- 优先解决大词库场景下的匹配性能问题
- 提供最基础的规避写法识别能力
- 提供可变词库接口，便于运行时热补充
- 保持接口简单，便于集成到其他 Go 项目

## 核心特性

- 离线加载词库
  - 支持从本地目录批量加载 `*.txt`
  - 支持从本地单个文件加载
- 动态维护词库
  - 支持运行时增加单个词或多个词
  - 支持带分类和来源信息的词条写入
- 命中检测
  - `Contains` 用于快速得到是否违规
  - `Detect` 用于获取详细命中结果
- 统计能力
  - `Count()` 返回当前内存中的唯一词数
  - `Stats()` 返回词条数、词数和自动机模式数
- 并发安全
  - 读操作支持并发
  - 写操作会重建自动机，并通过锁保护一致性

## 算法说明

本项目选择 `Aho-Corasick` 自动机作为核心匹配算法。

原因如下：

- 词库规模较大时，逐词 `strings.Contains` 或逐词正则匹配会随着词条数线性变慢
- `Aho-Corasick` 适合“一段文本 against 大量关键词”的场景
- 自动机构建完成后，单次检测复杂度接近 `O(文本长度 + 命中数)`
- 相比只用 Trie，`Aho-Corasick` 内置失败指针，更适合多模式匹配

这意味着它更适合做：

- 内容发布前的文本初筛
- 评论、昵称、帖子、消息等文本的本地拦截
- 批量离线扫描任务

## 匹配策略

当前实现使用两层匹配策略：

### 1. `strict` 匹配

对输入文本和词条做基础归一化：

- 英文转小写
- 全角字符折叠为半角
- 去除控制字符

在 `strict` 模式下，保留正文中的空格、标点和符号。

适合识别正常书写的敏感词或原始 URL、短语、固定词条。

### 2. `compact` 匹配

在 `strict` 基础上，进一步移除：

- 空格
- 标点
- 符号

适合识别带干扰字符的规避写法，例如：

- `习-近-平`
- `法.轮.功`
- `a b c`

### 命中结果裁剪

真实词库中可能包含大量片段词、变体词和通用片段。为了降低噪声，当前实现会对重叠命中做裁剪，只保留优先级更高的结果。

优先级主要依据：

- 命中跨度更长的优先
- 在同等条件下，`strict` 命中优先于 `compact`

## 安装与环境

当前项目是一个 Go 包，`go.mod` 中的模块名为：

```go
module github.com/blackgaryc/sensitive-words-go
```

开发环境要求：

- Go `1.26.0` 或兼容版本

如果你在同一仓库内直接使用，按普通 Go 包方式引入即可：

```go
import sensitivewords "github.com/blackgaryc/sensitive-words-go"
```

## 快速开始

### 方式一：启动时一次性加载本地词库

```go
package main

import (
	"fmt"
	"log"

	sensitivewords "github.com/blackgaryc/sensitive-words-go"
)

func main() {
	matcher, err := sensitivewords.NewMatcherFromDir(
		"/home/alex/git/Sensitive-lexicon/Vocabulary",
		sensitivewords.Options{},
	)
	if err != nil {
		log.Fatal(err)
	}

	result := matcher.Detect("测试文本：习-近-平")
	fmt.Println(result.Violates)
	fmt.Println(matcher.Count())

	for _, match := range result.Matches {
		fmt.Printf("%s %q %v\n", match.Mode, match.Matched, match.Categories)
	}
}
```

### 方式二：先创建空实例，再离线加载和动态增词

```go
package main

import (
	"fmt"
	"log"

	sensitivewords "github.com/blackgaryc/sensitive-words-go"
)

func main() {
	matcher := sensitivewords.NewEmptyMatcher(sensitivewords.Options{})

	added, err := matcher.LoadFromDir("/home/alex/git/Sensitive-lexicon/Vocabulary")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("loaded:", added)

	matcher.AddWord("自定义敏感词")
	matcher.AddWords([]string{"词A", "词B"})

	fmt.Println("count:", matcher.Count())
	fmt.Println("contains:", matcher.Contains("这段文本里有词A"))
}
```

### 方式三：从单个文件加载

```go
matcher := sensitivewords.NewEmptyMatcher(sensitivewords.Options{})

added, err := matcher.LoadFromFile("/path/to/政治类型.txt")
if err != nil {
    panic(err)
}

fmt.Println(added)
```

## API 说明

### 数据结构

#### `Entry`

表示一条原始词条：

```go
type Entry struct {
    Word     string
    Category string
    Source   string
}
```

字段说明：

- `Word`：敏感词内容
- `Category`：分类，例如 `政治`、`色情`、`广告`
- `Source`：来源文件或来源标识

#### `Options`

用于控制词条过滤和匹配策略：

```go
type Options struct {
    MinWordRunes        int
    MinCompactWordRunes int
    IncludeSingleRune   bool
}
```

字段说明：

- `MinWordRunes`：`strict` 模式下允许进入词库的最小词长
- `MinCompactWordRunes`：`compact` 模式下允许进入词库的最小词长
- `IncludeSingleRune`：是否允许单字词进入词库

### 构造函数

#### `NewEmptyMatcher`

创建一个空的匹配器，后续再手动加载或动态增词：

```go
matcher := sensitivewords.NewEmptyMatcher(sensitivewords.Options{})
```

适合以下场景：

- 程序先启动，再从本地文件加载词库
- 启动后由管理界面动态补充词库
- 需要多阶段初始化

#### `NewMatcher`

从内存中的 `[]Entry` 一次性构造匹配器：

```go
matcher, err := sensitivewords.NewMatcher(entries, sensitivewords.Options{})
```

当所有词条都被过滤后，会返回错误。

#### `NewMatcherFromDir`

从本地目录中的 `*.txt` 文件构造匹配器：

```go
matcher, err := sensitivewords.NewMatcherFromDir("/path/to/lexicon", sensitivewords.Options{})
```

### 词库加载

#### `LoadEntriesFromDir`

从本地目录读取 `*.txt` 文件，返回原始 `[]Entry`：

```go
entries, err := sensitivewords.LoadEntriesFromDir("/path/to/lexicon")
```

#### `LoadEntriesFromFile`

从本地单文件读取词条，返回原始 `[]Entry`：

```go
entries, err := sensitivewords.LoadEntriesFromFile("/path/to/file.txt")
```

#### `LoadFromDir`

把目录中的词条加载到现有匹配器中：

```go
added, err := matcher.LoadFromDir("/path/to/lexicon")
```

返回值说明：

- `added`：本次成功加入内存的词条数量
- `err`：文件读取失败时返回错误

#### `LoadFromFile`

把单个文件中的词条加载到现有匹配器中：

```go
added, err := matcher.LoadFromFile("/path/to/file.txt")
```

### 动态增加敏感词

#### `AddWord`

增加一个词：

```go
ok := matcher.AddWord("自定义敏感词")
```

返回 `false` 的情况：

- 输入为空
- 被当前过滤规则排除
- 该词已存在

#### `AddWords`

批量增加多个词：

```go
added := matcher.AddWords([]string{"词A", "词B", "词C"})
```

返回值是本次实际新增的数量。

#### `AddEntry`

增加一个带分类和来源信息的词条：

```go
ok := matcher.AddEntry(sensitivewords.Entry{
    Word:     "自定义词",
    Category: "自定义分类",
    Source:   "manual",
})
```

#### `AddEntries`

批量增加多个带元信息的词条：

```go
added := matcher.AddEntries(entries)
```

说明：

- 重复词条会被忽略
- 过滤后的无效词条会被忽略
- 每次成功新增后会重建自动机

### 检测接口

#### `Contains`

用于快速判断是否命中敏感词：

```go
if matcher.Contains(text) {
    // 触发拦截或进入下一步处理
}
```

说明：

- 返回值为 `true` 表示存在至少一个有效命中
- 该方法内部与 `Detect` 保持同一套判定语义

#### `Detect`

用于获取详细命中结果：

```go
result := matcher.Detect(text)
if result.Violates {
    for _, match := range result.Matches {
        fmt.Println(match.Keyword, match.Matched, match.Mode)
    }
}
```

### 统计接口

#### `Count`

返回当前内存中的唯一词数：

```go
count := matcher.Count()
```

说明：

- `Count()` 返回的是归一化去重后的唯一词数
- 大小写不同、全角半角不同但归一化后相同的词，会按一个词计算

#### `Stats`

返回更详细的统计信息：

```go
stats := matcher.Stats()
```

结构如下：

```go
type Stats struct {
    EntryCount          int
    WordCount           int
    StrictPatternCount  int
    CompactPatternCount int
}
```

字段说明：

- `EntryCount`：当前内存中保留的词条数，按 `Entry` 去重后统计
- `WordCount`：当前内存中的唯一词数
- `StrictPatternCount`：`strict` 自动机中的模式数
- `CompactPatternCount`：`compact` 自动机中的模式数

## 返回结果说明

`Detect` 返回的数据结构如下：

```go
type Result struct {
    Violates bool
    Matches  []Match
}
```

其中 `Match` 结构为：

```go
type Match struct {
    Keyword    string
    Matched    string
    Categories []string
    Sources    []string
    StartRune  int
    EndRune    int
    Mode       string
}
```

字段说明：

- `Keyword`：命中的归一化关键词
- `Matched`：原始输入文本中实际命中的片段
- `Categories`：该关键词关联到的所有分类
- `Sources`：该关键词关联到的所有来源
- `StartRune`：命中片段在原始文本中的起始 rune 偏移
- `EndRune`：命中片段在原始文本中的结束 rune 偏移，区间右边界不含
- `Mode`：命中模式，可能是 `strict` 或 `compact`

说明：

- 同一个词可能来自多个词库文件，因此 `Categories` 和 `Sources` 可能是多个值
- `Keyword` 是归一化后的匹配键，不一定与原始词条完全一致
- `Matched` 保留原始输入文本中的写法，便于审计或展示

## 默认行为

如果未显式传入阈值，当前默认行为如下：

- `MinWordRunes = 2`
- `MinCompactWordRunes = 3`
- `IncludeSingleRune = false`

这意味着：

- 单字词默认不会进入词库
- 过短词条在 `compact` 模式下会被更严格过滤
- 这样做是为了降低真实词库中的噪声和误判率

## 并发与更新策略

`Matcher` 支持并发使用。

当前实现策略如下：

- `Contains` 和 `Detect` 使用读锁，可并发执行
- `LoadFromDir`、`LoadFromFile`、`AddWord`、`AddWords`、`AddEntry`、`AddEntries` 使用写锁
- 写入成功后会重建 `strict` 和 `compact` 两套自动机

这意味着：

- 读多写少的场景很适合当前实现
- 动态增词不是增量更新，而是“追加词条后整体重建自动机”
- 如果你的系统需要非常高频的热更新，后续应考虑进一步做分层词库或增量结构优化

## 性能建议

对于当前实现，建议按以下方式使用：

- 程序启动时一次性加载主词库
- 运行时少量追加临时词或业务侧自定义词
- 对高频请求优先使用长期驻留的 `Matcher` 实例，避免重复构建

如果你的业务数据量较大，可以优先把它放在：

- HTTP 服务进程的全局单例
- 消费者进程的长生命周期对象
- 审核任务进程的批处理上下文

## 适用边界

这个包适合做第一层违规检测，但不应把“词库命中”直接等同于最终违规结论。

原因包括：

- 很多词存在明显语境依赖
- 同形词、缩写、新闻讨论和学术语境可能被误伤
- 纯词库方案对谐音、拼音、错别字、拆字、图片文本的识别能力有限

更稳妥的生产方案通常是：

1. 用本包做高召回初筛
2. 对 URL、广告、联系方式、规避写法增加规则层
3. 对边界样本增加人工审核或模型复核
