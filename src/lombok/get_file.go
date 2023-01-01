package lombok

import (
	"github.com/cavaliergopher/grab/v3"
	log "github.com/sirupsen/logrus"
	"lombok-plugin-action/src/util/targz"
	"lombok-plugin-action/src/util/zip"
	"os"
	"strings"
	"time"
)

func GetVersion(url string, version string) (string, error) {
	os.MkdirAll("/tmp/lombok-plugin/", 0744)

	compress := "/tmp/lombok-plugin/" + version
	path := compress + "/ideaU.zip"

	log.Infof("Start download: %s", url)
	client := grab.NewClient()
	client.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36 Edg/105.0.1343.33"
	var req *grab.Request
	var resp *grab.Response
	var err error
	retry := -1
download:
	retry = retry + 1
	req, _ = grab.NewRequest(path, url)
	resp = client.Do(req)
	for !resp.IsComplete() {
		time.Sleep(time.Second)
		log.Infof("Download %s...  %.2f%%", version, resp.Progress()*100)
	}
	if err = resp.Err(); err != nil {
		if retry < 5 {
			goto download
		}
		return "", err
	}

	log.Infof("Download finish: %s", version)
	prefix := "plugins/lombok"
	err = zip.DeCompress(path, prefix, compress)
	if err != nil {
		log.Debug("DeCompress error")
		return "", err
	}
	open, err := os.Open(compress + "/" + prefix + "/")
	if err != nil {
		log.Debug("Compress prepare error")
		return "", err
	}
	gzipFile := "/tmp/lombok-plugin/lombok-" + version + ".tar.gz"

	err = targz.Compress([]*os.File{open}, gzipFile)
	if err != nil {
		log.Debug("Compress error")
		return "", err
	}
	return gzipFile, nil
}

func CreateReleaseNote(verNames []string) (string, bool) {
	result := ""
	prerelease := true
	for _, name := range verNames {
		result += "\n+ " + name
		if prerelease && !strings.Contains(name, "Beta") &&
			!strings.Contains(name, "Canary") && !strings.Contains(name, "RC") {
			prerelease = false
		}
	}
	return result, prerelease
}
