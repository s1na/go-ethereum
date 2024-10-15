package tracers

import (
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/core/tracing"
)

// LiveDirectory is the collection of tracers which can be used
// during normal block import operations.
var LiveDirectory = liveDirectory{elems: make(map[string]tracing.LiveConstructorV2)}

type liveDirectory struct {
	elems map[string]tracing.LiveConstructorV2
}

// Register registers a tracer constructor by name.
func (d *liveDirectory) Register(name string, f tracing.LiveConstructor) {
	d.elems[name] = wrapV1(f)
}

// RegisterV2 registers a tracer constructor by name.
func (d *liveDirectory) RegisterV2(name string, f tracing.LiveConstructorV2) {
	d.elems[name] = f
}

// NewV2 instantiates a tracer by name.
func (d *liveDirectory) NewV2(name string, config json.RawMessage) (*tracing.HooksV2, error) {
	if f, ok := d.elems[name]; ok {
		return f(config)
	}
	return nil, errors.New("not found")
}

func wrapV1(ctor tracing.LiveConstructor) tracing.LiveConstructorV2 {
	return func(config json.RawMessage) (*tracing.HooksV2, error) {
		hooks, err := ctor(config)
		if err != nil {
			return nil, err
		}
		v2 := hooks.ToV2()
		v2.OnSystemCallStart = func(ctx *tracing.VMContext) {
			hooks.OnSystemCallStart()
		}
		return v2, nil
	}
}
