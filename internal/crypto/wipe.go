package crypto

import "runtime"

// Zero overwrites every byte of b with 0. runtime.KeepAlive prevents the
// compiler from eliding the writes when b appears to be dead after the call.
//
// Best-effort: a sufficiently aggressive compiler or GC may still hold copies.
// For defence in depth this is paired with explicit re-derivation from the MK
// at use-sites for sensitive keys.
func Zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
