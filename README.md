# alive

任务保活

### 直接安装

```
> go install github.com/Drelf2018/alive/cmd/keepalive@latest
> keepalive --cfg=config.yml --log=logs/2006-01-02.log
```

程序报错了！为什么？原来是要完善配置文件 `config.yml`

```yaml
# config.yml

name: 任务
desc: 任务自身可以绑定命令，也可以在子任务集绑定更多任务，任务的工作目录、环境和输出模板会传递给子任务
env:
  - PYTHONIOENCODING=utf-8
cmd: cmd
args:
  - /C
  - echo
  - Running!
format: '{{if ne .Dir "."}}{{.Dir}}{{end}}> {{.Cmd}}{{range .Args}} {{.}}{{end}}{{endl}}%s{{endl}}'
interval: -1
tasks:
  - name: python 脚本
    desc: 用命令行打印文本
    dir: logs
    env:
      - LOGURU_FORMAT={name}.{function}:{line} | {message}
    cmd: python
    args:
      - -u
      - -c
      - print("Hello KeepAlive!")
    delay: 1
    interval: 2.5
```

这是程序预设并自动生成的配置文件，看到这一系列参数肯定头大，别急，我带你们读

| 参数名   | 参数类型 | 参数含义                                                     |
| -------- | -------- | ------------------------------------------------------------ |
| name     | string   | 任务名称                                                     |
| desc     | string   | 任务介绍                                                     |
| dir      | string   | 工作路径，会将该值拼接在此任务的~~父~~母任务的工作路径后     |
| env      | []string | 环境变量，会继承~~父~~母任务的环境变量                       |
| cmd      | string   | 命令名                                                       |
| args     | []string | 命令参数                                                     |
| format   | string   | 输出模板，会通过 `go` 语言自带的 `text/template` 对内容进行填充，再将真正的输出内容填充进 `%s` 里面，特别的 `{{endl}}` 表示换行符 |
| delay    | float64  | 延迟启动，相较于同一任务集上一任务开始运行一段时间后再启动，单位秒 |
| interval | float64  | 重试间隔，当任务完成或报错终止后，等待一段时间再次运行该命令，负值则不再启动，单位秒 |
| tasks    | []Task   | 子任务集，也就是这个表中字段组成的对象的列表                 |

会了没？还没会？那咋办？
