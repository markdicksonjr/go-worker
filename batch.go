package work

import (
	"errors"
	"sync"
)

type Batch struct {
	batchPosition int
	batchSize     int
	itemsToSave   []interface{}
	pushHandler   BatchHandler
	flushHandler  BatchHandler
	mutex         *sync.Mutex
}

// BatchSource is a convenience interface - not used directly by this module
type BatchSource interface {
	// when the caller wants to process slices of data
	// gives the batch and some context about where in the whole set
	GetBatches(
		onBatch func(batch []interface{}, batchIndex, batchSize, totalItemCount int) error,
	) error

	// when the caller wants to close/finalize assets and resources
	Finalize() error
}

// convenience interface - not used directly by this module
type BatchSourceFactory func() BatchSource

// BatchDestination is a convenience interface - not used directly by this module
type BatchDestination interface {

	// when the caller wants to put a slice of data somewhere
	PutBatch([]interface{}) error

	// when the caller wants to close/finalize assets and resources
	Finalize() error
}

// BytesSource is a convenience interface - not used directly by this module
type BytesSource interface {

	// when the caller wants to process bytes of data per batch
	// gives the batch and some context about where in the whole set
	GetBatches(
		onBatch func(bytes []byte, batchIndex, batchSize, totalItemCount int) error,
	) error

	// when the caller wants to close/finalize assets and resources
	Finalize() error
}

// note: io.WriteCloser makes a convenient alternative to "BytesDestination"

type BatchHandler func([]interface{}) error

func NewBatch(batchSize int, pushHandler BatchHandler, flushHandler ...BatchHandler) *Batch {
	b := Batch{}
	b.Init(batchSize,  pushHandler, flushHandler...)
	return &b
}

func (b *Batch) Init(batchSize int, pushHandler BatchHandler, flushHandler ...BatchHandler) {
	b.batchPosition = 0

	// grab the batch size - default to 100
	b.batchSize = batchSize
	if b.batchSize == 0 {
		b.batchSize = 100
	}

	b.pushHandler = pushHandler
	b.flushHandler = pushHandler

	if len(flushHandler) > 0 {
		b.flushHandler = flushHandler[0]
	}

	b.mutex = &sync.Mutex{}
}

func (b *Batch) Push(record interface{}) error {
	if b.batchSize == 0 {
		return errors.New("batch not initialized")
	}

	// if only one item is in the batch, don't even bother storing it
	if b.batchSize == 1 {
		return b.pushHandler([]interface{}{record})
	}

	// lock around batch processing
	b.mutex.Lock()

	// allocate the buffer of items to save, if needed
	if b.itemsToSave == nil {
		b.itemsToSave = make([]interface{}, b.batchSize, b.batchSize)
	}

	// if our batch is full
	if b.batchPosition >= b.batchSize {
		batch := b.itemsToSave

		// allocate a new buffer, put the inbound record as the first item
		b.itemsToSave = make([]interface{}, b.batchSize, b.batchSize)
		b.itemsToSave[0] = record
		b.batchPosition = 1

		// release the lock
		b.mutex.Unlock()

		// TODO: review impact of making this call from a goroutine - definitely faster, but would bugs arise from timing changes?
		if err := b.pushHandler(batch); err != nil {
			return err
		}

		batch = nil
	} else {

		// our batch is not full - if the batch size
		b.itemsToSave[b.batchPosition] = record
		b.batchPosition++
		b.mutex.Unlock()
	}

	return nil
}

func (b *Batch) GetPosition() int {
	b.mutex.Lock()
	pos := b.batchPosition
	b.mutex.Unlock()
	return pos
}

func (b *Batch) Flush() error {
	if b.batchSize == 0 {
		return errors.New("batch not initialized")
	}

	// lock around batch processing
	b.mutex.Lock()
	if b.batchPosition > 0 {

		// snag the rest of the buffer as a slice, reset buffer
		subSlice := (b.itemsToSave)[0:b.batchPosition]
		b.itemsToSave = make([]interface{}, b.batchSize, b.batchSize)
		b.batchPosition = 0

		// we've finished batch processing, unlock
		b.mutex.Unlock()

		// call the configured flush handler
		err := b.flushHandler(subSlice)
		subSlice = nil
		return err
	}
	b.mutex.Unlock()

	return nil
}
