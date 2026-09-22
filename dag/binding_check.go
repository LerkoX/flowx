package dag

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// 上游绑定引用校验（exec 364 事故后的护栏）。
//
// 背景：节点 config.params / config.env / step.run 里的 {{ UpstreamNode.field }} 由
// pongo2 以非 strict 模式求值——上游字段缺失时渲染成空串且不报错；空串经执行器注入
// 节点 env 后，节点脚本把空串当作"缺参"，最终报错落在**下游**节点上
// （RuntimeError: missing required parameter），把排查方向带偏。
//
// exec 364 实测链路：docker exec 流被 cpolar 截断 → KSampler 节点绿灯但输出块丢失
// （metadata 无 KSampler.latent）→ VAEDecode 报"缺参"。
//
// 这里在节点执行前做一次静态校验：引用根是图内节点、而该字段在渲染上下文里不存在
// ⇒ 直接失败并指明上游。允许缺省的两类引用不参与校验：
//   - {{ Param.* }} 等保留根（Param 绑定回退语义本就允许缺省）
//   - 显式带 |default 过滤器的引用（作者主动声明"允许缺省"）
var (
	// reBindingRef 匹配模板中的引用对：Ident.field
	reBindingRef = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)`)

	// reDefaultFilter 引用后紧跟 |default 过滤器 ⇒ 用户显式声明允许缺省
	reDefaultFilter = regexp.MustCompile(`^\s*\|\s*default\b`)
)

// bindingCheckReservedRoots 非节点引用根：Param 有回退语义（缺省合法），
// Metadata 是聚合视图（键名不固定），两者都不做严格校验。
var bindingCheckReservedRoots = map[string]struct{}{
	"Param":    {},
	"Metadata": {},
}

// bindingExpr 待校验的表达式及其来源（用于报错定位）
type bindingExpr struct {
	where string // 如「参数 latent」「env SERVICE_URL」「步骤 run」
	expr  string
}

// validateNodeBindings 校验节点所有待渲染表达式的上游绑定引用，返回首个错误。
// 调用时机：节点执行前（上游已执行完毕，其输出应已进 metadata）。
func (p *WorkflowImpl) validateNodeBindings(node Node) error {
	exprs := collectNodeBindingExprs(node)
	if len(exprs) == 0 {
		return nil
	}
	nodeIDs := make(map[string]struct{})
	if p.graph != nil {
		for id := range p.graph.Nodes() {
			nodeIDs[id] = struct{}{}
		}
	}
	if len(nodeIDs) == 0 {
		return nil
	}

	ctx := p.buildRenderContext()
	for _, e := range exprs {
		if !strings.Contains(e.expr, "{{") {
			continue
		}
		if err := p.checkBindingRefs(node.Id(), e, nodeIDs, ctx); err != nil {
			return err
		}
	}
	return nil
}

// collectNodeBindingExprs 收集节点上所有会被模板引擎渲染的字符串。
// 顺序固定（params → env → steps，各自按键名/步骤名排序），保证报错稳定可复现。
func collectNodeBindingExprs(node Node) []bindingExpr {
	var out []bindingExpr

	params := nodeStringMap(node, "params")
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, bindingExpr{where: "参数 " + k, expr: params[k]})
	}

	env := nodeStringMap(node, "env")
	keys = keys[:0]
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, bindingExpr{where: "env " + k, expr: env[k]})
	}

	for _, step := range node.GetSteps() {
		out = append(out, bindingExpr{where: fmt.Sprintf("步骤 %q", step.Name), expr: step.Run})
	}
	return out
}

// checkBindingRefs 校验单个表达式中的 {{ Node.field }} 引用。
func (p *WorkflowImpl) checkBindingRefs(nodeID string, e bindingExpr, nodeIDs map[string]struct{}, ctx map[string]any) error {
	for _, m := range reBindingRef.FindAllStringSubmatchIndex(e.expr, -1) {
		root := e.expr[m[2]:m[3]]
		field := e.expr[m[4]:m[5]]

		if _, reserved := bindingCheckReservedRoots[root]; reserved {
			continue
		}
		if _, isNode := nodeIDs[root]; !isNode {
			continue // 不是节点引用（可能是扁平化的顶层 Param 名等）
		}
		if reDefaultFilter.MatchString(e.expr[m[1]:]) {
			continue // 显式声明允许缺省
		}

		obj, ok := ctx[root].(map[string]any)
		if !ok {
			return fmt.Errorf("节点 %s %s 的绑定 %s 无法解析：上游节点 %s 没有任何输出"+
				"（该节点执行成功但未提取到数据，常见于输出被截断或 extract 配置失效）；"+
				"若允许缺省请在绑定里写 %s.%s|default:\"\"",
				nodeID, e.where, e.expr, root, root, field)
		}
		if _, ok := obj[field]; !ok {
			return fmt.Errorf("节点 %s %s 的绑定 %s 无法解析：上游节点 %s 未输出字段 %s"+
				"（已输出: %s；常见于输出被截断或 extract 配置失效）；"+
				"若允许缺省请在绑定里写 %s.%s|default:\"\"",
				nodeID, e.where, e.expr, root, field, strings.Join(sortedKeys(obj), ", "), root, field)
		}
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return []string{"(无)"}
	}
	return keys
}
