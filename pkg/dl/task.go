package dl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	chunkSize  int64 = 4 * 1024 * 1024 // 4 MiB
	bufferSize       = 64 * 1024       // 64 KiB
)

type Task struct {
	ID       string
	URL      string
	DestPath string
	Options  TaskOptions

	mu       sync.Mutex
	progress *Progress
	state    TaskState

	cancelFn context.CancelFunc
	ctx      context.Context
}

func newTask(id, url, destPath string, opts TaskOptions) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	return &Task{
		ID:       id,
		URL:      url,
		DestPath: destPath,
		Options:  opts,
		progress: newProgress(),
		state:    TaskPending,
		ctx:      ctx,
		cancelFn: cancel,
	}
}

func (t *Task) info() TaskInfo {
	t.mu.Lock()
	state := t.state
	t.mu.Unlock()
	return TaskInfo{
		ID:       t.ID,
		URL:      t.URL,
		DestPath: t.DestPath,
		State:    state.String(),
		Progress: t.progress.Snapshot(),
	}
}

func (t *Task) setState(s TaskState) {
	t.mu.Lock()
	t.state = s
	t.mu.Unlock()
}

func (t *Task) cancel() { t.cancelFn() }

func (t *Task) fail(err string) {
	t.progress.SetError(err)
	t.setState(TaskError)
}

func (t *Task) run() {
	t.setState(TaskDownloading)

	// 1. HEAD probe
	totalSize, supportsRange := t.probe()

	if totalSize > 0 {
		t.progress.SetTotal(totalSize)
	}

	// 2. Create / pre-allocate file
	if err := os.MkdirAll(fileDir(t.DestPath), 0755); err != nil {
		t.fail(fmt.Sprintf("创建目录失败: %v", err))
		return
	}

	file, err := os.OpenFile(t.DestPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.fail(fmt.Sprintf("创建文件失败: %v", err))
		return
	}
	defer file.Close()

	if totalSize > 0 {
		if err := file.Truncate(totalSize); err != nil {
			t.fail(fmt.Sprintf("预分配空间失败: %v", err))
			return
		}
	}

	// 3. Choose strategy
	if supportsRange && totalSize > chunkSize {
		t.downloadMulti(file, totalSize)
	} else {
		t.downloadSingle(file, totalSize)
	}

	// 4. Check result — do not overwrite already-terminal states
	if t.ctx.Err() != nil {
		t.mu.Lock()
		state := t.state
		t.mu.Unlock()
		if state != TaskDone {
			t.progress.SetError("canceled")
			t.setState(TaskCanceled)
		}
	}

	// Clean up file on failure or cancel
	if t.state != TaskDone && t.state != TaskDownloading {
		_ = os.Remove(t.DestPath)
	}
}

// probe sends HEAD request to get Content-Length and check Range support.
func (t *Task) probe() (totalSize int64, supportsRange bool) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(t.ctx, http.MethodHead, t.URL, nil)
	if err != nil {
		return 0, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	supportsRange = strings.Contains(
		strings.ToLower(resp.Header.Get("Accept-Ranges")), "bytes")
	return resp.ContentLength, supportsRange
}

// downloadSingle does a single-connection download.
func (t *Task) downloadSingle(file *os.File, totalSize int64) {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(t.ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		t.fail(err.Error())
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		if t.ctx.Err() != nil {
			return
		}
		t.fail(err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.fail(fmt.Sprintf("HTTP %d", resp.StatusCode))
		return
	}

	// Update total if not known
	if totalSize <= 0 && resp.ContentLength > 0 {
		totalSize = resp.ContentLength
		t.progress.SetTotal(totalSize)
		if err := file.Truncate(totalSize); err == nil {
			// ok
		}
	}

	buf := make([]byte, bufferSize)
	var offset int64
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.WriteAt(buf[:n], offset); writeErr != nil {
				t.fail(writeErr.Error())
				return
			}
			offset += int64(n)
			t.progress.Add(int64(n))
			if totalSize <= 0 {
				t.progress.SetTotal(offset) // growing total for unknown size
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				t.progress.SetDone()
				t.setState(TaskDone)
				return
			}
			t.fail(readErr.Error())
			return
		}
	}
}

// downloadMulti splits the file into chunks and downloads in parallel.
func (t *Task) downloadMulti(file *os.File, totalSize int64) {
	numChunks := int((totalSize + chunkSize - 1) / chunkSize)

	errCh := make(chan error, numChunks)
	sem := make(chan struct{}, t.Options.Connections)
	var wg sync.WaitGroup

	for i := 0; i < numChunks; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if end >= totalSize {
			end = totalSize - 1
		}

		select {
		case <-t.ctx.Done():
			wg.Wait()
			return
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(chunkStart, chunkEnd int64) {
			defer wg.Done()
			defer func() { <-sem }()

			var lastErr error
			for attempt := 0; attempt <= t.Options.MaxRetries; attempt++ {
				if t.ctx.Err() != nil {
					return
				}
				if attempt > 0 {
					select {
					case <-t.ctx.Done():
						return
					case <-time.After(time.Duration(1<<uint(attempt)) * time.Second):
					}
				}
				if err := t.downloadChunk(file, chunkStart, chunkEnd); err != nil {
					lastErr = err
					if !isRetryable(err) {
						break
					}
					continue
				}
				return
			}
			select {
			case errCh <- fmt.Errorf("chunk %d-%d: %w", chunkStart, chunkEnd, lastErr):
			default:
			}
		}(start, end)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		t.fail(err.Error())
	default:
		t.progress.SetDone()
		t.setState(TaskDone)
	}
}

// downloadChunk downloads a single byte range.
func (t *Task) downloadChunk(file *os.File, start, end int64) error {
	req, err := http.NewRequestWithContext(t.ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	total := end - start + 1
	var received int64
	offset := start
	buf := make([]byte, bufferSize)

	for received < total {
		select {
		case <-t.ctx.Done():
			return context.Canceled
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			toWrite := int64(n)
			remaining := total - received
			if remaining < toWrite {
				toWrite = remaining
			}
			if _, writeErr := file.WriteAt(buf[:toWrite], offset); writeErr != nil {
				return writeErr
			}
			offset += toWrite
			received += toWrite
			t.progress.Add(toWrite)
		}

		if readErr != nil {
			if readErr == io.EOF {
				if received >= total {
					return nil
				}
				return fmt.Errorf("unexpected EOF: got %d of %d bytes", received, total)
			}
			return readErr
		}
	}

	return nil
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	msg := err.Error()
	for _, code := range []string{"404", "403", "410", "416"} {
		if strings.Contains(msg, code) {
			return false
		}
	}
	return true
}

func fileDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
