package pipelinex

// GraphModifications 表示一组要原子应用的图修改操作
type GraphModifications struct {
	// AddNodes 要添加的节点配置列表
	AddNodes []NodeConfig `yaml:"addNodes"`
	// RemoveNodes 要删除的节点 ID 列表（同时删除关联边）
	RemoveNodes []string `yaml:"removeNodes"`
	// AddEdges 要添加的边列表
	AddEdges []EdgeModification `yaml:"addEdges"`
	// RemoveEdges 要删除的边列表
	RemoveEdges []EdgeRemoval `yaml:"removeEdges"`
	// AddGraph 可选的 Mermaid stateDiagram-v2 片段，用于批量添加边
	AddGraph string `yaml:"addGraph"`
}

// EdgeModification 表示一条要添加的边
type EdgeModification struct {
	Source     string `yaml:"source"`
	Target     string `yaml:"target"`
	Expression string `yaml:"expression,omitempty"`
}

// EdgeRemoval 表示一条要删除的边
type EdgeRemoval struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

// IsEmpty 判断修改集是否为空
func (m GraphModifications) IsEmpty() bool {
	return len(m.AddNodes) == 0 && len(m.RemoveNodes) == 0 &&
		len(m.AddEdges) == 0 && len(m.RemoveEdges) == 0 && m.AddGraph == ""
}
