package tools

import (
	"encoding/json"
	"fmt"
)

// SendInput writes data to a waiting process stdin.
func SendInput(id, data string, pm *ProcManager) ([]byte, error) {
	p := pm.Get(id)
	if p == nil || p.SendInput == nil {
		return nil, fmt.Errorf("unknown process_id or session expired")
	}
	if err := p.SendInput([]byte(data)); err != nil {
		return nil, err
	}
	out := pm.SnapshotOutput(id)
	res := map[string]any{
		"status": "ok",
		"stdout": out,
	}
	return json.Marshal(res)
}

// CancelCommand kills a tracked process.
func CancelCommand(id string, pm *ProcManager) ([]byte, error) {
	ok := pm.Cancel(id)
	res := map[string]any{
		"status": map[bool]string{true: "cancelled", false: "not_found"}[ok],
	}
	return json.Marshal(res)
}
