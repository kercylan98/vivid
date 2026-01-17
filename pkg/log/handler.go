package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

// VividHandler 是 slog.Handler 的扩展实现。
// 它支持多输出目标、格式化与颜色控制、source 与调用栈跳过，以及 error 堆栈追踪。
type VividHandler struct {
	leveler slog.Leveler
	options HandlerConfig
	outputs []outputWriter
	attrs   []slog.Attr
	groups  []string
	limiter *rateLimiter
}

// NewHandler 创建日志 Handler。
// 若未指定任何输出目标，则默认输出到 stderr，格式为文本且启用颜色。
func NewHandler(options ...Option) *VividHandler {
	opts := NewHandlerConfig(options...)
	outputs := buildOutputs(opts)

	return &VividHandler{
		leveler: opts.Level,
		options: *opts,
		outputs: outputs,
		limiter: newRateLimiter(opts.RateLimit),
	}
}

// Enabled 判断该日志等级是否需要输出。
func (h *VividHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.leveler.Level()
}

// Handle 处理日志记录并输出到各个目标。
func (h *VividHandler) Handle(_ context.Context, record slog.Record) error {
	if len(h.outputs) == 0 {
		return nil
	}

	if h.limiter != nil && !h.limiter.Allow(record.Time) {
		if shouldWarn, dropped := h.limiter.OnDrop(record.Time); shouldWarn {
			h.emitRateLimitWarning(record.Time, dropped)
		}
		return nil
	}

	var attrs []flatAttr
	var foundErr error
	h.collectAttrs(&attrs, &foundErr, record)

	for _, output := range h.outputs {
		if record.Level < output.level.Level() {
			continue
		}

		entry := logEntry{
			Time:    record.Time,
			Level:   record.Level,
			Message: record.Message,
			Source:  "",
			Attrs:   attrs,
		}

		if output.enableSource && record.PC != 0 {
			entry.Source = formatSource(record.PC)
		}

		if output.enableErrorStack && foundErr != nil {
			entry.ErrorStack = extractErrorStack(foundErr)
		}

		formatted, err := formatEntry(entry, output)
		if err != nil {
			return err
		}

		output.mu.Lock()
		_, _ = output.writer.Write(formatted)
		output.mu.Unlock()
	}

	return nil
}

// WithAttrs 返回附加属性的新 Handler。
func (h *VividHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	clone := h.clone()
	clone.attrs = append(clone.attrs, attrs...)
	return clone
}

// WithGroup 返回附加分组的新 Handler。
func (h *VividHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	clone := h.clone()
	clone.groups = append(clone.groups, name)
	return clone
}

func (h *VividHandler) clone() *VividHandler {
	attrs := make([]slog.Attr, len(h.attrs))
	copy(attrs, h.attrs)

	groups := make([]string, len(h.groups))
	copy(groups, h.groups)

	outputs := make([]outputWriter, len(h.outputs))
	copy(outputs, h.outputs)

	return &VividHandler{
		leveler: h.leveler,
		options: h.options,
		outputs: outputs,
		attrs:   attrs,
		groups:  groups,
		limiter: h.limiter,
	}
}

func (h *VividHandler) emitRateLimitWarning(now time.Time, dropped int64) {
	if dropped <= 0 {
		return
	}

	entry := logEntry{
		Time:    now,
		Level:   slog.LevelWarn,
		Message: "log rate limit triggered, entries dropped",
		Attrs: []flatAttr{
			{keyPath: []string{"dropped"}, value: dropped},
			{keyPath: []string{"rate_per_second"}, value: h.options.RateLimit.RatePerSecond},
			{keyPath: []string{"burst"}, value: h.options.RateLimit.Burst},
			{keyPath: []string{"warn_interval"}, value: h.options.RateLimit.WarnInterval},
		},
	}

	for _, output := range h.outputs {
		if slog.LevelWarn < output.level.Level() {
			continue
		}

		formatted, err := formatEntry(entry, output)
		if err != nil {
			continue
		}

		output.mu.Lock()
		_, _ = output.writer.Write(formatted)
		output.mu.Unlock()
	}
}

