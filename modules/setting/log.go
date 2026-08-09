// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	golog "log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"forgejo.org/modules/log"
	"forgejo.org/modules/util"
)

type LogGlobalConfig struct {
	RootPath string

	Mode               string
	Level              log.Level
	StacktraceLogLevel log.Level
	BufferLen          int

	AccessLogTemplate string
	RequestIDHeaders  []string
}

var Log LogGlobalConfig

const accessLogTemplateDefault = `{{.Ctx.RemoteHost}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.URL.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}" "{{.Ctx.Req.UserAgent}}"`

func loadLogGlobalFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("log")

	Log.Level = log.LevelFromString(sec.Key("LEVEL").MustString(log.INFO.String()))
	Log.StacktraceLogLevel = log.LevelFromString(sec.Key("STACKTRACE_LEVEL").MustString(log.NONE.String()))
	Log.BufferLen = sec.Key("BUFFER_LEN").MustInt(10000)
	Log.Mode = sec.Key("MODE").MustString("console")

	Log.RootPath = sec.Key("ROOT_PATH").MustString(path.Join(AppWorkPath, "log"))
	if !filepath.IsAbs(Log.RootPath) {
		Log.RootPath = filepath.Join(AppWorkPath, Log.RootPath)
	}
	Log.RootPath = util.FilePathJoinAbs(Log.RootPath)

	Log.AccessLogTemplate = sec.Key("ACCESS_LOG_TEMPLATE").MustString(accessLogTemplateDefault)
	Log.RequestIDHeaders = sec.Key("REQUEST_ID_HEADERS").Strings(",")
}

