package alive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/axgle/mahonia"
)

var enc = mahonia.NewDecoder("gbk")

// 任务
type Task struct {
	// 工作路径
	Dir string `yaml:"dir"`

	// 命令名
	Name string `yaml:"name"`

	// 命令参数
	Args []string `yaml:"args"`

	// 输出模板
	Format string `yaml:"format"`

	// 重试间隔
	Interval int64 `yaml:"interval"`

	// 常规输出
	Out io.Writer `yaml:"-"`

	// 错误输出
	Err io.Writer `yaml:"-"`
}

func (t *Task) Info(s string) (int, error) {
	if t.Out == nil || t.Format == "" {
		return 0, nil
	}
	return fmt.Fprintf(t.Out, t.Format, strings.TrimRight(enc.ConvertString(s), "\r\n"))
}

func (t *Task) Error(s string) (int, error) {
	if t.Err == nil || t.Format == "" {
		return 0, nil
	}
	return fmt.Fprintf(t.Err, t.Format, strings.TrimRight(enc.ConvertString(s), "\r\n"))
}

// 执行任务
func (t Task) Run() error {
	return t.RunWithContext(context.Background())
}

// 任务保活
func (t Task) RunForever(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := t.RunWithContext(ctx)
			if err != nil {
				t.Error(err.Error())
			}
			time.Sleep(time.Duration(t.Interval) * time.Millisecond)
		}
	}
}

// 携带上下文执行任务
func (t Task) RunWithContext(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, t.Name, t.Args...)
	cmd.Dir = t.Dir

	// 获取持续输出流
	stdout, errPipe := cmd.StdoutPipe()
	if errPipe != nil {
		return errPipe
	}

	stderr, errPipe := cmd.StderrPipe()
	if errPipe != nil {
		return errPipe
	}

	// 启动命令
	if errStart := cmd.Start(); errStart != nil {
		return errStart
	}

	// 实时读取输出
	outScanner := bufio.NewScanner(stdout)
	go func() {
		for outScanner.Scan() {
			t.Info(outScanner.Text())
		}
	}()

	errScanner := bufio.NewScanner(stderr)
	go func() {
		for errScanner.Scan() {
			t.Error(errScanner.Text())
		}
	}()

	// 等待命令结束
	return cmd.Wait()
}
