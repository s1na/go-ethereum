type Contract struct {
	Caller common.Address
    Address common.Address
    Value *big.Int
    Input []byte
}

type StackWrapper struct {
	stack *vm.Stack
}

func (s *StackWrapper) Peek(idx int) *big.Int {
    if len(sw.stack.Data()) <= idx || idx < 0 {
        log.Warn("Tracer accessed out of bound stack", "size", len(sw.stack.Data()), "index", idx)
        return new(big.Int)
    }
    return sw.stack.Back(idx).ToBig()
}

type MemoryWrapper struct {
	memory *vm.Memory
}

func (mw *MemoryWrapper) Slice(begin, end int64) []byte {
    if end == begin {
        return []byte{}
    }
    if end < begin || begin < 0 {
        log.Warn("Tracer accessed out of bound memory", "offset", begin, "end", end)
        return nil
    }
    if mw.memory.Len() < int(end) {
        log.Warn("Tracer accessed out of bound memory", "available", mw.memory.Len(), "offset", begin, "size", end-begin)
        return nil
    }
    return mw.memory.GetCopy(begin, end-begin)
}

func (mw *MemoryWrapper) GetUint(addr uint64) *big.Int {
    if mw.memory.Len() < int(addr)+32 || addr < 0 {
        log.Warn("Tracer accessed out of bound memory", "available", mw.memory.Len(), "offset", addr, "size", 32)
        return new(big.Int)
    }
    return new(big.Int).SetBytes(mw.memory.GetPtr(addr, 32))
}

type StepLog struct {
	Pc       uint
	Gas      uint
	Cost     uint
	Depth    uint
	Refund   uint
	Memory   PluginMemoryWrapper
	Contract PluginContractWrapper
	Stack    PluginStackWrapper
}