func prepareLoggerConfig(rootCfg ConfigProvider) {
	sec := rootCfg.Section("log")

	// Priority: `LOGGER_DEFAULT_MODE` -> `logger.default.MODE`
	deprecatedSettingWarning(rootCfg, "log", "logger.default.MODE", "log", "LOGGER_DEFAULT_MODE")
	hasNoValue := !sec.HasKey("LOGGER_DEFAULT_MODE")
	if hasNoValue && sec.HasKey("logger.default.MODE") {
		sec.Key("LOGGER_DEFAULT_MODE").SetValue(sec.Key("logger.default.MODE").String())
		hasNoValue = false
	}
	if hasNoValue {
		sec.Key("LOGGER_DEFAULT_MODE").SetValue(",") // use default logger
	}

	// Priority: `ENABLE_ACCESS_LOG` -> `LOGGER_ACCESS_MODE` -> `logger.access.MODE` -> `ACCESS`
	deprecatedSettingWarning(rootCfg, "log", "ACCESS", "log", "LOGGER_ACCESS_MODE")
	deprecatedSettingWarning(rootCfg, "log", "ENABLE_ACCESS_LOG", "log", "LOGGER_ACCESS_MODE")
	deprecatedSettingWarning(rootCfg, "log", "logger.access.MODE", "log", "LOGGER_ACCESS_MODE")
	hasNoValue = !sec.HasKey("LOGGER_ACCESS_MODE")
	if hasNoValue && sec.HasKey("logger.access.MODE") {
		sec.Key("LOGGER_ACCESS_MODE").SetValue(sec.Key("logger.access.MODE").String())
		hasNoValue = false
	}
	if val := sec.Key("ACCESS").String(); hasNoValue && val != "" {
		sec.Key("LOGGER_ACCESS_MODE").SetValue(val)
	}
	if sec.HasKey("ENABLE_ACCESS_LOG") && !sec.Key("ENABLE_ACCESS_LOG").MustBool() {
		sec.Key("LOGGER_ACCESS_MODE").SetValue("")
	}

	// Priority: `DISABLE_ROUTER_LOG` -> `LOGGER_ROUTER_MODE` -> `logger.router.MODE` -> `ROUTER`
	deprecatedSettingWarning(rootCfg, "log", "ROUTER", "log", "LOGGER_ROUTER_MODE")
	deprecatedSettingWarning(rootCfg, "log", "DISABLE_ROUTER_LOG", "log", "LOGGER_ROUTER_MODE")
	deprecatedSettingWarning(rootCfg, "log", "logger.router.MODE", "log", "LOGGER_ROUTER_MODE")
	hasNoValue = !sec.HasKey("LOGGER_ROUTER_MODE")
	if hasNoValue && sec.HasKey("logger.router.MODE") {
		sec.Key("LOGGER_ROUTER_MODE").SetValue(sec.Key("logger.router.MODE").String())
		hasNoValue = false
	}
	if val := sec.Key("ROUTER").String(); hasNoValue && val != "" {
		sec.Key("LOGGER_ROUTER_MODE").SetValue(val)
		hasNoValue = false
	}
	if sec.HasKey("DISABLE_ROUTER_LOG") && sec.Key("DISABLE_ROUTER_LOG").MustBool() {
		sec.Key("LOGGER_ROUTER_MODE").SetValue("")
		hasNoValue = false
	}
	if hasNoValue {
		sec.Key("LOGGER_ROUTER_MODE").SetValue(",") // use default logger
	}

	// Priority: `ENABLE_XORM_LOG` -> `LOGGER_XORM_MODE` -> `logger.xorm.MODE` -> `XORM`
	deprecatedSettingWarning(rootCfg, "log", "XORM", "log", "LOGGER_XORM_MODE")
	deprecatedSettingWarning(rootCfg, "log", "ENABLE_XORM_LOG", "log", "LOGGER_XORM_MODE")
	deprecatedSettingWarning(rootCfg, "log", "logger.xorm.MODE", "log", "LOGGER_XORM_MODE")
	hasNoValue = !sec.HasKey("LOGGER_XORM_MODE")
	if hasNoValue && sec.HasKey("logger.xorm.MODE") {
		sec.Key("LOGGER_XORM_MODE").SetValue(sec.Key("logger.xorm.MODE").String())
		hasNoValue = false
	}
	if val := sec.Key("XORM").String(); hasNoValue && val != "" {
		sec.Key("LOGGER_XORM_MODE").SetValue(val)
		hasNoValue = false
	}
	if sec.HasKey("ENABLE_XORM_LOG") && !sec.Key("ENABLE_XORM_LOG").MustBool() {
		sec.Key("LOGGER_XORM_MODE").SetValue("")
		hasNoValue = false
	}
	if hasNoValue {
		sec.Key("LOGGER_XORM_MODE").SetValue(",") // use default logger
	}

	// Priority: `LOGGER_SSH_MODE` -> `ENABLE_SSH_LOG`
	deprecatedSettingWarning(rootCfg, "log", "ENABLE_SSH_LOG", "log", "LOGGER_SSH_MODE")
	if !sec.HasKey("LOGGER_SSH_MODE") && sec.HasKey("ENABLE_SSH_LOG") {
		if sec.Key("ENABLE_SSH_LOG").MustBool() {
			sec.Key("LOGGER_SSH_MODE").SetValue(",") // use default logger
		} else {
			sec.Key("LOGGER_SSH_MODE").SetValue("") // disable ssh logger
		}
	}
}

var (
	logDefaultFileName            = "forgejo.log"
	logBackwardCompatibleFileName = "gitea.log"
)

func logBackwardCompatibleDefaultToForgejo(fileName, configFileName string) string {
	// do not default to forgejo.log if the log file is explicitly
	// specified in the config file
	if configFileName != "" {
		return fileName
	}
	backwardCompatibleFileName := filepath.Join(filepath.Dir(fileName), logBackwardCompatibleFileName)
	stat, err := os.Lstat(backwardCompatibleFileName)
	if err != nil && !os.IsNotExist(err) {
		panic(fmt.Sprintf("os.Lstat %q: %v", backwardCompatibleFileName, err.Error()))
	}
	// gitea.log does not exist although the config file does not
	// specify a default let's use forgejo.log instead and create a
	// gitea.log symlink for backward compatibility
	if os.IsNotExist(err) {
		if err := os.Symlink(fileName, backwardCompatibleFileName); err != nil {
			panic(fmt.Sprintf("os.Symlink %q to %q: %v", fileName, backwardCompatibleFileName, err.Error()))
		}
		return fileName
	}
	// gitea.log was not created by this function because it is not a
	// symlink, use gitea.log instead of forgejo.log
	if stat.Mode()&os.ModeSymlink == 0 {
		return backwardCompatibleFileName
	}
	destination, err := os.Readlink(backwardCompatibleFileName)
	if err != nil {
		panic(fmt.Sprintf("os.Readlink %q: %v", backwardCompatibleFileName, err))
	}
	// the file was not created by this function because it does not
	// point to the expected path, default to gitea.log instead of
	// forgejo.log
	if destination != fileName {
		return backwardCompatibleFileName
	}
	// the symlink is as expected, let's use forgejo.log as a default
	return fileName
}

