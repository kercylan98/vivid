package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RotatingWriter 实现按大小/时间轮转的文件 Writer。
// 该实现线程安全，可在高并发日志输出场景中安全使用。
type RotatingWriter struct {
	mu         sync.Mutex
	path       string
	append     bool
	file       *os.File
	size       int64
	nextRotate time.Time
	options    RotationOptions
}

// RotatingWriterOptions 定义创建轮转 Writer 时的配置。
type RotatingWriterOptions struct {
	Path     string
	Append   bool
	Rotation RotationOptions
}

// NewRotatingWriter 创建一个支持轮转的 Writer。
func NewRotatingWriter(options RotatingWriterOptions) (*RotatingWriter, error) {
	if options.Path == "" {
		return nil, fmt.Errorf("log file path is empty")
	}

	rw := &RotatingWriter{
		path:    options.Path,
		append:  options.Append,
		options: normalizeRotationOptions(options.Rotation),
	}

	if err := rw.openFile(); err != nil {
		return nil, err
	}

	return rw, nil
}

// Write 将日志内容写入文件，并在需要时触发轮转。
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		if err := w.openFile(); err != nil {
			return 0, err
		}
	}

	if w.shouldRotate(len(p)) {
		if err := w.rotate(time.Now()); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// Close 关闭底层文件句柄。
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	err := w.file.Close()
	w.file = nil
	return err
}

func (w *RotatingWriter) openFile() error {
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return err
	}

	flags := os.O_CREATE | os.O_WRONLY
	if w.append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(w.path, flags, 0o644)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}

	w.file = file
	if w.append {
		w.size = info.Size()
	} else {
		w.size = 0
	}

	w.resetNextRotate(time.Now())
	return nil
}

func (w *RotatingWriter) shouldRotate(incoming int) bool {
	if w.options.Policy == RotationNone {
		return false
	}

	shouldBySize := false
	shouldByTime := false

	if (w.options.Policy == RotationBySize || w.options.Policy == RotationBySizeAndTime) && w.options.MaxSize > 0 {
		shouldBySize = w.size+int64(incoming) > w.options.MaxSize
	}

	if (w.options.Policy == RotationByTime || w.options.Policy == RotationBySizeAndTime) && w.options.Interval > 0 {
		shouldByTime = time.Now().After(w.nextRotate)
	}

	return shouldBySize || shouldByTime
}

func (w *RotatingWriter) rotate(now time.Time) error {
	if w.file == nil {
		return nil
	}

	if w.size == 0 {
		w.resetNextRotate(now)
		return nil
	}

	if err := w.file.Close(); err != nil {
		return err
	}
	w.file = nil

	rotatedName, err := w.rotatedFilename(now)
	if err != nil {
		return err
	}

	if err := os.Rename(w.path, rotatedName); err != nil {
		return err
	}

	if err := w.openFile(); err != nil {
		return err
	}

	return w.cleanup(now)
}

func (w *RotatingWriter) rotatedFilename(now time.Time) (string, error) {
	dir := filepath.Dir(w.path)
	base := filepath.Base(w.path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	ts := now.Format("20060102-150405")

	for i := 0; i < 1000; i++ {
		suffix := ts
		if i > 0 {
			suffix = fmt.Sprintf("%s-%d", ts, i)
		}
		filename := filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, suffix, ext))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return filename, nil
		}
	}

	return "", fmt.Errorf("unable to create rotated file name for %s", w.path)
}

func (w *RotatingWriter) cleanup(now time.Time) error {
	if w.options.MaxBackups <= 0 && w.options.MaxAge <= 0 && w.options.MaxTotalSize <= 0 {
		return nil
	}

	dir := filepath.Dir(w.path)
	base := filepath.Base(w.path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	prefix := name + "."

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var files []rotatedFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		entryName := entry.Name()
		if !strings.HasPrefix(entryName, prefix) || filepath.Ext(entryName) != ext {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, rotatedFile{
			path:    filepath.Join(dir, entryName),
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	var remaining []rotatedFile
	for _, file := range files {
		if w.options.MaxAge > 0 && now.Sub(file.modTime) > w.options.MaxAge {
			_ = os.Remove(file.path)
			continue
		}
		remaining = append(remaining, file)
	}

	if w.options.MaxBackups > 0 && len(remaining) > w.options.MaxBackups {
		for _, file := range remaining[w.options.MaxBackups:] {
			_ = os.Remove(file.path)
		}
		remaining = remaining[:w.options.MaxBackups]
	}

	if w.options.MaxTotalSize > 0 {
		var total int64
		for _, file := range remaining {
			total += file.size
		}
		if total > w.options.MaxTotalSize {
			for i := len(remaining) - 1; i >= 0 && total > w.options.MaxTotalSize; i-- {
				file := remaining[i]
				_ = os.Remove(file.path)
				total -= file.size
			}
		}
	}

	return nil
}

func (w *RotatingWriter) resetNextRotate(now time.Time) {
	if w.options.Policy == RotationByTime || w.options.Policy == RotationBySizeAndTime {
		if w.options.Interval > 0 {
			w.nextRotate = now.Add(w.options.Interval)
			return
		}
	}
	w.nextRotate = time.Time{}
}

type rotatedFile struct {
	path    string
	modTime time.Time
	size    int64
}

func normalizeRotationOptions(options RotationOptions) RotationOptions {
	if options.Policy != RotationNone {
		return options
	}

	if options.MaxSize > 0 && options.Interval > 0 {
		options.Policy = RotationBySizeAndTime
	} else if options.MaxSize > 0 {
		options.Policy = RotationBySize
	} else if options.Interval > 0 {
		options.Policy = RotationByTime
	}

	return options
}
