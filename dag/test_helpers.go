package dag

import "github.com/LerkoX/flowx/core"

// SetStatusForTest sets the pipeline status for testing purposes.
// This is intentionally exported to allow integration tests in other packages
// to set up specific pipeline states without modifying the production API.
func (p *PipelineImpl) SetStatusForTest(status string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = status
}

// DoneChanForTest returns the doneChan for testing purposes.
func (p *PipelineImpl) DoneChanForTest() chan struct{} {
	return p.doneChan
}

// SetParamForTest sets the pipeline param for testing purposes.
func (p *PipelineImpl) SetParamForTest(param map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.param = make(map[string]core.FieldItem)
	for k, v := range param {
		p.param[k] = core.ConvertToFieldItem(v)
	}
}

// ParamForTest returns the pipeline param for testing purposes.
func (p *PipelineImpl) ParamForTest() map[string]core.FieldItem {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.param
}