func LogPrepareFilenameForWriter(configFileName, defaultFileName string) string {
	fileName := configFileName
	if fileName == "" {
		fileName = defaultFileName
	}
	if !filepath.IsAbs(fileName) {
		fileName = filepath.Join(Log.RootPath, fileName)
	} else {
		fileName = filepath.Clean(fileName)
	}
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		panic(fmt.Sprintf("unable to create directory for log %q: %v", fileName, err.Error()))
	}
	// only relevant for the forgejo.log default (not when the default is
	// access.log for instance)
	if defaultFileName == logDefaultFileName {
		fileName = logBackwardCompatibleDefaultToForgejo(fileName, configFileName)
	}
	return fileName
}

func loadLogModeByName(rootCfg ConfigProvider, loggerName, modeName string) (writerName, writerType string, writerMode log.WriterMode, err error) {
	sec := rootCfg.Section("log." + modeName)

	writerMode = log.WriterMode{}
	writerType = ConfigSectionKeyString(sec, "MODE")
	if writerType == "" {
		writerType = modeName
	}

	writerName = modeName
	defaultFlags := "stdflags"
	defaultFileName := logDefaultFileName
	if loggerName == "access" {
		// "access" logger is special, by default it doesn't have output flags, so it also needs a new writer name to avoid conflicting with other writers.
		// so "access" logger's writer name is usually "file.access" or "console.access"
		writerName += ".access"
		defaultFlags = "none"
		defaultFileName = "access.log"
	}

	writerMode.Level = log.LevelFromString(ConfigInheritedKeyString(sec, "LEVEL", Log.Level.String()))
	writerMode.StacktraceLevel = log.LevelFromString(ConfigInheritedKeyString(sec, "STACKTRACE_LEVEL", Log.StacktraceLogLevel.String()))
	writerMode.Prefix = ConfigInheritedKeyString(sec, "PREFIX")
	writerMode.Expression = ConfigInheritedKeyString(sec, "EXPRESSION")
	writerMode.Exclusion = ConfigInheritedKeyString(sec, "EXCLUSION")
	// flags are updated and set below

	switch writerType {
	case "console":
		// if stderr is on journald, prefer stderr by default
		useStderr := ConfigInheritedKey(sec, "STDERR").MustBool(log.JournaldOnStderr)
		defaultCanColor := log.CanColorStdout
		defaultJournald := log.JournaldOnStdout
		if useStderr {
			defaultCanColor = log.CanColorStderr
			defaultJournald = log.JournaldOnStderr
		}
		writerOption := log.WriterConsoleOption{Stderr: useStderr}
		writerMode.Colorize = ConfigInheritedKey(sec, "COLORIZE").MustBool(defaultCanColor)
		writerMode.WriterOption = writerOption
		// if we are ultimately on journald, update default flags
		if defaultJournald {
			defaultFlags = "journaldflags"
		}
	case "file":
		fileName := LogPrepareFilenameForWriter(ConfigInheritedKey(sec, "FILE_NAME").String(), defaultFileName)
		writerOption := log.WriterFileOption{}
		writerOption.FileName = fileName + filenameSuffix // FIXME: the suffix doesn't seem right, see its related comments
		writerOption.LogRotate = ConfigInheritedKey(sec, "LOG_ROTATE").MustBool(true)
		writerOption.MaxSize = 1 << uint(ConfigInheritedKey(sec, "MAX_SIZE_SHIFT").MustInt(28))
		writerOption.DailyRotate = ConfigInheritedKey(sec, "DAILY_ROTATE").MustBool(true)
		writerOption.MaxDays = ConfigInheritedKey(sec, "MAX_DAYS").MustInt(7)
		writerOption.Compress = ConfigInheritedKey(sec, "COMPRESS").MustBool(true)
		writerOption.CompressionLevel = ConfigInheritedKey(sec, "COMPRESSION_LEVEL").MustInt(-1)
		writerMode.WriterOption = writerOption
	case "conn":
		writerOption := log.WriterConnOption{}
		writerOption.ReconnectOnMsg = ConfigInheritedKey(sec, "RECONNECT_ON_MSG").MustBool()
		writerOption.Reconnect = ConfigInheritedKey(sec, "RECONNECT").MustBool()
		writerOption.Protocol = ConfigInheritedKey(sec, "PROTOCOL").In("tcp", []string{"tcp", "unix", "udp"})
		writerOption.Addr = ConfigInheritedKey(sec, "ADDR").MustString(":7020")
		writerMode.WriterOption = writerOption
	default:
		if !log.HasEventWriter(writerType) {
			return "", "", writerMode, fmt.Errorf("invalid log writer type (mode): %s, maybe it needs something like 'MODE=file' in [log.%s] section", writerType, modeName)
		}
	}

	// set flags last because the console writer code may update default flags
	writerMode.Flags = log.FlagsFromString(ConfigInheritedKeyString(sec, "FLAGS", defaultFlags))

	return writerName, writerType, writerMode, nil
}

