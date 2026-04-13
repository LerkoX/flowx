# 单元测试文档

本文档记录项目中所有单元测试的信息，包括测试文件、测试函数及其功能说明。

## 测试统计

| 指标 | 数量 |
|------|------|
| 测试文件数 | 31 |
| 测试函数数 | 343 |
| 覆盖率目标 | 80%+ |

## 核心包测试

### edge_test.go

**包名**: `flowx`
**测试函数数**: 9
**功能概述**: Edge 边结构的创建与条件表达式求值

| 测试函数 | 说明 |
|---------|------|
| `TestNewDGAEdge` | 创建无条件边，验证 source、target、expression（空）、ID 格式 |
| `TestNewConditionalEdge` | 创建条件边，验证表达式正确设置 |
| `TestDGAEdge_Evaluate_Unconditional` | 无条件边始终求值为 true |
| `TestDGAEdge_Evaluate_ConditionalTrue` | 条件边在条件满足时求值为 true |
| `TestDGAEdge_Evaluate_ConditionalFalse` | 条件边在条件不满足时求值为 false |
| `TestDGAEdge_Evaluate_WithParams` | 条件边可使用求值上下文中的参数 |
| `TestDGAEdge_Evaluate_InvalidExpression` | 无效模板表达式返回错误 |
| `TestDGAEdge_ID` | 验证边 ID 格式为 `source->target` |
| `TestDGAEdge_ID_SpecialChars` | 边 ID 处理节点名中的特殊字符 |

### edge_extra_test.go

**包名**: `flowx`
**测试函数数**: 4
**功能概述**: 自定义模板引擎与边缘切换

| 测试函数 | 说明 |
|---------|------|
| `TestNewConditionalEdgeWithEngine` | 使用自定义模板引擎创建条件边 |
| `TestNewConditionalEdgeWithEngine_NilEngine` | nil 引擎回退到默认 Pongo2 |
| `TestDGAEdge_SetEngine` | 运行时替换模板引擎 |
| `TestDGAEdge_SetEngine_Overwrites` | 多次调用 SetEngine 只使用最后一次设置的引擎 |

### conditional_edge_test.go

**包名**: `flowx`
**测试函数数**: 5
**功能概述**: 条件边的遍历行为与路由逻辑

| 测试函数 | 说明 |
|---------|------|
| `TestConditionalEdge_Basic` | 条件不满足时遍历在条件边处停止 |
| `TestConditionalEdge_WithTrueCondition` | 条件满足时继续穿过条件边 |
| `TestConditionalEdge_WithParams` | 基于参数的条件路由：main→c，develop→d |
| `TestEdge_Evaluate` | 边求值：无条件始终 true，条件边根据节点状态求值 |
| `TestEdge_ID` | 边 ID 格式为 `source->target` |

### eval_context_test.go

**包名**: `flowx`
**测试函数数**: 16
**功能概述**: 求值上下文的创建、链式调用与不可变性

| 测试函数 | 说明 |
|---------|------|
| `TestNewEvaluationContext` | 验证新求值上下文非空 |
| `TestDGAEvaluationContext_Get` | 按 key 获取参数值 |
| `TestDGAEvaluationContext_Get_NotFound` | 获取不存在的 key 返回 `ok=false` |
| `TestDGAEvaluationContext_All_Empty` | 空上下文的 `All()` 仅返回 iteration 字段 |
| `TestDGAEvaluationContext_All_WithParams` | `All()` 包含参数和 iteration |
| `TestDGAEvaluationContext_All_WithNode` | `All()` 包含节点信息（nodeId、nodeStatus） |
| `TestDGAEvaluationContext_WithNode_Chaining` | 链式调用 `WithParams` 和 `WithNode` 保留两者 |
| `TestDGAEvaluationContext_WithNode_DoesNotModifyOriginal` | `WithNode` 不修改原始上下文（不可变性） |
| `TestDGAEvaluationContext_WithParams_DoesNotModifyOriginal` | `WithParams` 不修改原始上下文（不可变性） |
| `TestDGAEvaluationContext_WithParams_Overwrite` | 相同 key 的 `WithParams` 覆盖先前值 |
| `TestDGAEvaluationContext_All_MultipleTypes` | `All()` 处理混合类型（string、int、bool、float、nil） |
| `TestDGAEvaluationContext_Chaining_Multiple` | 多次 `WithParams` 和 `WithNode` 的复杂链式调用 |
| `TestDGAEvaluationContext_EmptyKey` | 空字符串 key 正常工作 |
| `TestDGAEvaluationContext_WithPipeline` | `WithPipeline` 添加 pipelineId 并保留现有参数 |
| `TestConvertBoolToString` | 布尔到字符串转换的表驱动测试（多种输入类型） |
| `TestDGAEvaluationContext_WithPipeline_DoesNotModifyOriginal` | `WithPipeline` 不修改原始上下文 |

### eval_context_extra_test.go

**包名**: `flowx`
**测试函数数**: 7
**功能概述**: 迭代计数与字符串查找工具

| 测试函数 | 说明 |
|---------|------|
| `TestDGAEvaluationContext_Iteration_Default` | 默认迭代计数为 0 |
| `TestDGAEvaluationContext_WithIteration` | `WithIteration` 设置迭代值 |
| `TestDGAEvaluationContext_WithIteration_DoesNotModifyOriginal` | `WithIteration` 不可变性验证 |
| `TestDGAEvaluationContext_Iteration_InAll` | `All()` 包含迭代值 |
| `TestDGAEvaluationContext_All_WithIterationAndPipeline` | `All()` 同时包含 iteration 和 pipelineId |
| `TestLastIndexOf` | `lastIndexOf` 字符串查找的表驱动测试 |
| `TestLastIndexOfByte` | `lastIndexOfByte` 单字节查找的表驱动测试 |

