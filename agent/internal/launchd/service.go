package launchd

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const Label = "com.saviolopes.ai-usage-monitor"

type Paths struct{ Binary, Plist, Stdout, Stderr string }

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{Binary: filepath.Join(home, ".local", "bin", "usage-agent"), Plist: filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), Stdout: filepath.Join(home, ".ai-usage", "agent.log"), Stderr: filepath.Join(home, ".ai-usage", "agent.error.log")}, nil
}

func Install(source string) (Paths, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return paths, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.Binary), 0700); err != nil {
		return paths, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.Plist), 0700); err != nil {
		return paths, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.Stdout), 0700); err != nil {
		return paths, err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return paths, err
	}
	temp := paths.Binary + ".tmp"
	if err := os.WriteFile(temp, data, 0755); err != nil {
		return paths, err
	}
	defer os.Remove(temp)
	if err := os.Rename(temp, paths.Binary); err != nil {
		return paths, err
	}
	home, _ := os.UserHomeDir()
	servicePath := filepath.Join(home, ".local", "bin") + ":/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
	plist := renderPlist(paths, servicePath)
	if err := os.WriteFile(paths.Plist, []byte(plist), 0600); err != nil {
		return paths, err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+Label).Run()
	var bootstrapOutput []byte
	var bootstrapErr error
	for attempt := 0; attempt < 3; attempt++ {
		bootstrapOutput, bootstrapErr = exec.Command("launchctl", "bootstrap", domain, paths.Plist).CombinedOutput()
		if bootstrapErr == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if bootstrapErr != nil {
		return paths, fmt.Errorf("launchctl bootstrap: %w: %s", bootstrapErr, strings.TrimSpace(string(bootstrapOutput)))
	}
	if output, err := exec.Command("launchctl", "kickstart", "-k", domain+"/"+Label).CombinedOutput(); err != nil {
		return paths, fmt.Errorf("launchctl kickstart: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return paths, nil
}

func Uninstall() error {
	paths, err := DefaultPaths()
	if err != nil {
		return err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+Label).Run()
	for _, path := range []string{paths.Plist, paths.Binary} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func Status() (string, error) {
	domain := "gui/" + strconv.Itoa(os.Getuid())
	output, err := exec.Command("launchctl", "print", domain+"/"+Label).CombinedOutput()
	if err != nil {
		return "not installed", nil
	}
	text := string(output)
	state := "loaded"
	if strings.Contains(text, "state = running") {
		state = "running"
	}
	return state, nil
}

func renderPlist(paths Paths, pathEnv string) string {
	escape := html.EscapeString
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>` + Label + `</string>
<key>ProgramArguments</key><array><string>` + escape(paths.Binary) + `</string><string>serve</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/><key>ProcessType</key><string>Background</string>
<key>EnvironmentVariables</key><dict><key>PATH</key><string>` + escape(pathEnv) + `</string><key>AI_USAGE_LOG_FILE</key><string>` + escape(paths.Stdout) + `</string></dict>
<key>StandardOutPath</key><string>/dev/null</string><key>StandardErrorPath</key><string>` + escape(paths.Stderr) + `</string>
<key>ThrottleInterval</key><integer>10</integer>
</dict></plist>
`
}
