package pipeline

import (
	"context"
	"net/url"

	"github.com/pkg/errors"
	"github.com/smartcontractkit/chainlink/core/logger"
	"github.com/smartcontractkit/chainlink/core/store/models"
)

type BridgeTask struct {
	BaseTask `mapstructure:",squash"`

	Name        string          `json:"name"`
	RequestData HttpRequestData `json:"requestData"`

	safeTx SafeTx
	config Config
}

var _ Task = (*BridgeTask)(nil)

func (t *BridgeTask) Type() TaskType {
	return TaskTypeBridge
}

func (t *BridgeTask) SetDefaults(inputValues map[string]string, g TaskDAG, self taskDAGNode) error {
	return nil
}

func (t *BridgeTask) Run(ctx context.Context, meta JSONSerializable, inputs []Result) (result Result) {
	if len(inputs) > 0 {
		return Result{Error: errors.Wrapf(ErrWrongInputCardinality, "BridgeTask requires 0 inputs")}
	}

	url, err := t.getBridgeURLFromName()
	if err != nil {
		return Result{Error: err}
	}

	var metaMap map[string]interface{}
	switch v := meta.Val.(type) {
	case map[string]interface{}:
		metaMap = v
	case nil:
	default:
		logger.Warnw(`"meta" field on task run is malformed, discarding`,
			"task", t.DotID(),
			"meta", meta,
		)
	}

	result = (&HTTPTask{
		URL:         models.WebURL(url),
		Method:      "POST",
		RequestData: withMeta(t.RequestData, metaMap),
		// URL is "safe" because it comes from the node's own database
		// Some node operators may run external adapters on their own hardware
		AllowUnrestrictedNetworkAccess: MaybeBoolTrue,
		config:                         t.config,
	}).Run(ctx, meta, inputs)
	if result.Error != nil {
		return result
	}
	logger.Debugw("Bridge task: fetched answer",
		"answer", result.Value,
		"url", url.String(),
	)
	return result
}

func (t BridgeTask) getBridgeURLFromName() (url.URL, error) {
	task := models.TaskType(t.Name)

	if t.safeTx.txMu != nil {
		t.safeTx.txMu.Lock()
		defer t.safeTx.txMu.Unlock()
	}

	bridge, err := FindBridge(t.safeTx.tx, task)
	if err != nil {
		return url.URL{}, err
	}
	bridgeURL := url.URL(bridge.URL)
	return bridgeURL, nil
}

func withMeta(request HttpRequestData, meta HttpRequestData) HttpRequestData {
	output := make(HttpRequestData)
	for k, v := range request {
		output[k] = v
	}
	output["meta"] = meta
	return output
}
