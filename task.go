package alive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
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
	// 工作路径
	Dir string `yaml:"dir"`

	// 环境变量
	Env []string `yaml:"env"`

	// 命令名
	Name string `yaml:"name"`

	// 命令参数
	Args []string `yaml:"args"`

	// 输出模板
	Format string `yaml:"format"`

	// 延迟启动
	Delay float64 `yaml:"delay"`

	// 重试间隔
	Interval float64 `yaml:"interval"`

	// 常规输出
	Out io.Writer `yaml:"-"`

	// 错误输出
	Err io.Writer `yaml:"-"`

	// 合并输出间隔
	MergeThreshold time.Duration `yaml:"-"`

	// 强制输出超时
	FlushTimeout time.Duration `yaml:"-"`
}

// 记录任务输出
func (t *Task) Log(ctx context.Context, w io.Writer) io.Writer {
	if w == nil || t.Format == "" {
		return nil
	}
	buf := &bytes.Buffer{}
	chw := make(ChanWriter, 100*t.FlushTimeout/time.Second) // 每秒 100 条消息应该够用了吧
	printf := func() {
		if buf.Len() != 0 {
			fmt.Fprintf(w, t.Format, strings.TrimRight(enc.ConvertString(buf.String()), "\r\n"))
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
	cmd := exec.CommandContext(ctx, t.Name, t.Args...)
	cmd.Dir = t.Dir
	cmd.Env = append(cmd.Env, t.Env...)

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
		cmd.Stdout = t.Log(ctx, t.Out)
		cmd.Stderr = t.Log(ctx, t.Err)
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

// 任务保活
func (t Task) RunForever(ctx context.Context) {
	for {
		err := t.RunWithContext(ctx)
		if err != nil && t.Err != nil && t.Format != "" {
			fmt.Fprintf(t.Err, t.Format, strings.TrimRight(enc.ConvertString(err.Error()), "\r\n"))
		}
		timer := time.NewTimer(time.Duration(1000*t.Interval) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			timer.Stop()
		}
	}
}
