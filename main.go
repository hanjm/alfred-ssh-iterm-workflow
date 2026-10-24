package main

import (
	"github.com/deanishe/awgo"
	"github.com/hanjm/errors"
	"github.com/hanjm/log"
	"os"
	"strings"
)

var wf *aw.Workflow

func run() {
	// # 解析命令行参数
	// ## 第1个参数  查询条件
	var query string
	if len(os.Args) > 1 {
		query = os.Args[1]
	}
	// ## 第2个参数 用 iTerm的 哪个 profile 打开
	var iTermProfileName = "Default"
	if len(os.Args) > 2 {
		iTermProfileName = os.Args[2]
	}
	// ## 第3个参数 自定义的 iTerm user var key
	var userVarKey = "sshHost"
	if len(os.Args) > 3 {
		userVarKey = os.Args[3]
	}
	// # 解析 ssh config
	hosts, err := GetSSHHostList()
	if err != nil {
		err = errors.Errorf(err, "failed to get ssh hosts")
		wf.FatalError(err)
		return
	}
	log.Infof("hosts:%+v, query:%s", hosts, query)
	for _, v := range hosts {
		if strings.Contains(v, query) {
			arg, err := sshWorkflowArg(iTermProfileName, v, userVarKey)
			if err != nil {
				wf.FatalError(err)
				return
			}
			wf.NewItem(v).Arg(arg).Valid(true)
		}
	}
	wf.SendFeedback()
}

func main() {
	wf = aw.New()
	wf.Run(run)
}
