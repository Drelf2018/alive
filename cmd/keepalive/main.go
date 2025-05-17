package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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

var _ io.Writer = (LogWriter)(nil)

type Config struct {
	Log   string       `yaml:"log"`
	Env   []string     `yaml:"env"`
	Tasks []alive.Task `yaml:"tasks"`
}

func (cfg Config) Run(ctx context.Context, dir string) (cancel context.CancelFunc) {
	if len(cfg.Tasks) == 0 {
		return nil
	}
	if cfg.Log == "" {
		cfg.Log = "logs/2006-01-02.log"
	}

	ctx, cancel = context.WithCancel(ctx)

	hook := &utils.DateHook{Format: cfg.Log}
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

	for _, task := range cfg.Tasks {
		if task.Delay > 0 {
			time.Sleep(time.Duration(1000*task.Delay) * time.Millisecond)
		}
		task.Dir = filepath.Join(dir, task.Dir)
		task.Env = append(cfg.Env, task.Env...)
		if task.Format == "" {
			task.Format = "> " + task.Name + " " + strings.Join(task.Args, " ") + "\n%s\n"
			if task.Dir != "." {
				task.Format = task.Dir + task.Format
			}
		}
		task.Out = LogWriter(logger.Info)
		task.Err = LogWriter(logger.Error)
		go task.RunForever(ctx)
	}

	return cancel
}

func WriteYAML(path string, in any) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	f.WriteString("# 时间单位为浮点数秒\n\n")
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	err = enc.Encode(in)
	if err1 := enc.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err2 := f.Close(); err2 != nil && err == nil {
		err = err2
	}
	return err
}

func main() {
	cfgPath := flag.String("cfg", "config.yml", "configuration file path")
	flag.Parse()

	if _, err := os.Stat(*cfgPath); err != nil {
		cfg := Config{
			Log: "logs/2006-01-02.log",
			Env: []string{"PYTHONIOENCODING=utf-8"},
			Tasks: []alive.Task{{
				Dir:      "",
				Env:      []string{"LOGURU_FORMAT={name}.{function}:{line} | {message}"},
				Name:     "python",
				Args:     []string{"-u", "-c", "print(\"Hello KeepAlive!\")"},
				Delay:    0.5,
				Interval: 2.5,
			}}}
		err = WriteYAML(*cfgPath, cfg)
		if err != nil {
			panic(err)
		} else {
			panic(fmt.Errorf("alive/cmd/keepalive: 请完善配置文件: %s", *cfgPath))
		}
	}

	data, err := os.ReadFile(*cfgPath)
	if err != nil {
		panic(err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	cfg.Run(ctx, filepath.Dir(*cfgPath))
	<-ctx.Done()
}
