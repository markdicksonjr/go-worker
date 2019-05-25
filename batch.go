package worker

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

type BatchHandler func([]interface{}) error

func (b *Batch) Init(batchSize int, pushHandler BatchHandler, flushHandler BatchHandler) {
	b.batchPosition = 0

	// grab the batch size - default to 100
	b.batchSize = batchSize
	if b.batchSize == 0 {
		b.batchSize = 100
	}

	b.pushHandler = pushHandler
	b.flushHandler = flushHandler
	b.mutex = &sync.Mutex{}
}

func (b *Batch) Push(record interface{}) error {
	if b.batchSize == 0 {
		return errors.New("batch not initialized")
	}

	// lock around batch processing
	b.mutex.Lock()

	// allocate the buffer of items to save, if needed
	if b.itemsToSave == nil {
		b.itemsToSave = make([]interface{}, b.batchSize, b.batchSize)
		b.batchPosition = 0
		b.mutex.Unlock()
	} else if b.batchPosition >= b.batchSize {
		batch := b.itemsToSave

		// allocate a new buffer, put the inbound record as the first item
		b.itemsToSave = make([]interface{}, b.batchSize, b.batchSize)
		b.itemsToSave[0] = record
		b.batchPosition = 1

		// release the lock
		b.mutex.Unlock()

		if err := b.pushHandler(batch); err != nil {
			return err
		}

		// dereference batch to clue GC, unless user wants to retain data
		batch = nil
	} else {
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
	if len(b.itemsToSave) > 0 {

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
