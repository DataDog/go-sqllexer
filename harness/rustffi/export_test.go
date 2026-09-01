//go:build rustffi

package rustffi

// AcquireForTest puts the handle in the state a call holds while Rust is
// running, and returns the release. It exists so the concurrency guard can be
// tested deterministically instead of by racing two goroutines and hoping the
// windows overlap.
func (p *Processor) AcquireForTest() func() {
	if !p.busy.CompareAndSwap(false, true) {
		panic("rustffi: processor was already busy")
	}
	return func() { p.busy.Store(false) }
}