### pipeline_test.go

**包名**: `flowx`
**测试函数数**: 3
**功能概述**: DAG 遍历算法与基本流水线执行

| 测试函数 | 说明 |
|---------|------|
| `TestDGA_BFS` | 验证菱形图的 BFS 遍历按正确顺序访问节点 |
| `TestDGA_MultipleStartNodes` | 多个根节点（A、B）都被访问 |
| `TestPipeline_Run` | 菱形图流水线执行完成无错误 |

### cycle_test.go

**包名**: `flowx`
**测试函数数**: 15
**功能概述**: 循环检测与循环图遍历

| 测试函数 | 说明 |
|---------|------|
| `TestDGAGraph_ConditionalBackEdge` | 条件回边被接受，图被标记为 cyclic |
| `TestDGAGraph_UnconditionalCycle` | 无条件循环被拒绝并返回 `ErrHasCycle` |
| `TestDGAGraph_BuildForwardGraph` | 前向图不包含回边 |
| `TestDGAGraph_LoopNodeSet` | 通过 BFS 计算循环节点集合 |
| `TestDGAGraph_TraversalSteps_CyclicGraph` | 循环图 BFS 层级计算（4 层） |
| `TestDGAGraph_EntryExitNodes` | 入口/出口节点追踪 |
| `TestCyclicPipeline_Execution` | 从 `cyclic_loop.yaml` 执行循环流水线 |
| `TestCyclicPipeline_MaxIterations` | 最大迭代次数限制防止无限循环 |
| `TestAcyclicPipeline_NoRegression` | 无环流水线仍正常工作 |
| `TestDGAGraph_Traversal_CyclicGraph` | 遍历回调访问循环图中的所有节点 |
| `TestDGAGraph_Traversal_CyclicGraph_VisitOrder` | 循环图 BFS 层级访问顺序 |
| `TestDGAGraph_Traversal_AcyclicUnchanged` | 无环图遍历行为不变 |
| `TestDGAGraph_Traversal_CyclicGraph_ConditionalSkipped` | 非回边的条件边在遍历中被求值 |
| `TestDGAGraph_Traversal_CyclicGraph_ConditionalEdgeError` | 循环图中无效表达式返回错误 |
| `TestDGAGraph_TraversalSteps_WithEntryNodes` | `TraversalSteps` 使用入口节点 |

### graph_edge_test.go

**包名**: `flowx`
**测试函数数**: 11
**功能概述**: 图的边操作与条件边遍历

| 测试函数 | 说明 |
|---------|------|
| `TestDGAGraph_AddEdge_Success` | 在已有节点间添加边成功 |
| `TestDGAGraph_AddEdge_SourceNotFound` | 源节点不存在返回错误 |
| `TestDGAGraph_AddEdge_TargetNotFound` | 目标节点不存在返回错误 |
| `TestDGAGraph_AddEdge_Cycle` | 循环检测返回 `ErrHasCycle` |
| `TestDGAGraph_AddEdge_MultipleEdges` | 多条边和遍历 |
| `TestDGAGraph_AddEdge_SelfLoop` | 自环返回 `ErrHasCycle` |
| `TestDGAGraph_Traversal_WithConditionalEdge` | 条件为 false 时跳过目标节点 |
| `TestDGAGraph_Traversal_MultipleConditionalEdges` | env=prod 经由正确的条件边路由 |
| `TestDGAGraph_Traversal_ConditionalEdgeError` | 遍历中无效表达式返回错误 |
| `TestDGAGraph_Traversal_EmptyGraph` | 空图遍历不访问任何节点 |
| `TestDGAGraph_Traversal_SingleNode` | 单节点图只访问该节点 |

### graph_dynamic_test.go

**包名**: `flowx`
**测试函数数**: 16
**功能概述**: 图动态修改与 UpdateConfig

| 测试函数 | 说明 |
|---------|------|
| `TestDGAGraph_RemoveVertex` | 删除节点同时删除关联的边 |
| `TestDGAGraph_RemoveEdge` | 删除边保留节点 |
| `TestDGAGraph_GetNode_GetEdge` | 存在/不存在项的查询 |
| `TestDGAGraph_IncomingOutgoingEdges` | 入边和出边查询 |
| `TestDGAGraph_TraversalSteps` | BFS 层级计算：Level 0=A，Level 1=[B,C]，Level 2=D |
| `TestRuntimeImpl_PauseResume` | 运行中流水线的暂停/恢复生命周期 |
| `TestPipelineImpl_IsModifiable` | SUCCESS 和 CANCELLED 流水线的可修改性 |
| `TestGraph_DirectModification` | 完成后的直接修改：删除 C，添加 D/E 及边 |
| `TestGraph_RemoveAndReplace` | 替换未执行的节点 D 为 F 和 G |
| `TestGraph_CycleDetection` | 添加 B→A 后的循环检测 |
| `TestRuntimeImpl_UpdateConfig_AddNodes` | 通过 `UpdateConfig` 添加节点 C |
| `TestRuntimeImpl_UpdateConfig_RemoveUnexecutedNode` | 通过 `UpdateConfig` 删除未执行的节点 B |
| `TestRuntimeImpl_UpdateConfig_RemoveExecutedNode` | 删除已执行节点 A 被拒绝 |
| `TestRuntimeImpl_UpdateConfig_ModifyExecutedNode` | 修改已执行节点 A 被拒绝 |
| `TestRuntimeImpl_UpdateConfig_ImmutableField` | 更改 Name 字段被拒绝 |
| `TestRuntimeImpl_UpdateConfig_NoChanges` | 相同配置被接受 |

### graph_modify_test.go

**包名**: `flowx`
**测试函数数**: 9
**功能概述**: ModifyGraph 操作与循环预防