type outputWriter struct {
	writer           io.Writer
	mu               *sync.Mutex
	level            slog.Leveler
	timeFormat       string
	format           OutputFormat
	color            bool
	enableSource     bool
	enableErrorStack bool
}

type rateLimiter struct {
	mu               sync.Mutex
	rate             float64
	burst            float64
	tokens           float64
	last             time.Time
	droppedSinceWarn int64
	lastWarn         time.Time
	warnInterval     time.Duration
}

func newRateLimiter(options RateLimitOptions) *rateLimiter {
	if !options.Enabled || options.RatePerSecond <= 0 {
		return nil
	}
	burst := options.Burst
	if burst <= 0 {
		burst = 1
	}
	return &rateLimiter{
		rate:         float64(options.RatePerSecond),
		burst:        float64(burst),
		tokens:       float64(burst),
		last:         time.Now(),
		warnInterval: options.WarnInterval,
	}
}

func (r *rateLimiter) Allow(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if now.IsZero() {
		now = time.Now()
	}

	elapsed := now.Sub(r.last).Seconds()
	if elapsed > 0 {
		r.tokens += elapsed * r.rate
		if r.tokens > r.burst {
			r.tokens = r.burst
		}
		r.last = now
	}

	if r.tokens >= 1 {
		r.tokens -= 1
		return true
	}
	return false
}

func (r *rateLimiter) OnDrop(now time.Time) (bool, int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.droppedSinceWarn++
	if r.warnInterval <= 0 {
		return false, 0
	}

	if now.IsZero() {
		now = time.Now()
	}

	if now.Sub(r.lastWarn) >= r.warnInterval {
		dropped := r.droppedSinceWarn
		r.droppedSinceWarn = 0
		r.lastWarn = now
		return true, dropped
	}

	return false, 0
}

type logEntry struct {
	Time       time.Time
	Level      slog.Level
	Message    string
	Source     string
	Attrs      []flatAttr
	ErrorStack string
}

type flatAttr struct {
	keyPath []string
	value   any
}

func buildOutputs(options *HandlerConfig) []outputWriter {
	var outputs []outputWriter

	for _, output := range options.Outputs {
		writer := output.Writer
		if writer == nil {
			continue
		}
		outputs = append(outputs, makeOutputWriter(options, output, writer))
	}

	for _, output := range options.FileOutputs {
		writer, err := NewRotatingWriter(RotatingWriterOptions{
			Path:     output.Path,
			Append:   output.Append,
			Rotation: output.Rotation,
		})
		if err != nil {
			continue
		}
		outputs = append(outputs, makeOutputWriter(options, OutputOptions{
			Writer:           writer,
			Format:           output.Format,
			Color:            output.Color,
			TimeFormat:       output.TimeFormat,
			EnableSource:     output.EnableSource,
			EnableErrorStack: output.EnableErrorStack,
		}, writer))
	}

	if len(outputs) == 0 {
		outputs = append(outputs, makeOutputWriter(options, OutputOptions{
			Writer: os.Stderr,
			Format: OutputText,
			Color:  true,
		}, os.Stderr))
	}

	return outputs
}

func makeOutputWriter(options *HandlerConfig, output OutputOptions, writer io.Writer) outputWriter {
	timeFormat := output.TimeFormat
	if timeFormat == "" {
		timeFormat = options.TimeFormat
	}

	enableSource := options.EnableSource
	if output.EnableSource != nil {
		enableSource = *output.EnableSource
	}

	enableErrorStack := options.EnableErrorStack
	if output.EnableErrorStack != nil {
		enableErrorStack = *output.EnableErrorStack
	}

	return outputWriter{
		writer:           writer,
		format:           output.Format,
		color:            output.Color,
		timeFormat:       timeFormat,
		enableSource:     enableSource,
		enableErrorStack: enableErrorStack,
		level:            options.Level,
		mu:               &sync.Mutex{},
	}
}

