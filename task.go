package alive

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

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
}

// 执行任务
func (t Task) Run(output io.Writer) error {
	cmd := exec.Command(t.Name, t.Args...)
	cmd.Dir = t.Dir

	// 获取持续输出流
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// 将 Stderr 合并到 Stdout
	cmd.Stderr = cmd.Stdout

	// 启动命令
	if err := cmd.Start(); err != nil {
		return err
	}

	// 实时读取输出
	scanner := bufio.NewScanner(stdout)
	go func() {
		if t.Format == "" {
			t.Format = t.Dir + "> (" + t.Name + " " + strings.Join(t.Args, " ") + ") %s\n"
		}
		for scanner.Scan() {
			fmt.Fprintf(output, t.Format, strings.TrimRight(enc.ConvertString(scanner.Text()), "\r\n"))
		}
	}()

	// 等待命令结束
	return cmd.Wait()
}