| 测试函数 | 说明 |
|---------|------|
| `TestModifyGraph_AddNode` | 通过 `ModifyGraph` 添加节点 |
| `TestModifyGraph_RemoveNode` | 通过 `ModifyGraph` 删除节点 |
| `TestModifyGraph_AddEdge` | 通过 `ModifyGraph` 添加边 |
| `TestModifyGraph_RemoveEdge` | 通过 `ModifyGraph` 删除边 |
| `TestModifyGraph_ConditionalBackEdge` | 接受条件回边 |
| `TestModifyGraph_UnconditionalCycle` | 拒绝无条件循环 |
| `TestModifyGraph_NotFound` | 修改不存在的流水线返回错误 |
| `TestModifyGraph_RollbackOnEdgeError` | 边添加失败时回滚图修改 |
| `TestModifyGraph_ComplexModification` | 组合操作：删除边 + 添加节点 + 添加边 |

### pipeline_extra_test.go

**包名**: `flowx`
**测试函数数**: 14
**功能概述**: 流水线生命周期、事件通知与输入请求

| 测试函数 | 说明 |
|---------|------|
| `TestPipelineImpl_Pause_NotRunning` | 暂停未运行的流水线返回错误 |
| `TestPipelineImpl_Pause_Running` | 暂停运行中的流水线关闭 `pauseChan` |
| `TestPipelineImpl_Resume_NotPaused` | 恢复未暂停的流水线返回错误 |
| `TestPipelineImpl_Resume_Paused` | 恢复已暂停的流水线关闭 `resumeChan` |
| `TestPipelineImpl_IsModifiable_Extra` | 表驱动验证 CANCELLED 和 SUCCESS 状态 |
| `TestPipelineImpl_Notify_WithListeningFn` | `Notify()` 调用监听函数 |
| `TestPipelineImpl_Notify_WithListener` | `Notify()` 调用 listener 的 `Handle` 并记录事件 |
| `TestPipelineImpl_Notify_WithBoth` | `Notify()` 同时调用函数和 listener |
| `TestPipelineImpl_HandleInputRequest_NilEvent` | nil 事件不 panic |
| `TestPipelineImpl_HandleInputRequest_NilRequest` | nil 请求不 panic |
| `TestPipelineImpl_HandleInputRequest_NilRuntimeStatus` | nil runtimeStatus 不 panic |
| `TestPipelineImpl_HandleInputRequest_Valid` | 有效输入请求设置节点为 PAUSED 并存储请求 |
| `TestGetStepStatusString` | 步骤状态查找的表驱动测试 |
| `TestPipelineImpl_MakeTraversalFn` | `makeTraversalFn` 创建有效遍历函数并执行节点 |

### runtime_test.go

**包名**: `flowx`
**测试函数数**: 41
**功能概述**: Runtime 核心功能、流水线执行、模板渲染、并发安全

| 测试函数 | 说明 |
|---------|------|
| `TestNewRuntime` | `NewRuntime` 返回非空 `*RuntimeImpl` |
| `TestRuntimeImpl_Get` | 获取不存在的流水线返回错误 |
| `TestRuntimeImpl_RunSync` | 同步执行流水线完成并清理 |
| `TestRuntimeImpl_RunSync_InvalidConfig` | 无效 YAML 配置返回错误 |
| `TestRuntimeImpl_RunSync_DuplicateID` | 重复流水线 ID 被拒绝 |
| `TestRuntimeImpl_RunAsync` | 异步执行并在 runtime 中存储 |
| `TestRuntimeImpl_Cancel` | 取消运行中的异步流水线 |
| `TestRuntimeImpl_Cancel_NonExistent` | 取消不存在的流水线返回错误 |
| `TestRuntimeImpl_Rm` | 删除流水线不 panic |
| `TestRuntimeImpl_Done` | `StopBackground()` 后 `Done()` 通道关闭 |
| `TestRuntimeImpl_Notify` | 通知 string、map、number 类型 |
| `TestRuntimeImpl_Ctx` | Runtime 上下文可取消 |
| `TestRuntimeImpl_StopBackground` | 停止后台处理取消上下文 |
| `TestRuntimeImpl_ConcurrentAccess` | 10 个并发异步流水线启动成功 |
| `TestRuntimeImpl_MultiPipelineConcurrency` | 5 个并发流水线，数据传递、元数据验证、事件计数 |
| `TestParseGraphEdges_BasicStateDiagram` | 基本状态图解析创建 3 个节点 |
| `TestParseGraphEdges_ComplexDiagram` | 复杂图（并行路径）创建 5 个节点 |
| `TestParseGraphEdges_EmptyGraph` | 空图仍从配置创建节点 |
| `TestParseGraphEdges_InvalidSyntax` | 无效语法仍创建配置的节点 |
| `TestParseGraphEdges_MissingNode` | 配置中缺失的节点只创建已有节点 |
| `TestExtractExpression` | 从标签提取条件表达式的表驱动测试 |
| `TestParseGraphEdges_ConditionalEdges` | 条件边表达式正确解析 |
| `TestParseGraphEdges_UnconditionalEdges` | 纯标签的边无表达式 |
| `TestParseGraphEdges_WithNotes` | 带注释和备注的图正确解析 |
| `TestRuntimeImpl_RenderParam_SelfReference` | Param 自引用正确渲染 |
| `TestRuntimeImpl_RenderParam_CircularReference` | Param 循环引用不崩溃 |
| `TestRuntimeImpl_RenderMetadata_ReferenceParam` | Metadata 可引用 Param 值 |
| `TestRuntimeImpl_RenderParam_NestedStructures` | 嵌套 Param 结构正确渲染 |
| `TestRuntimeImpl_RenderParam_WithUndefinedVariable` | 未定义变量保持模板字符串原样 |
| `TestRenderValue_NestedStructures` | `renderValue` 处理 map、slice、嵌套结构的表驱动测试 |
| `TestRuntimeImpl_NodeDataPassing` | 通过 extract 和 metadata 进行节点间数据传递 |
| `TestRuntimeImpl_ParallelNodes` | 并行节点执行比串行更快 |
| `TestRuntimeImpl_RuntimeRecovery` | 运行时状态恢复跳过已 SUCCESS 的节点 |
| `TestRuntimeImpl_ParallelStepRecovery` | 并行节点中的步骤级别恢复 |
| `TestRuntimeImpl_ConditionalEdge_SimpleParam` | 基于 Param 的简单条件边路由 |
| `TestRuntimeImpl_ConditionalEdge_Metadata` | 基于 Metadata 的条件边路由 |
| `TestRuntimeImpl_ConditionalEdge_Complex` | 复杂条件边组合 |
| `TestRuntimeImpl_ConditionalEdge_MultiCondition` | 多条件边和数值比较 |
| `TestComprehensivePipelineExecution` | 综合测试：同步/异步执行、Param 渲染、Metadata、多节点 DAG |
| `TestRuntimeImpl_ExportConfig` | 导出流水线配置为 YAML |
| `TestRuntimeImpl_ExportConfig_NotFound` | 导出不存在的流水线返回错误 |

