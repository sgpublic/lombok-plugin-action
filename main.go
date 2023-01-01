package main

import (
	"flag"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/mattn/go-colorable"
	cron3 "github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"lombok-plugin-action/src/config"
	"lombok-plugin-action/src/git"
	"lombok-plugin-action/src/lombok"
	"lombok-plugin-action/src/util"
	"lombok-plugin-action/src/versions/as"
	"lombok-plugin-action/src/versions/iu"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	service = false
	cron    = "" // 0 * 2 * * *
	debug   = false
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
	} else {
		config.KeepWhenException = true
	}

	c := cron3.New()
	c.AddFunc(cron, doAction)
	log.Infof("lombok-plugin-action started!")
	c.Run()
}

func initFlag() {
	flag.StringVar(&git.TOKEN, "token", "", "Security Token")
	flag.StringVar(&git.REPO, "repo", "", "Target repo")
	flag.BoolVar(&debug, "debug", false, "Debug mod")
	flag.BoolVar(&service, "service", false, "Service mod")
	flag.StringVar(&cron, "cron", "", "Crontab operation")
	flag.Parse()
}

func initLogrus() {
	log.SetOutput(colorable.NewColorableStdout())
	log.SetFormatter(util.LogFormat{EnableColor: true})

	log.RegisterExitHandler(func() {
		_ = os.RemoveAll("/tmp/lombok-plugin/")
	})

	rotateOptions := []rotatelogs.Option{
		rotatelogs.WithRotationTime(time.Hour * 24),
	}
	rotateOptions = append(rotateOptions, rotatelogs.WithMaxAge(259200*time.Second))
	err := os.MkdirAll("/var/log/lombok/", 0644)
	if err != nil {
		log.Errorf("log dir init err: %v", err)
		return
	}
	w, err := rotatelogs.New(path.Join("/var/log/lombok/", "%Y-%m-%d.log"), rotateOptions...)
	if err != nil {
		log.Errorf("rotatelogs init err: %v", err)
	} else {
		log.AddHook(util.NewLocalHook(w, debug))
	}

	// debug mode
	if debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Enable debug mode!")
	} else {
		log.SetLevel(log.InfoLevel)
	}
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
			note, prerelease := lombok.CreateReleaseNote(verNames)
			if release.GetBody() == note && *release.Prerelease == prerelease {
				log.Warnf("Tag of %s is up to date, skip.", verTag)
				continue
			}
			release.Body = &note
			release.Prerelease = &prerelease
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
	log.Info("Updating lombok plugin finish!")
}