func (h *VividHandler) collectAttrs(out *[]flatAttr, foundErr *error, record slog.Record) {
	groups := append([]string{}, h.groups...)
	for _, attr := range h.attrs {
		h.appendAttr(out, groups, attr, foundErr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		h.appendAttr(out, groups, attr, foundErr)
		return true
	})
}

func (h *VividHandler) appendAttr(out *[]flatAttr, groups []string, attr slog.Attr, foundErr *error) {
	if attr.Equal(slog.Attr{}) {
		return
	}

	if attr.Value.Kind() == slog.KindAny {
		if valuer, ok := attr.Value.Any().(slog.LogValuer); ok {
			attr.Value = valuer.LogValue()
		}
	}

	if attr.Value.Kind() == slog.KindGroup {
		groupAttrs := attr.Value.Group()
		nextGroups := append(append([]string{}, groups...), attr.Key)
		for _, groupAttr := range groupAttrs {
			h.appendAttr(out, nextGroups, groupAttr, foundErr)
		}
		return
	}

	keyPath := append(append([]string{}, groups...), attr.Key)
	value := valueToAny(attr.Value)

	if foundErr != nil && *foundErr == nil {
		if err, ok := value.(error); ok {
			*foundErr = err
		}
	}

	*out = append(*out, flatAttr{
		keyPath: keyPath,
		value:   value,
	})
}

func valueToAny(value slog.Value) any {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindTime:
		return value.Time()
	case slog.KindGroup:
		return value.Group()
	default:
		return value.Any()
	}
}

func formatEntry(entry logEntry, output outputWriter) ([]byte, error) {
	switch output.format {
	case OutputJSON:
		return formatJSON(entry, output)
	default:
		return formatText(entry, output), nil
	}
}

func formatText(entry logEntry, output outputWriter) []byte {
	var buffer bytes.Buffer
	appendTime(&buffer, entry.Time, output.timeFormat, output.color)
	appendLevel(&buffer, entry.Level, output.color)
	appendSource(&buffer, entry.Source, output.color)
	appendMessage(&buffer, entry.Message, output.color)
	appendAttrsText(&buffer, entry.Attrs, output.color)
	appendStack(&buffer, entry.ErrorStack, output.color)
	buffer.WriteByte('\n')
	return buffer.Bytes()
}

func appendTime(buffer *bytes.Buffer, t time.Time, format string, color bool) {
	if t.IsZero() {
		return
	}
	text := t.Format(format)
	if color {
		text = colorizeText(colorGray, text)
	}
	buffer.WriteString(text)
	buffer.WriteByte(' ')
}

func appendLevel(buffer *bytes.Buffer, level slog.Level, color bool) {
	levelText := strings.ToUpper(level.String())
	if color {
		levelText = colorizeLevel(level, levelText)
	}
	buffer.WriteString(levelText)
}

func appendMessage(buffer *bytes.Buffer, message string, color bool) {
	if message == "" {
		return
	}
	buffer.WriteByte(' ')
	if color {
		message = colorizeText(colorWhite, message)
	}
	buffer.WriteString(message)
}

func appendSource(buffer *bytes.Buffer, source string, color bool) {
	if source == "" {
		return
	}
	buffer.WriteByte(' ')
	if color {
		source = colorizeText(colorBlue, source)
	}
	buffer.WriteString(source)
}

func appendAttrsText(buffer *bytes.Buffer, attrs []flatAttr, color bool) {
	for _, attr := range attrs {
		buffer.WriteByte(' ')
		key := strings.Join(attr.keyPath, ".")
		separator := "="
		value := formatValue(attr.value)
		if color {
			key = colorizeText(colorCyan, key)
			separator = colorizeText(colorGray, separator)
			value = formatValueColored(attr.value)
		}
		buffer.WriteString(key)
		buffer.WriteString(separator)
		buffer.WriteString(value)
	}
}