### runtime_extra_test.go

**包名**: `flowx`
**测试函数数**: 19
**功能概述**: Runtime 扩展功能、不可变字段验证、清理

| 测试函数 | 说明 |
|---------|------|
| `TestRuntimeImpl_SetPusher` | `SetPusher` 存储 pusher |
| `TestRuntimeImpl_SetTemplateEngine_Custom` | 设置自定义模板引擎 |
| `TestRuntimeImpl_SetTemplateEngine_Nil` | 设置 nil 引擎返回默认 Pongo2 |
| `TestRuntimeImpl_Pause_NotFound` | 暂停不存在的流水线返回错误 |
| `TestRuntimeImpl_Resume_NotFound` | 恢复不存在的流水线返回错误 |
| `TestRuntimeImpl_CleanupCompletedPipelines` | 已完成的流水线被清理，运行中的不清理 |
| `TestSetPipelineParam` | `SetPipelineParam` 设置 param map |
| `TestSetPipelineParam_NonPipelineImpl` | nil 不 panic |
| `TestValidateImmutableFields_NoChanges` | 相同配置通过验证 |
| `TestValidateImmutableFields_VersionChanged` | Version 不可变 |
| `TestValidateImmutableFields_NameChanged` | Name 不可变 |
| `TestValidateImmutableFields_MaxLoopIterationsChanged` | MaxLoopIterations 不可变 |
| `TestValidateImmutableFields_ParamChanged` | Param 不可变 |
| `TestValidateImmutableFields_ExecutorsChanged` | Executors 配置不可变 |
| `TestValidateImmutableFields_LoggingChanged` | Logging 配置不可变 |
| `TestValidateImmutableFields_AIChanged` | AI 配置不可变 |
| `TestValidateImmutableFields_MetadateChanged` | Metadata 配置不可变 |
| `TestValidateImmutableFields_NodesMutable` | Nodes 允许变更 |
| `TestValidateImmutableFields_GraphMutable` | Graph 允许变更 |

### template_test.go

**包名**: `flowx`
**测试函数数**: 19
**功能概述**: Pongo2 模板引擎的布尔求值、字符串渲染与验证

| 测试函数 | 说明 |
|---------|------|
| `TestNewPongo2TemplateEngine` | 引擎创建返回非空 |
| `TestPongo2TemplateEngine_EvaluateBool_True` | 布尔 true 求值 |
| `TestPongo2TemplateEngine_EvaluateBool_False` | 布尔 false 求值 |
| `TestPongo2TemplateEngine_EvaluateBool_EmptyResult` | 空模板输出返回 false |
| `TestPongo2TemplateEngine_EvaluateBool_StringTrue` | 字符串 "true" 返回 true |
| `TestPongo2TemplateEngine_EvaluateBool_StringFalse` | 字符串 "false" 为 truthy（非空），"false" == false |
| `TestPongo2TemplateEngine_EvaluateBool_StringOne` | 字符串 "1" 为 truthy |
| `TestPongo2TemplateEngine_EvaluateBool_StringZero` | 字符串 "0" 为 truthy，"0" == "0" 为 true |
| `TestPongo2TemplateEngine_EvaluateBool_NonEmptyString` | 非空字符串为 truthy |
| `TestPongo2TemplateEngine_EvaluateBool_InvalidSyntax` | 无效模板语法返回错误 |
| `TestPongo2TemplateEngine_EvaluateBool_ComplexCondition` | `and` 逻辑的双条件 |
| `TestPongo2TemplateEngine_EvaluateString` | 字符串模板求值 |
| `TestPongo2TemplateEngine_EvaluateString_WithSpaces` | 字符串输出被 trim |
| `TestPongo2TemplateEngine_EvaluateString_InvalidSyntax` | 无效语法返回错误 |
| `TestPongo2TemplateEngine_Validate_Valid` | 有效表达式通过验证 |
| `TestPongo2TemplateEngine_Validate_Invalid` | 无效表达式验证失败 |
| `TestPongo2TemplateEngine_Validate_Empty` | 空字符串通过验证 |
| `TestPongo2TemplateEngine_EvaluateBool_WithPipelineData` | 使用 pipeline 级数据求值 |
| `TestPongo2TemplateEngine_EvaluateBool_WithNodeData` | 使用节点级数据求值 |

### pipeline_metadata_test.go

**包名**: `flowx`
**测试函数数**: 3
**功能概述**: 元数据存储的线程安全与输出提取

