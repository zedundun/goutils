package shmqueue

import (
	"errors"
	"sync/atomic"
	"unsafe"
)

var (
	ErrBlockR   = errors.New("BlockR")
	ErrBlockW   = errors.New("BlockW")
	ErrEOF      = errors.New("EOF")
	ErrTruncate = errors.New("truncate")
)

type ShmQueue interface {
	Push([]byte) error
	Pop([]byte) error
	Destroy()
}

type shmQueue struct {
	rIdx  uint64
	wIdx  uint64
	nDrop uint64
	max   uint64
	each  uint64

	buffer []byte

	sem BinarySem
}

func New(segmentSize int, total int) (ShmQueue, error) {
	var q shmQueue
	q.max = uint64(total)
	q.buffer = make([]byte, segmentSize*total)
	q.sem = NewBinarySem()
	return &q, nil
}

func (q *shmQueue) Destroy() {
	q.sem.Destroy()
}

func (q *shmQueue) Push(data []byte) error {
	remain := q.wIdx - q.rIdx
	sem := q.sem

	if remain == q.max {
		q.nDrop++
		return ErrBlockW
	}
	// do copy
	atomic.AddUint64(&q.wIdx, 1)

	offset := q.wIdx % q.max

	length := len(data)
	if uint64(length) > q.each {
		length = int(q.each)
	}

	var uLen = uint32(length)
	p := unsafe.Pointer(&uLen)
	p1 := (*[4]byte)(p)

	//write data size
	copy(q.buffer[offset:], (*p1)[0:])
	copy(q.buffer[offset+4:], data[:length])

	if remain == 1 {
		sem.Give(false)
	}
	return nil
}

func (q *shmQueue) Pop(data []byte) error {
	sem := q.sem
	var err error
	for {
		remain := q.wIdx - q.rIdx
		if remain == 0 {
			if ok := sem.Take(); !ok {
				return ErrEOF
			}
		} else {
			offset := q.rIdx % q.max

			var uLen uint32
			p := unsafe.Pointer(&uLen)
			p1 := (*[4]byte)(p)

			//read data size
			copy((*p1)[0:], q.buffer[offset:])

			capLen := uint64(cap(data))
			if capLen > q.each {
				capLen = q.each
			} else if capLen < uint64(uLen) {
				err = ErrTruncate
			}

			copy(data, q.buffer[offset+4:offset+4+capLen])
			break
		}
	}

	atomic.AddUint64(&q.rIdx, 1)
	return err
}

func (q *shmQueue) PopNoneblock(data []byte) error {
	remain := q.wIdx - q.rIdx
	if remain == 0 {
		return ErrBlockR
	}

	var err error
	offset := q.rIdx % q.max

	var uLen uint32
	p := unsafe.Pointer(&uLen)
	p1 := (*[4]byte)(p)

	//read data size
	copy((*p1)[0:], q.buffer[offset:])

	capLen := uint64(cap(data))
	if capLen > q.each {
		capLen = q.each
	} else if capLen < uint64(uLen) {
		err = ErrTruncate
	}
	copy(data, q.buffer[offset+4:offset+4+capLen])
	return err
}