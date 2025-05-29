package alive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/axgle/mahonia"
)

var enc = mahonia.NewDecoder("UTF-8")

type ChanWriter chan []byte

func (c ChanWriter) Write(p []byte) (int, error) {
	data := make([]byte, len(p))
	copy(data, p)
	c <- data
	return len(p), nil
}

// 任务
type Task struct {
	// 任务名称
	Name string `yaml:"name,omitempty" json:"name,omitempty" toml:"name,omitempty"`

	// 任务介绍
	Desc string `yaml:"desc,omitempty" json:"desc,omitempty" toml:"desc,omitempty"`

	// 工作路径
	Dir string `yaml:"dir,omitempty" json:"dir,omitempty" toml:"dir,omitempty"`

	// 环境变量
	Env []string `yaml:"env,omitempty" json:"env,omitempty" toml:"env,omitempty"`

	// 命令名
	Cmd string `yaml:"cmd,omitempty" json:"cmd,omitempty" toml:"cmd,omitempty"`

	// 命令参数
	Args []string `yaml:"args,omitempty" json:"args,omitempty" toml:"args,omitempty"`

	// 输出模板
	Format string `yaml:"format,omitempty" json:"format,omitempty" toml:"format,omitempty"`

	// 延迟启动
	Delay float64 `yaml:"delay,omitempty" json:"delay,omitempty" toml:"delay,omitempty"`

	// 重试间隔
	Interval float64 `yaml:"interval,omitempty" json:"interval,omitempty" toml:"interval,omitempty"`

	// 子任务
	Tasks []Task `yaml:"tasks,omitempty" json:"tasks,omitempty" toml:"tasks,omitempty"`

	// 常规输出
	Out io.Writer `yaml:"-" json:"-" toml:"-"`

	// 错误输出
	Err io.Writer `yaml:"-" json:"-" toml:"-"`

	// 合并输出间隔
	MergeThreshold time.Duration `yaml:"-" json:"-" toml:"-"`

	// 强制输出超时
	FlushTimeout time.Duration `yaml:"-" json:"-" toml:"-"`
}

// 模板输出
func (t *Task) Fprint(w io.Writer, s string) (n int, err error) {
	if w == nil || t.Format == "" {
		return 0, nil
	}
	return fmt.Fprintf(w, t.Format, strings.TrimRight(enc.ConvertString(s), "\r\n"))
}

// 任务输出包装器
func (t *Task) WriterWrapper(ctx context.Context, w io.Writer) io.Writer {
	if w == nil || t.Format == "" {
		return nil
	}
	buf := &bytes.Buffer{}
	chw := make(ChanWriter, 100*t.FlushTimeout/time.Second) // 每秒 100 条消息的缓冲应该够用了吧
	printf := func() {
		if buf.Len() != 0 {
			t.Fprint(w, buf.String())
			buf.Reset()
		}
	}
	go func() {
		merge := time.NewTimer(t.MergeThreshold)
		flush := time.NewTicker(t.FlushTimeout)
		defer flush.Stop()
		defer merge.Stop()
		for {
			select {
			case b := <-chw:
				buf.Write(b)
				merge.Reset(t.MergeThreshold)
			case <-merge.C:
				printf()
			case <-flush.C:
				printf()
			case <-ctx.Done():
				return
			}
		}
	}()
	return chw
}

// 携带上下文执行任务
func (t Task) RunWithContext(ctx context.Context) error {
	// 初始化命令
	cmd := exec.CommandContext(ctx, t.Cmd, t.Args...)
	cmd.Env = append(cmd.Env, t.Env...)
	cmd.Dir = t.Dir

	// 配置日志打印
	if t.Format != "" && (t.Out != nil || t.Err != nil) {
		if t.MergeThreshold == 0 {
			t.MergeThreshold = 100 * time.Millisecond
		}
		if t.FlushTimeout == 0 {
			t.FlushTimeout = time.Second
		}
		ctx, cancel := context.WithCancel(ctx)
		defer time.AfterFunc(2*t.FlushTimeout, cancel)
		cmd.Stdout = t.WriterWrapper(ctx, t.Out)
		cmd.Stderr = t.WriterWrapper(ctx, t.Err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return err
	}

	// 等待命令结束
	return cmd.Wait()
}

// 执行任务
func (t Task) Run() error {
	return t.RunWithContext(context.Background())
}

var tmpl = template.New("format").Funcs(template.FuncMap{"endl": func() string { return "\n" }})

// 任务保活
func (t Task) RunForever(ctx context.Context) {
	if t.Format != "" {
		tmpl, err := tmpl.Parse(t.Format)
		if err != nil {
			panic(fmt.Errorf("alive: parse format %q error: %w", t.Format, err))
		}
		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, t)
		if err != nil {
			panic(fmt.Errorf("alive: parse format %q error: %w", t.Format, err))
		}
		t.Format = buf.String()
	}
	for {
		err := t.RunWithContext(ctx)
		if err != nil {
			t.Fprint(t.Err, err.Error())
		}
		if t.Interval < 0 {
			break
		}
		timer := time.NewTimer(time.Duration(1000*t.Interval) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// 运行子任务
func (t *Task) RunTasks(ctx context.Context) {
	for _, task := range t.Tasks {
		task := task // 私募了 goroutine
		if task.Delay > 0 {
			time.Sleep(time.Duration(1000*task.Delay) * time.Millisecond)
		}
		task.Dir = filepath.Join(t.Dir, task.Dir)
		task.Env = append(t.Env, task.Env...)
		if task.Format == "" {
			task.Format = t.Format
		}
		if task.Out == nil {
			task.Out = t.Out
		}
		if task.Err == nil {
			task.Err = t.Err
		}
		if task.MergeThreshold == 0 {
			task.MergeThreshold = t.MergeThreshold
		}
		if task.FlushTimeout == 0 {
			task.FlushTimeout = t.FlushTimeout
		}
		if task.Cmd != "" {
			go task.RunForever(ctx)
		}
		go task.RunTasks(ctx)
	}
}

func (t Task) String() string {
	b := &strings.Builder{}
	b.WriteString("Task(")
	first := true
	if t.Name != "" {
		b.WriteString("name=\"")
		b.WriteString(t.Name)
		b.WriteByte('"')
		first = false
	}
	if t.Desc != "" {
		if !first {
			b.WriteString(", ")
		}
		b.WriteString("desc=\"")
		b.WriteString(t.Desc)
		b.WriteByte('"')
		first = false
	}
	if t.Cmd != "" {
		if !first {
			b.WriteString(", ")
		}
		b.WriteString("cmd=\"")
		b.WriteString(t.Cmd)
		for _, arg := range t.Args {
			b.WriteByte(' ')
			b.WriteString(arg)
		}
		b.WriteByte('"')
	}
	b.WriteByte(')')
	return b.String()
}