| 测试函数 | 说明 |
|---------|------|
| `TestInConfigMetadataStore_ThreadSafety` | 100 个并发 goroutine 写入 InConfigMetadataStore |
| `TestInConfigMetadataStore_Delete` | Delete 移除 key 且不影响其他 key |
| `TestExtractOutput_Basic` | 从输出中解析 `flowx-yaml` codec block 并存储到流水线元数据 |

### pipeline_extract_test.go

**包名**: `flowx`
**测试函数数**: 3
**功能概述**: 输出提取器与 codec block 解析

| 测试函数 | 说明 |
|---------|------|
| `TestPipeline_OutputExtraction` | 从输出中提取 codec block（buildId、version、status） |
| `TestPipeline_OutputExtraction_Regex` | 从输出中用正则提取（coverage、testsPassed） |
| `TestCreateExtractor_InvalidConfig` | nil 配置、无效 type、不支持 type、无 pattern 的正则、无效正则的表驱动测试 |

### extractor_test.go

**包名**: `flowx`
**测试函数数**: 7
**功能概述**: Codec block 提取器与正则提取器

| 测试函数 | 说明 |
|---------|------|
| `TestCodecBlockExtractor_ExtractYAML` | 从 `flowx-yaml` 代码块提取 YAML |
| `TestCodecBlockExtractor_ExtractWithComments` | 从 `flowx-yaml` 代码块提取带注释的字段 |
| `TestRegexExtractor` | 正则模式提取（coverage、testsPassed、buildStatus） |
| `TestRegexExtractor_NoMatches` | 无匹配返回空 map |
| `TestRegexExtractor_InvalidPattern` | 无效正则创建时返回错误 |
| `TestRegexExtractor_WithoutCaptureGroup` | 无捕获组使用完整匹配 |
| `TestRegexExtractor_MultipleCapturingGroups` | 多捕获组使用第一个组 |

### snapshot_test.go

**包名**: `flowx`
**测试函数数**: 9
**功能概述**: 流水线快照的序列化、反序列化与深拷贝

| 测试函数 | 说明 |
|---------|------|
| `TestPipelineSnapshotter_ToYAML_BasicConfig` | 基本配置的 YAML 序列化 |
| `TestPipelineSnapshotter_ToYAML_EmptyConfig` | 空配置序列化为非空 YAML |
| `TestPipelineSnapshotter_FromYAML_BasicConfig` | YAML 反序列化产生正确配置 |
| `TestPipelineSnapshotter_FromYAML_RoundTrip` | ToYAML → FromYAML 往返保留 Name |
| `TestPipelineSnapshotter_FromYAML_InvalidYAML` | 无效 YAML 返回错误 |
| `TestPipelineSnapshotter_TakeSnapshot_SimplePipeline` | 快照包含运行时状态 |
| `TestPipelineSnapshotter_TakeSnapshot_NilRuntimeStatus` | 未启动节点的快照 runtimeStatus 为 nil |
| `TestPipelineSnapshotter_TakeSnapshot_DeepCopy` | 快照为深拷贝（修改原对象不影响快照） |
| `TestPipelineSnapshotter_TakeSnapshot_WithSteps` | 快照保留步骤 ID |

### boolean_conversion_test.go

**包名**: `flowx`
**测试函数数**: 7
**功能概述**: 布尔值转换与复杂 CI/CD 条件表达式

| 测试函数 | 说明 |
|---------|------|
| `TestConvertBoolToString_Simple` | 简单 bool、string、int、float、nil 转换 |
| `TestConvertBoolToString_NestedMap` | 嵌套 map 布尔转换 |
| `TestConvertBoolToString_MixedInterfaceMap` | `map[interface{}]interface{}` 布尔转换 |
| `TestDGAEvaluationContext_All_BooleanConversion` | `All()` 将布尔值转换为字符串 |
| `TestDGAEvaluationContext_All_NestedBooleanConversion` | `All()` 深度嵌套布尔转换 |
| `TestPongo2TemplateEngine_EvaluateBool_StringBooleanComparison` | Pongo2 中字符串到布尔比较的表驱动测试 |
| `TestPongo2TemplateEngine_EvaluateBool_ComplexCICDCondition` | 嵌套对象和布尔比较的 CI/CD 复杂条件表驱动测试 |

### uuid_test.go

**包名**: `flowx`
**测试函数数**: 3
**功能概述**: UUID 验证、格式化与生成

| 测试函数 | 说明 |
|---------|------|
| `TestValidateUUID` | UUID 验证的表驱动测试（带/不带连字符、全零、无效、空、过长、无效十六进制、大小写、错误分组） |
| `TestFormatUUID` | 32 字符字符串格式化为带连字符 UUID，已格式化保持不变 |
| `TestNewUUID` | `NewUUID` 返回 32 字符、无连字符、有效、唯一的 UUID |

## Logger 包测试

### logger/logger_test.go

**包名**: `logger`
**测试函数数**: 2
**功能概述**: 日志级别常量与 Entry 结构

| 测试函数 | 说明 |
|---------|------|
| `TestLevel_Constants` | 验证日志级别字符串常量：debug、info、warn、error |
| `TestEntry_Fields` | 验证 Entry 结构字段赋值（Pipeline、BuildID、Node、Step、Timestamp、Level、Message、Output） |

### logger/console_pusher_test.go

**包名**: `logger`
**测试函数数**: 13
**功能概述**: 控制台日志推送器的格式化输出

