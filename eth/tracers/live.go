package tracers

import (
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/core/tracing"
)

// LiveDirectory is the collection of tracers which can be used
// during normal block import operations.
var LiveDirectory = liveDirectory{elems: make(map[string]tracing.LiveConstructor)}

type liveDirectory struct {
	elems   map[string]tracing.LiveConstructor
	elemsV2 map[string]tracing.LiveConstructorV2
}

// Register registers a tracer constructor by name.
func (d *liveDirectory) Register(name string, f tracing.LiveConstructor) {
	d.elems[name] = f
}

// RegisterV2 registers a tracer constructor by name.
func (d *liveDirectory) RegisterV2(name string, f tracing.LiveConstructorV2) {
	d.elemsV2[name] = f
}

// New instantiates a tracer by name.
func (d *liveDirectory) New(name string, config json.RawMessage) (*tracing.Hooks, error) {
	if f, ok := d.elems[name]; ok {
		return f(config)
	}
	return nil, errors.New("not found")
}

// NewV2 instantiates a tracer by name.
func (d *liveDirectory) NewV2(name string, config json.RawMessage) (*tracing.HooksV2, error) {
	if f, ok := d.elemsV2[name]; ok {
		return f(config)
	}
	return nil, errors.New("not found")
}
