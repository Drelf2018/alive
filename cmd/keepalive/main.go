package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Drelf2018/alive"
	"github.com/Drelf2018/webhook/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	nested "github.com/antonfisher/nested-logrus-formatter"
)

type LogWriter func(...any)

func (l LogWriter) Write(p []byte) (n int, err error) {
	l(string(p))
	return len(p), nil
}

func WriteYAML(path string, in any) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	defer enc.Close()

	enc.SetIndent(2)
	return enc.Encode(in)
}

func main() {
	var cfgPath string
	var logPath string

	flag.StringVar(&cfgPath, "cfg", "config.yml", "configuration file path")
	flag.StringVar(&logPath, "log", "logs/2006-01-02.log", "log file path")
	flag.Parse()

	if _, err := os.Stat(cfgPath); err != nil {
		task := alive.Task{
			Name:     "任务",
			Desc:     "任务自身可以绑定命令，也可以在子任务集绑定更多任务，任务的工作目录、环境和输出模板会传递给子任务",
			Env:      []string{"PYTHONIOENCODING=utf-8"},
			Cmd:      "cmd",
			Args:     []string{"/C", "echo", "Running!"},
			Format:   "{{if ne .Dir \".\"}}{{.Dir}}{{end}}> {{.Cmd}}{{range .Args}} {{.}}{{end}}{{endl}}%s{{endl}}",
			Interval: -1,
			Tasks: []alive.Task{{
				Name:     "python 脚本",
				Desc:     "用命令行打印文本",
				Dir:      "logs",
				Env:      []string{"LOGURU_FORMAT={name}.{function}:{line} | {message}"},
				Cmd:      "python",
				Args:     []string{"-u", "-c", "print(\"Hello KeepAlive!\")"},
				Delay:    1,
				Interval: 2.5,
			}},
		}
		err = WriteYAML(cfgPath, task)
		if err != nil {
			panic(err)
		} else {
			panic(fmt.Errorf("alive/cmd/keepalive: 请完善配置文件: %s", cfgPath))
		}
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		panic(err)
	}

	var task alive.Task
	err = yaml.Unmarshal(data, &task)
	if err != nil {
		panic(err)
	}

	if len(task.Tasks) == 0 {
		return
	}

	hook := &utils.DateHook{Format: logPath}
	logger := &logrus.Logger{
		Out:   io.MultiWriter(hook, os.Stdout),
		Hooks: make(logrus.LevelHooks),
		Formatter: &nested.Formatter{
			HideKeys:        true,
			NoColors:        true,
			TimestampFormat: "15:04:05.000",
			ShowFullLevel:   true,
		},
		Level: logrus.DebugLevel,
	}
	logger.AddHook(hook)

	task.Dir = filepath.Dir(cfgPath)
	task.Out = LogWriter(logger.Info)
	task.Err = LogWriter(logger.Error)

	ctx := context.Background()
	if task.Cmd != "" {
		go task.RunForever(ctx)
	}
	go task.RunTasks(ctx)
	<-ctx.Done()
}