| 测试函数 | 说明 |
|---------|------|
| `TestNewConsolePusher` | 默认 showTime 和 showNode 为 true |
| `TestConsolePusher_Push_WithMessage` | 带消息条目的 Push 成功 |
| `TestConsolePusher_Push_WithOutput` | 带输出内容的 Push 成功 |
| `TestConsolePusher_Push_ZeroTimestamp` | 零时间戳自动生成 |
| `TestConsolePusher_Push_UnknownLevel` | 未知级别成功处理 |
| `TestConsolePusher_Push_EmptyEntry` | 空条目成功处理 |
| `TestConsolePusher_PushBatch` | 批量推送多条日志 |
| `TestConsolePusher_PushBatch_Empty` | 空切片批量推送 |
| `TestConsolePusher_Close` | Close 成功 |
| `TestConsolePusher_SetShowTime` | SetShowTime 切换 |
| `TestConsolePusher_SetShowNode` | SetShowNode 切换 |
| `TestConsolePusher_Push_AllLevels` | 所有四个级别（debug、info、warn、error） |
| `TestConsolePusher_Push_NoMessageNoOutput` | 无 message 和 output 的 Push |

## Executor 包测试

### executor/provider/provider_test.go

**包名**: `provider`
**测试函数数**: 7
**功能概述**: 执行器提供者的注册与获取

| 测试函数 | 说明 |
|---------|------|
| `TestNewProvider` | Provider 创建和 map 初始化 |
| `TestProvider_GetExecutor_NotFound` | 未注册的执行器返回错误 |
| `TestProvider_GetExecutor_UnsupportedType` | 不支持的类型返回错误 |
| `TestProvider_GetExecutor_LocalType` | local 类型执行器创建和清理 |
| `TestProvider_GetExecutor_LocalWithInvalidTimeout_IgnoresGracefully` | 无效 timeout 配置被忽略 |
| `TestProvider_RegisterExecutor_MultipleExecutors` | 注册和获取多个执行器 |
| `TestProvider_RegisterExecutor_Overwrite` | 覆盖执行器注册 |

### executor/docker/docker_unit_test.go

**包名**: `docker`
**测试函数数**: 11
**功能概述**: Docker 执行器单元测试（无需 Docker 运行时）

| 测试函数 | 说明 |
|---------|------|
| `TestNewDockerExecutorWithClient_NilClient` | nil client 构造函数正常工作 |
| `TestNewDockerExecutorWithClient_GetType` | GetType 返回 "docker" |
| `TestNewDockerExecutorWithClient_GetInstanceId` | Prepare 前 GetInstanceId 为空 |
| `TestDockerExecutor_DetectShell` | 基于镜像名检测 shell（alpine→sh、ubuntu→bash 等）的表驱动测试 |
| `TestDockerExecutor_ResolveImageName` | 带/不带 registry 的镜像名解析的表驱动测试 |
| `TestDockerExecutor_BuildEnvList` | 环境变量列表构建（空、3个变量、KEY=VALUE 格式） |
| `TestDockerExecutor_BuildMounts` | 挂载点构建（空、2个卷） |
| `TestDockerExecutor_CopyToContainer_NotPrepared` | 未 Prepare 时复制返回错误 |
| `TestDockerExecutor_CopyFromContainer_NotPrepared` | 未 Prepare 时复制返回错误 |
| `TestDockerExecutor_GetRuntimeInfo_BeforePrepare` | Prepare 前运行时信息（无 containerId，有 image/workdir/network/registry） |
| `TestDockerExecutor_InterfaceCompliance` | 接口合规性检查 |

### executor/docker/docker_local_test.go

**包名**: `docker`
**测试函数数**: 3
**功能概述**: Docker 配置解析与本地环境检测

| 测试函数 | 说明 |
|---------|------|
| `TestDockerAdapter_Config` | DockerAdapter 配置：registry、network、workdir、volumes、env、TTY |
| `TestDockerBridge_Conn` | DockerBridge 创建 DockerExecutor |
| `TestDockerExecutor_Setters` | 所有 setter 方法（image、workdir、env、volume、network、registry、TTY） |
| `TestParseVolume` | 卷解析（有效、带模式、无冒号、空 host）的表驱动测试 |
| `TestDockerExecutor_Interface` | DockerExecutor 实现 Executor 接口 |
| `TestDockerAdapter_Interface` | DockerAdapter 实现 Adapter 接口 |
| `TestDockerBridge_Interface` | DockerBridge 实现 Bridge 接口 |
| `TestDockerExecutor_PrepareAndDestruction` | 容器生命周期（Docker 不可用时跳过） |
| `TestDockerExecutor_ConfigApplication` | 配置应用场景（基本、volumes、env、tty、混合）的表驱动测试 |
| `TestDockerExecutor_AdapterConfigWithInvalidTypes` | 无效配置类型被优雅接受的表驱动测试 |
| `TestDockerExecutor_VolumeParsing` | 适配器配置卷解析（字符串列表、任意列表、nil、空）的表驱动测试 |
| `TestDockerExecutor_EnvConfigParsing` | 环境配置解析（map[string]string、map[string]any、nil、空）的表驱动测试 |

### executor/docker/docker_test.go

**包名**: `docker`
**测试函数数**: 13
**功能概述**: Docker 执行器集成测试（需要 Docker 运行时）

| 测试函数 | 说明 |
|---------|------|
| `TestDockerExecutor_WithDefaultRegistry` | 默认 registry 镜像拉取、命令执行、环境检查 |
| `TestDockerExecutor_MultiCommands` | 一个容器中 3 个顺序命令 |
| `TestDockerExecutor_CustomRegistry` | 自定义/空 registry 配置 |
| 所有测试在 Docker 不可用时自动跳过 |

### executor/docker/docker_extra_test.go

**包名**: `docker`
**测试函数数**: 25
**功能概述**: Docker 执行器扩展集成测试