var filenameSuffix = ""

// RestartLogsWithPIDSuffix restarts the logs with a PID suffix on files
// FIXME: it seems not right, it breaks log rotating or log collectors
func RestartLogsWithPIDSuffix() {
	filenameSuffix = fmt.Sprintf(".%d", os.Getpid())
	initAllLoggers() // when forking, before restarting, rename logger file and re-init all loggers
}

func InitLoggersForTest() {
	initAllLoggers()
}

// initAllLoggers creates all the log services
func initAllLoggers() {
	initManagedLoggers(log.GetManager(), CfgProvider)

	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.LoggerToWriter(log.GetLogger(log.DEFAULT).Info))
}

func initManagedLoggers(manager *log.LoggerManager, cfg ConfigProvider) {
	loadLogGlobalFrom(cfg)
	prepareLoggerConfig(cfg)

	initLoggerByName(manager, cfg, log.DEFAULT) // default
	initLoggerByName(manager, cfg, "access")
	initLoggerByName(manager, cfg, "router")
	initLoggerByName(manager, cfg, "xorm")
	initLoggerByName(manager, cfg, "ssh")
}

func initLoggerByName(manager *log.LoggerManager, rootCfg ConfigProvider, loggerName string) {
	sec := rootCfg.Section("log")
	key := "LOGGER_" + strings.ToUpper(loggerName) + "_MODE"

	disabled := sec.HasKey(key) && sec.Key(key).String() == ""
	if disabled {
		return
	}

	modeVal := sec.Key(key).String()
	if modeVal == "," {
		modeVal = Log.Mode
	}

	var eventWriters []log.EventWriter
	modes := strings.SplitSeq(modeVal, ",")
	for modeName := range modes {
		modeName = strings.TrimSpace(modeName)
		if modeName == "" {
			continue
		}
		writerName, writerType, writerMode, err := loadLogModeByName(rootCfg, loggerName, modeName)
		if err != nil {
			log.FallbackErrorf("Failed to load writer mode %q for logger %s: %v", modeName, loggerName, err)
			continue
		}
		if writerMode.BufferLen == 0 {
			writerMode.BufferLen = Log.BufferLen
		}
		eventWriter := manager.GetSharedWriter(writerName)
		if eventWriter == nil {
			eventWriter, err = manager.NewSharedWriter(writerName, writerType, writerMode)
			if err != nil {
				log.FallbackErrorf("Failed to create event writer for logger %s: %v", loggerName, err)
				continue
			}
		}
		eventWriters = append(eventWriters, eventWriter)
	}

	manager.GetLogger(loggerName).ReplaceAllWriters(eventWriters...)
}

func InitSQLLoggersForCli(level log.Level) {
	log.SetConsoleLogger("xorm", "console", level)
}

func IsAccessLogEnabled() bool {
	return log.IsLoggerEnabled("access")
}

func IsRouteLogEnabled() bool {
	return log.IsLoggerEnabled("router")
}
