package main

import (
	"flag"
	"github.com/mattn/go-colorable"
	cron3 "github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"lombok-plugin-action/src/git"
	"lombok-plugin-action/src/lombok"
	"lombok-plugin-action/src/util/formater"
	"lombok-plugin-action/src/versions/as"
	"lombok-plugin-action/src/versions/iu"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	service = false
	cron    = "" // 0 * 2 * * *
)

func init() {
	initFlag()
	initLogrus()
	git.Init()
}

func main() {
	// daemon mode
	if service {
		args := os.Args[1:]
		execArgs := make([]string, 0)
		l := len(args)
		cronEnable := false
		for i := 0; i < l; i++ {
			if strings.Compare(args[i], "-service") == 0 {
				continue
			}
			if strings.Compare(args[i], "-cron") == 0 {
				cronEnable = true
			}
			execArgs = append(execArgs, args[i])
		}
		if !cronEnable {
			execArgs = append(execArgs, "-cron", "0 0 2 * * *")
		}

		ex, _ := os.Executable()
		p, _ := filepath.Abs(ex)
		proc := exec.Command(p, execArgs...)
		err := proc.Start()
		if err != nil {
			panic(err)
		}
		log.Infof("[PID] %d", proc.Process.Pid)
		os.Exit(0)
	}

	jar, _ := cookiejar.New(nil)
	http.DefaultClient = &http.Client{
		Jar: jar,
	}

	// cron mode
	if strings.Compare(cron, "") == 0 {
		doAction()
		return
	}

	c := cron3.New()
	c.AddFunc(cron, doAction)
	c.Run()
}

func initFlag() {
	debug := false

	flag.StringVar(&git.TOKEN, "token", "", "Security Token")
	flag.StringVar(&git.REPO, "repo", "", "Target repo")
	flag.BoolVar(&debug, "debug", false, "Debug mod")
	flag.BoolVar(&service, "service", false, "Service mod")
	flag.StringVar(&cron, "cron", "", "Crontab operation")
	flag.Parse()

	// debug mode
	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

func initLogrus() {
	log.SetOutput(colorable.NewColorableStdout())
	log.SetFormatter(formater.LogFormat{EnableColor: true})
	log.RegisterExitHandler(func() {
		_ = os.RemoveAll("/tmp/lombok-plugin/")
	})
}

func doAction() {
	log.Info("Start updating lombok plugin...")

	iuVer := iu.ListVersions()
	asVer, info := as.ListVersions()
	log.Infof("Android Studio versions (%d in total):", asVer.Size())
	var item interface{}
	var hasNext bool
	for {
		log.Infoln("Sleep 10 second...")
		time.Sleep(time.Second * 10)
		item, hasNext = asVer.Dequeue()
		if !hasNext {
			break
		}

		verTag := item.(string)
		verStr, _ := info.Get(item)
		verNames := verStr.([]string)

		log.Infof("- %s:\n%s", verTag, strings.Join(verNames, "\n  > "))

		release, err := git.GetReleaseByTag(verTag)
		if err == nil {
			log.Infof("Tag of %s already exits, updateing...", verTag)
			note := lombok.CreateReleaseNote(verTag, verNames)
			if release.GetBody() == note {
				log.Warnf("Tag of %s is up to date, skip.", verTag)
				continue
			}
			release.Body = &note
			err = git.UpdateReleaseBody(release)
			if err != nil {
				log.Warnf("Tag of %s update failed.", verTag)
			} else {
				log.Warnf("Tag of %s update success.", verTag)
			}
			continue
		}

		url, _ := iuVer.Get(item)
		if url == nil {
			log.Warnf("Version %s exists in Android Studio, but not exists in IDEA.", verTag)
			continue
		}

		gzipFile, err := lombok.GetVersion(url.(string), verTag)
		if err != nil {
			log.Errorf("Failed to get version %s: %s", verTag, err.Error())
			continue
		}
		if git.CreateTag(verTag, verNames, gzipFile) != nil {
			log.Errorf("Failed to upload version %s: %s", verTag, err.Error())
		} else {
			log.Infof("Version %s upload finish.", verTag)
		}
	}
}