| 测试函数 | 说明 |
|---------|------|
| `TestDockerExecutor_DestructionTerminatesProcess` | Destruction 移除容器 |
| `TestDockerExecutor_ContextCancelTerminatesProcess` | 上下文取消终止进程 |
| `TestDockerExecutor_Transfer_CommandFailure` | 非零退出码返回带退出码的错误 |
| `TestDockerExecutor_InvalidImage` | 无效镜像在 Prepare 阶段失败 |
| `TestDockerExecutor_InvalidWorkdir` | 不存在 workdir 的行为 |
| `TestDockerExecutor_TransferWithoutPrepare` | 未 Prepare 时 Transfer 返回错误 |
| `TestDockerExecutor_UnsupportedDataType` | 发送 int 到 commandChan 返回错误 |
| `TestDockerExecutor_VolumeMounting` | 绑定挂载工作 |
| `TestDockerExecutor_EnvVarsInContainer` | 环境变量在容器内设置 |
| `TestDockerExecutor_NetworkMode` | host 网络模式应用 |
| `TestDockerExecutor_DoublePrepare` | 双重 Prepare 行为 |
| `TestDockerExecutor_MultipleCommandsSequential` | 3 个顺序命令 |
| `TestDockerExecutor_LargeOutput` | 生成 1000 行输出 |
| `TestDockerExecutor_ShellDetectionAlpine` | Alpine 使用 sh |
| `TestDockerExecutor_ShellDetectionUbuntu` | Ubuntu 使用 bash |
| `TestDockerExecutor_GetRuntimeInfo` | Prepare 后运行时信息 |
| `TestDockerExecutor_GetType` | GetType 返回 "docker" |
| `TestDockerExecutor_InvalidAdapter` | 错误的适配器类型返回错误 |
| `TestDockerExecutor_BridgeConfigError` | config 中无效卷格式 |
| `TestDockerExecutor_WorkdirInContainer` | workdir 在容器中设置 |
| `TestDockerExecutor_CommandWithPipe` | 管道命令工作 |
| `TestDockerExecutor_SpecialCharacters` | 引号、分号、管道等特殊字符 |
| `TestDockerExecutor_ContainerIsolation` | 两个容器互相隔离 |
| `TestDockerExecutor_RegistryConfiguration` | registry getter/setter 和默认值 |
| `TestDockerExecutor_ConcurrentCommands` | 一次 Transfer 中 5 个并发命令 |

### executor/local/local_test.go

**包名**: `local`
**测试函数数**: 20
**功能概述**: Local 执行器核心功能

| 测试函数 | 说明 |
|---------|------|
| `TestLocalAdapter_Config` | LocalAdapter 配置存储 |
| `TestLocalBridge_Conn` | LocalBridge 创建正确配置的 LocalExecutor |
| `TestLocalExecutor_Interface` | LocalExecutor 实现 Executor 接口 |
| `TestLocalAdapter_Interface` | LocalAdapter 实现 Adapter 接口 |
| `TestLocalBridge_Interface` | LocalBridge 实现 Bridge 接口 |
| `TestNewLocalExecutor` | 执行器创建和默认 shell 检测 |
| `TestLocalExecutor_Prepare` | 子测试：默认、有效 workdir、无效 workdir |
| `TestLocalExecutor_Destruction` | 无运行命令时 Destruction 成功 |
| `TestLocalExecutor_Transfer_SingleCommand` | 单命令执行和输出 |
| `TestLocalExecutor_Transfer_CommandFailure` | 非零退出码返回错误 |
| `TestLocalExecutor_EnvConfiguration` | 环境变量在命令中设置 |
| `TestLocalExecutor_ConfigApplication` | 配置场景（基本、timeout string/number、pty、env、混合）的表驱动测试 |
| `TestLocalExecutor_InvalidAdapter` | 错误的适配器类型返回错误 |
| `TestParseTimeout` | timeout 解析（string、int、int64、float64、无效、不支持）的表驱动测试 |
| `TestDetectDefaultShell` | 按 OS 检测默认 shell |
| `TestLocalExecutor_CancelContext` | 长时间运行命令的上下文取消 |
| `TestLocalExecutor_Getters` | workdir、shell 的 getter 方法 |
| `TestLocalExecutor_UnsupportedDataType` | 发送 int 到 commandChan 返回错误 |
| `TestLocalExecutor_DestructionTerminatesProcess` | Destruction 终止运行中的进程（文件增长测试） |
| `TestLocalExecutor_ContextCancelTerminatesProcess` | 上下文取消终止运行中的进程（文件增长测试） |

### executor/local/local_coverage_test.go

**包名**: `local`
**测试函数数**: 8
**功能概述**: Local 执行器覆盖率增强测试

| 测试函数 | 说明 |
|---------|------|
| `TestParseInputRequest_YAML` | YAML 输入请求解析 |
| `TestParseInputRequest_JSON` | JSON 输入请求解析 |
| `TestParseInputRequest_Empty` | 空内容返回 nil |
| `TestParseInputRequest_InvalidFormat` | 不可解析内容返回 nil |
| `TestParseInputRequest_MissingType` | 缺少 type 字段返回 nil |
| `TestIsShellAvailable` | "sh" 可用，不存在的 shell 不可用 |
| `TestLocalExecutor_GetInstanceId` | Prepare 前 GetInstanceId 为空 |
| `TestLocalExecutor_InterfaceCompliance` | Executor 和 ExecutorInfoProvider 接口合规性 |

### executor/local/interactive_test.go

**包名**: `local`
**测试函数数**: 3
**功能概述**: Local 执行器交互式输入

| 测试函数 | 说明 |
|---------|------|
| `TestLocalExecutor_InteractiveInput_NilChannel` | nil 输入通道的命令执行 |
| `TestLocalExecutor_InteractiveInput_WithChannel` | 通过通道的交互式输入（cat 命令） |
| `TestLocalExecutor_InteractiveInput_ContextCancel` | 上下文取消终止运行中的命令 |