func appendStack(buffer *bytes.Buffer, stack string, color bool) {
	if stack == "" {
		return
	}
	buffer.WriteByte(' ')
	content := "error_stack=" + stack
	if color {
		content = colorizeText(colorBrightRed, content)
	}
	buffer.WriteString(content)
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return `""`
		}
		if strings.ContainsAny(typed, " \t\r\n=") {
			return strconv.Quote(typed)
		}
		return typed
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	case time.Duration:
		return typed.String()
	case error:
		return strconv.Quote(typed.Error())
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func formatValueColored(value any) string {
	switch typed := value.(type) {
	case error:
		return colorizeText(colorBrightRed, strconv.Quote(typed.Error()))
	case string:
		return colorizeText(colorGreen, formatValue(value))
	case time.Time:
		return colorizeText(colorGreen, typed.Format(time.RFC3339Nano))
	case time.Duration:
		return colorizeText(colorGreen, typed.String())
	default:
		return colorizeText(colorGreen, formatValue(value))
	}
}

func formatJSON(entry logEntry, output outputWriter) ([]byte, error) {
	payload := make(map[string]any)
	if !entry.Time.IsZero() {
		payload["time"] = entry.Time.Format(output.timeFormat)
	}
	payload["level"] = entry.Level.String()
	payload["msg"] = entry.Message
	if entry.Source != "" {
		payload["source"] = entry.Source
	}
	if entry.ErrorStack != "" {
		payload["error_stack"] = entry.ErrorStack
	}

	for _, attr := range entry.Attrs {
		setNestedValue(payload, attr.keyPath, normalizeJSONValue(attr.value, output.timeFormat))
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return data, nil
}

func setNestedValue(target map[string]any, path []string, value any) {
	if len(path) == 0 {
		return
	}
	current := target
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		next, ok := current[key].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[key] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

func normalizeJSONValue(value any, timeFormat string) any {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format(timeFormat)
	case time.Duration:
		return typed.String()
	case error:
		return typed.Error()
	default:
		return typed
	}
}

func formatSource(pc uintptr) string {
	frames := runtime.CallersFrames([]uintptr{pc})
	frame, _ := frames.Next()
	path := filepath.ToSlash(frame.File)
	return fmt.Sprintf("%s:%d", trimSourcePath(path), frame.Line)
}

func trimSourcePath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

type stackProvider interface {
	Stack() []byte
}

type stackTracer interface {
	StackTrace() fmt.Formatter
}

func extractErrorStack(err error) string {
	if err == nil {
		return ""
	}

	var provider stackProvider
	if errors.As(err, &provider) {
		return string(provider.Stack())
	}

	var tracer stackTracer
	if errors.As(err, &tracer) {
		return fmt.Sprintf("%+v", tracer.StackTrace())
	}

	return string(debug.Stack())
}

const (
	colorReset        = "\x1b[0m"  //   #FFFFFF
	colorGreen        = "\x1b[32m" //   #00FF00
	colorYellow       = "\x1b[33m" //   #FFFF00
	colorCyan         = "\x1b[36m" //   #00FFFF
	colorBlue         = "\x1b[34m" //   #0000FF
	colorMagenta      = "\x1b[35m" //   #FF00FF
	colorGray         = "\x1b[90m" //   #808080
	colorWhite        = "\x1b[97m" //   #FFFFFF
	colorBrightRed    = "\x1b[91m" //   #FF5555
	colorBrightYellow = "\x1b[93m" //   #FFFF55
	colorBrightGreen  = "\x1b[92m" //   #55FF55
	colorBrightCyan   = "\x1b[96m" //   #55FFFF
)

func colorizeLevel(level slog.Level, text string) string {
	switch {
	case level >= slog.LevelError:
		return colorizeText(colorBrightRed, text)
	case level >= slog.LevelWarn:
		return colorizeText(colorBrightYellow, text)
	case level >= slog.LevelInfo:
		return colorizeText(colorBrightGreen, text)
	default:
		return colorizeText(colorBrightCyan, text)
	}
}

func colorizeText(color, text string) string {
	return color + text + colorReset
}
