package bpool

import (
	"bytes"
)

const (
	min_align int = 4 // buffer sizes will be always a multiple of 1<<min_align
	subs int = 3      // buffer sizes will be a multiple of a>>subs
	alpha int = 5     // smoothing factor for the exponential moving average [0 100]
)

// SizedBufferPool implements a pool of bytes.Buffers in the form of a bounded
// channel. Buffers are pre-allocated to the requested size.
type SizedBufferPool struct {
	c chan *bytes.Buffer
	a int
}

// SizedBufferPool creates a new BufferPool bounded to the given size.
// size defines the number of buffers to be retained in the pool and alloc sets
// the initial capacity of new buffers to minimize calls to make().
//
// The value of alloc should seek to provide a buffer that is representative of
// most data written to the the buffer (i.e. 95th percentile) without being
// overly large (which will increase static memory consumption). You may wish to
// track the capacity of your last N buffers (i.e. using an []int) prior to
// returning them to the pool as input into calculating a suitable alloc value.
func NewSizedBufferPool(size int, alloc int) (bp *SizedBufferPool) {
	return &SizedBufferPool{
		c: make(chan *bytes.Buffer, size),
		a: alloc,
	}
}

// Get gets a Buffer from the SizedBufferPool, or creates a new one if none are
// available in the pool. Buffers have a pre-allocated capacity.
func (bp *SizedBufferPool) Get() *bytes.Buffer {
	select {
	case b := <-bp.c:
		// reuse existing buffer
		return b
	default:
		// create new buffer
		return bp.get()
	}
}

// Put returns the given Buffer to the SizedBufferPool.
func (bp *SizedBufferPool) Put(b *bytes.Buffer) {
	// Exponential moving average of the buffer sizes (we don't use b.Cap() as-is
	// because otherwise bp.a could only increase, never decrease)
	cap := b.Cap()
	bp.a = (bp.a * (100 - alpha) + (cap - cap>>subs) * alpha) / 100
	
	// If the pool is full opportunistically throw the buffer away
	if len(bp.c) == cap(bp.c) {
		return
	} 
	
	// Release buffers over our maximum capacity and re-create a pre-sized
	// buffer to replace it.
	if b.Cap() > bp.a {
		b = bp.get()
	} else {
		b.Reset()
	}

	select {
	case bp.c <- b:
	default: // Discard the buffer if the pool is full.
	}
}

func (bp *SizedBufferPool) get() *bytes.Buffer {
	cap := bp.a
	align := nextPowerOf2(uint32(cap)) - subs
	if align < min_align {
		align = min_align
	}
	mask := (1 << align) - 1
	cap = (cap + mask) & ~mask
	return bytes.NewBuffer(make([]byte, 0, cap))
}

func nextPowerOfTwo(v uint32) uint32 {
    v--
    v |= v >> 1
    v |= v >> 2
    v |= v >> 4
    v |= v >> 8
    v |= v >> 16
    v++
    return v
}
