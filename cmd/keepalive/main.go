package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Drelf2018/alive"
	"github.com/Drelf2018/webhook/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	nested "github.com/antonfisher/nested-logrus-formatter"
)

type LogWriter func(...any)

func (l LogWriter) Write(p []byte) (n int, err error) {
	l(string(p))
	return len(p), io.EOF
}

var _ io.Writer = (LogWriter)(nil)

type Config struct {
	Log   string       `yaml:"log"`
	Tasks []alive.Task `yaml:"tasks"`
}

func (cfg Config) Run(dir string) {
	if len(cfg.Tasks) == 0 {
		return
	}
	if cfg.Log == "" {
		cfg.Log = "logs/2006-01-02.log"
	}

	hook := &utils.DateHook{Format: cfg.Log}
	logger := &logrus.Logger{
		Out:   io.MultiWriter(hook, os.Stdout),
		Hooks: make(logrus.LevelHooks),
		Formatter: &nested.Formatter{
			HideKeys:        true,
			NoColors:        true,
			TimestampFormat: "15:04:05",
			ShowFullLevel:   true,
		},
		Level: logrus.DebugLevel,
	}
	logger.AddHook(hook)

	for i, task := range cfg.Tasks {
		task.Dir = filepath.Join(dir, task.Dir)
		if task.Format == "" {
			task.Format = task.Dir + "> (" + task.Name + " " + strings.Join(task.Args, " ") + ") %s"
		}
		task.Out = LogWriter(logger.Info)
		task.Err = LogWriter(logger.Error)

		if i == len(cfg.Tasks)-1 {
			task.RunForever(context.Background())
		} else {
			go task.RunForever(context.Background())
		}
	}
}

func WriteYAML(path string, in any) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
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
		cfg := Config{Log: "logs/2006-01-02.log", Tasks: []alive.Task{{
			Name:     "cmd",
			Args:     []string{"/C", "echo", "Hello KeepAlive!"},
			Interval: 1000,
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

	cfg.Run(filepath.Dir(*cfgPath))
}