### executor/local/local_extra_test.go

**包名**: `local`
**测试函数数**: 18
**功能概述**: Local 执行器扩展功能测试

| 测试函数 | 说明 |
|---------|------|
| `TestLocalExecutor_MultipleCommandsSequential` | 3 个顺序命令 |
| `TestLocalExecutor_ConcurrentCommands` | 一次 Transfer 中 5 个并发命令 |
| `TestLocalExecutor_LargeOutput` | `seq 1 1000` 输出处理 |
| `TestLocalExecutor_CommandWithPipe` | 管道命令执行 |
| `TestLocalExecutor_SpecialCharacters` | 命令中的引号、分号、管道等特殊字符 |
| `TestLocalExecutor_InvalidAdapterExtra` | 无效适配器类型错误 |
| `TestLocalExecutor_BridgeConfigError` | bridge 中无效 timeout 配置 |
| `TestLocalExecutor_ShellConfiguration` | bash shell 执行 |
| `TestLocalExecutor_Timeout` | 1 秒超时终止 `sleep 5` |
| `TestLocalExecutor_GetType` | GetType 返回 "local" |
| `TestLocalExecutor_GetRuntimeInfo` | 运行时信息包含 workdir 和 shell |
| `TestLocalExecutor_WorkdirValidation` | 子测试：有效、不存在、以文件为 workdir |
| `TestLocalExecutor_WorkdirInCommand` | 命令在 `/tmp` workdir 执行 |
| `TestLocalExecutor_EnvOverride` | 自定义环境变量可访问 |
| `TestLocalExecutor_EmptyCommand` | `true` 命令成功 |
| `TestLocalExecutor_CommandNotFound` | 不存在的命令返回错误 |
| `TestLocalExecutor_PTYConfiguration` | PTY 默认值和 setter 方法 |
| `TestLocalExecutor_DefaultShellDetectionExtra` | 按 OS 的默认 shell 检测 |

## 测试配置文件

测试配置文件位于 `test/fixtures/runtime/` 目录，详细说明见 [test/fixtures/runtime/README.md](test/fixtures/runtime/README.md)。

### 配置文件总览

| 配置文件 | 对应测试 | 功能 |
|---------|---------|------|
| `sync_pipeline.yaml` | `TestRuntimeImpl_RunSync` | 同步执行流水线，执行完成后自动清理 |
| `async_pipeline.yaml` | `TestRuntimeImpl_RunAsync` | 异步执行流水线，流水线保持在 Runtime 中 |
| `invalid_config.yaml` | `TestRuntimeImpl_RunSync_InvalidConfig` | 无效 YAML 配置的错误处理 |
| `single_node.yaml` | `TestRuntimeImpl_RunSync_DuplicateID` | 重复 ID 检测 |
| `long_running.yaml` | `TestRuntimeImpl_Cancel` | 流水线取消功能 |
| `concurrent_template.yaml` | `TestRuntimeImpl_ConcurrentAccess` | 并发访问安全 |
| `param_self_reference.yaml` | `TestRuntimeImpl_RenderParam_SelfReference` | Param 自引用 |
| `param_circular_reference.yaml` | `TestRuntimeImpl_RenderParam_CircularReference` | Param 循环引用检测 |
| `metadata_ref_param.yaml` | `TestRuntimeImpl_RenderMetadata_ReferenceParam` | Metadata 引用 Param |
| `param_nested.yaml` | `TestRuntimeImpl_RenderParam_NestedStructures` | Param 嵌套结构模板渲染 |
| `param_undefined.yaml` | `TestRuntimeImpl_RenderParam_WithUndefinedVariable` | 未定义变量保持原样 |
| `node_data_passing.yaml` | `TestRuntimeImpl_NodeDataPassing` | 节点间数据传递 |
| `parallel_nodes.yaml` | `TestRuntimeImpl_ParallelNodes` | 并行节点执行 |
| `runtime_recovery.yaml` | `TestRuntimeImpl_RuntimeRecovery` | 运行时状态恢复 |
| `parallel_with_step_recovery.yaml` | `TestRuntimeImpl_ParallelStepRecovery` | 并行节点步骤级别恢复 |
| `comprehensive_sync.yaml` | 综合同步执行测试 | 同步流水线执行和事件监听 |
| `comprehensive_async.yaml` | 综合异步执行测试 | 异步流水线执行和存储 |
| `comprehensive_param_render.yaml` | 综合 Param 渲染测试 | Param 模板渲染功能 |
| `comprehensive_metadata.yaml` | 综合 Metadata 测试 | Metadata 创建、参数引用和渲染 |
| `comprehensive_dag.yaml` | 综合 DAG 测试 | 多节点 DAG 执行和验证事件触发 |
| `conditional_edge_simple.yaml` | `TestRuntimeImpl_ConditionalEdge_SimpleParam` | 基于 Param 的简单条件边 |
| `conditional_edge_metadata.yaml` | `TestRuntimeImpl_ConditionalEdge_Metadata` | 基于 Metadata 条件的边 |
| `conditional_edge_complex.yaml` | `TestRuntimeImpl_ConditionalEdge_Complex` | 复杂条件边组合 |
| `conditional_edge_multi_cond.yaml` | `TestRuntimeImpl_ConditionalEdge_MultiCondition` | 多条件边逻辑 |
| `cyclic_loop.yaml` | `TestCyclicPipeline_Execution` | 循环流水线执行 |
| `dynamic_modify.yaml` | 图动态修改测试 | UpdateConfig 动态修改 |

## 运行测试

```bash
# 运行所有测试
go test ./...

# 运行测试并显示详细输出
go test ./... -v

# 运行特定包的测试
go test ./... -run TestRuntimeImpl

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```
