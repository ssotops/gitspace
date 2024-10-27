// plugin/buffered_writer.go

package plugin

import (
	"fmt"
	"io"
)

// plugin/buffered_writer.go

func (bwc *bufferedWriteCloser) Write(p []byte) (n int, err error) {
	if bwc.logger == nil {
		// Fallback if logger is not initialized
		n, err = bwc.Writer.Write(p)
		if err != nil {
			return n, err
		}
		return n, bwc.Writer.Flush()
	}

	bwc.mu.Lock()
	defer bwc.mu.Unlock()

	if bwc.closed {
		return 0, io.ErrClosedPipe
	}

	// Add logging context
	bwc.logger.Debug("Writing to buffered writer",
		"dataLength", len(p),
		"data", fmt.Sprintf("%x", p))

	n, err = bwc.Writer.Write(p)
	if err != nil {
		bwc.logger.Error("Failed to write to buffer",
			"error", err,
			"errorType", fmt.Sprintf("%T", err),
			"bytesWritten", n)
		return n, err
	}

	bwc.logger.Debug("Flush after write",
		"bytesWritten", n)
	return n, bwc.Writer.Flush()
}

func (bwc *bufferedWriteCloser) Flush() error {
	bwc.mu.Lock()
	defer bwc.mu.Unlock()

	if bwc.closed {
		return io.ErrClosedPipe
	}

	bwc.logger.Debug("Flushing buffered writer")
	err := bwc.Writer.Flush()
	if err != nil {
		bwc.logger.Error("Failed to flush buffer",
			"error", err,
			"errorType", fmt.Sprintf("%T", err))
	} else {
		bwc.logger.Debug("Buffer flushed successfully")
	}
	return err
}

func (bwc *bufferedWriteCloser) Close() error {
	bwc.mu.Lock()
	defer bwc.mu.Unlock()

	if bwc.closed {
		bwc.logger.Debug("Writer already closed")
		return nil
	}

	bwc.logger.Debug("Closing buffered writer")
	bwc.closed = true

	if err := bwc.Flush(); err != nil {
		bwc.logger.Error("Failed to flush on close",
			"error", err,
			"errorType", fmt.Sprintf("%T", err))
		return err
	}

	err := bwc.closer.Close()
	if err != nil {
		bwc.logger.Error("Failed to close underlying writer",
			"error", err,
			"errorType", fmt.Sprintf("%T", err))
	} else {
		bwc.logger.Debug("Writer closed successfully")
	}
	return err
}

// func (bwc *bufferedWriteCloser) Write(p []byte) (n int, err error) {
// 	bwc.mu.Lock()
// 	defer bwc.mu.Unlock()

// 	if bwc.closed {
// 		return 0, io.ErrClosedPipe
// 	}

// 	// Add logging context
// 	bwc.logger.Debug("Writing to buffered writer",
// 		"dataLength", len(p),
// 		"data", fmt.Sprintf("%x", p))

// 	n, err = bwc.Writer.Write(p)
// 	if err != nil {
// 		bwc.logger.Error("Failed to write to buffer",
// 			"error", err,
// 			"errorType", fmt.Sprintf("%T", err),
// 			"bytesWritten", n)
// 		return n, err
// 	}

// 	bwc.logger.Debug("Flush after write",
// 		"bytesWritten", n)
// 	return n, bwc.Writer.Flush()
// }

// func (bwc *bufferedWriteCloser) Flush() error {
// 	bwc.mu.Lock()
// 	defer bwc.mu.Unlock()

// 	if bwc.closed {
// 		return io.ErrClosedPipe
// 	}

// 	bwc.logger.Debug("Flushing buffered writer")
// 	err := bwc.Writer.Flush()
// 	if err != nil {
// 		bwc.logger.Error("Failed to flush buffer",
// 			"error", err,
// 			"errorType", fmt.Sprintf("%T", err))
// 	} else {
// 		bwc.logger.Debug("Buffer flushed successfully")
// 	}
// 	return err
// }

// func (bwc *bufferedWriteCloser) Close() error {
// 	bwc.mu.Lock()
// 	defer bwc.mu.Unlock()

// 	if bwc.closed {
// 		bwc.logger.Debug("Writer already closed")
// 		return nil
// 	}

// 	bwc.logger.Debug("Closing buffered writer")
// 	bwc.closed = true

// 	if err := bwc.Flush(); err != nil {
// 		bwc.logger.Error("Failed to flush on close",
// 			"error", err,
// 			"errorType", fmt.Sprintf("%T", err))
// 		return err
// 	}

// 	err := bwc.closer.Close()
// 	if err != nil {
// 		bwc.logger.Error("Failed to close underlying writer",
// 			"error", err,
// 			"errorType", fmt.Sprintf("%T", err))
// 	} else {
// 		bwc.logger.Debug("Writer closed successfully")
// 	}
// 	return err
// }
