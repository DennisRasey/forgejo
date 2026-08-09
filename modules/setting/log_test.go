// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forgejo.org/modules/json"
	"forgejo.org/modules/log"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initLoggersByConfig(t *testing.T, config string) (*log.LoggerManager, func()) {
	defer test.MockVariableValue(&Log, LogGlobalConfig{})()

	cfg, err := NewConfigProviderFromData(config)
	require.NoError(t, err)

	manager := log.NewManager()
	initManagedLoggers(manager, cfg)
	return manager, manager.Close
}

func initLoggerConfig(t *testing.T, config string) ConfigProvider {
	defer test.MockVariableValue(&Log, LogGlobalConfig{})()

	cfg, err := NewConfigProviderFromData(config)
	require.NoError(t, err)

	prepareLoggerConfig(cfg)

	return cfg
}

func toJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "\t")
	return string(b)
}

func TestLogConfigDefault(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, ``)
	defer managerClose()

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	}
}
`

	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))
}

func TestLogConfigDisable(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, `
[log]
logger.router.MODE =
logger.xorm.MODE =
`)
	defer managerClose()

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	}
}
`

	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))
}

func TestLogConfigLegacyDefault(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, `
[log]
MODE = console
`)
	defer managerClose()

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	}
}
`

	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))
}

func TestLogConfigLegacyMode(t *testing.T) {
	tempDir := t.TempDir()

	tempPath := func(file string) string {
		return filepath.Join(tempDir, file)
	}

	manager, managerClose := initLoggersByConfig(t, `
[log]
ROOT_PATH = `+tempDir+`
MODE = file
ROUTER = file
ACCESS = file
`)
	defer managerClose()

	writerDump := `
{
	"file": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Compress": true,
			"CompressionLevel": -1,
			"DailyRotate": true,
			"FileName": "$FILENAME",
			"LogRotate": true,
			"MaxDays": 7,
			"MaxSize": 268435456
		},
		"WriterType": "file"
	}
}
`
	writerDumpAccess := `
{
	"file.access": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "none",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Compress": true,
			"CompressionLevel": -1,
			"DailyRotate": true,
			"FileName": "$FILENAME",
			"LogRotate": true,
			"MaxDays": 7,
			"MaxSize": 268435456
		},
		"WriterType": "file"
	}
}
`
	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	require.JSONEq(t, strings.ReplaceAll(writerDump, "$FILENAME", tempPath("forgejo.log")), toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, strings.ReplaceAll(writerDumpAccess, "$FILENAME", tempPath("access.log")), toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, strings.ReplaceAll(writerDump, "$FILENAME", tempPath("forgejo.log")), toJSON(dump))
}

func TestLogConfigLegacyModeDisable(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, `
[log]
ROUTER = file
ACCESS = file
DISABLE_ROUTER_LOG = true
ENABLE_ACCESS_LOG = false
`)
	defer managerClose()

	dump := manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))

	dump = manager.GetLogger("router").DumpWriters()
	require.JSONEq(t, "{}", toJSON(dump))
}

func TestLogConfigNewConfig(t *testing.T) {
	manager, managerClose := initLoggersByConfig(t, `
[log]
LOGGER_ACCESS_MODE = console
LOGGER_XORM_MODE = console, console-1

[log.console]
LEVEL = warn

[log.console-1]
MODE = console
LEVEL = error
STDERR = true
`)
	defer managerClose()

	writerDump := `
{
	"console": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "stdflags",
		"Level": "warn",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	},
	"console-1": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "stdflags",
		"Level": "error",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": true
		},
		"WriterType": "console"
	}
}
`
	writerDumpAccess := `
{
	"console.access": {
		"BufferLen": 10000,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "none",
		"Level": "warn",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Stderr": false
		},
		"WriterType": "console"
	}
}
`
	dump := manager.GetLogger("xorm").DumpWriters()
	require.JSONEq(t, writerDump, toJSON(dump))

	dump = manager.GetLogger("access").DumpWriters()
	require.JSONEq(t, writerDumpAccess, toJSON(dump))
}

func TestLogConfigModeFile(t *testing.T) {
	tempDir := t.TempDir()

	tempPath := func(file string) string {
		return filepath.Join(tempDir, file)
	}

	manager, managerClose := initLoggersByConfig(t, `
[log]
ROOT_PATH = `+tempDir+`
BUFFER_LEN = 10
MODE = file, file1

[log.file1]
MODE = file
LEVEL = error
STACKTRACE_LEVEL = fatal
EXPRESSION = filter
EXCLUSION = not
FLAGS = medfile
PREFIX = "[Prefix] "
FILE_NAME = file-xxx.log
LOG_ROTATE = false
MAX_SIZE_SHIFT = 1
DAILY_ROTATE = false
MAX_DAYS = 90
COMPRESS = false
COMPRESSION_LEVEL = 4
`)
	defer managerClose()

	writerDump := `
{
	"file": {
		"BufferLen": 10,
		"Colorize": false,
		"Expression": "",
		"Exclusion": "",
		"Flags": "stdflags",
		"Level": "info",
		"Prefix": "",
		"StacktraceLevel": "none",
		"WriterOption": {
			"Compress": true,
			"CompressionLevel": -1,
			"DailyRotate": true,
			"FileName": "$FILENAME-0",
			"LogRotate": true,
			"MaxDays": 7,
			"MaxSize": 268435456
		},
		"WriterType": "file"
	},
	"file1": {
		"BufferLen": 10,
		"Colorize": false,
		"Expression": "filter",
		"Exclusion": "not",
		"Flags": "medfile",
		"Level": "error",
		"Prefix": "[Prefix] ",
		"StacktraceLevel": "fatal",
		"WriterOption": {
			"Compress": false,
			"CompressionLevel": 4,
			"DailyRotate": false,
			"FileName": "$FILENAME-1",
			"LogRotate": false,
			"MaxDays": 90,
			"MaxSize": 2
		},
		"WriterType": "file"
	}
}
`

	dump := manager.GetLogger(log.DEFAULT).DumpWriters()
	expected := writerDump
	expected = strings.ReplaceAll(expected, "$FILENAME-0", tempPath("forgejo.log"))
	expected = strings.ReplaceAll(expected, "$FILENAME-1", tempPath("file-xxx.log"))
	require.JSONEq(t, expected, toJSON(dump))
}

func TestLogPrepareFilenameForWriter(t *testing.T) {
	t.Run("forgejo.log is the default and a backward compatible symlink is created for gitea.log", func(t *testing.T) {
		d := t.TempDir()
		defer test.MockVariableValue(&Log.RootPath, d)()
		logFileBackward := filepath.Join(d, logBackwardCompatibleFileName)
		logFile := filepath.Join(d, logDefaultFileName)
		assert.NoFileExists(t, logFileBackward)
		assert.Equal(t, logFile, LogPrepareFilenameForWriter("", logDefaultFileName))
		assert.FileExists(t, logFileBackward)
		// it already exists so it does nothing
		assert.Equal(t, logFile, LogPrepareFilenameForWriter("", logDefaultFileName))
		assert.FileExists(t, logFileBackward)
	})

	t.Run("lstat on the gitea.log file panics", func(t *testing.T) {
		d := t.TempDir()
		defer test.MockVariableValue(&Log.RootPath, d)()
		defer test.MockVariableValue(&logBackwardCompatibleFileName, "\000")()
		test.PanicErrorContains(t, func() { LogPrepareFilenameForWriter("", logDefaultFileName) }, "os.Lstat")
	})

	t.Run("forgejo.log is not the default if gitea.log already exists", func(t *testing.T) {
		t.Run("as a regular file", func(t *testing.T) {
			d := t.TempDir()
			defer test.MockVariableValue(&Log.RootPath, d)()
			logFileBackward := filepath.Join(d, logBackwardCompatibleFileName)
			logFile := filepath.Join(d, logDefaultFileName)
			require.NoError(t, os.WriteFile(logFileBackward, []byte("logline"), 0o666))
			assert.NoFileExists(t, logFile)
			assert.Equal(t, logFileBackward, LogPrepareFilenameForWriter("", logDefaultFileName))
			assert.NoFileExists(t, logFile)
		})
		t.Run("as a symlink to a different file", func(t *testing.T) {
			d := t.TempDir()
			defer test.MockVariableValue(&Log.RootPath, d)()
			logFileBackward := filepath.Join(d, logBackwardCompatibleFileName)
			logFile := filepath.Join(d, logDefaultFileName)
			require.NoError(t, os.Symlink("/somewhere/else", logFileBackward))
			assert.NoFileExists(t, logFile)
			assert.Equal(t, logFileBackward, LogPrepareFilenameForWriter("", logDefaultFileName))
			assert.NoFileExists(t, logFile)
		})
	})

	t.Run("absolute filename", func(t *testing.T) {
		d := t.TempDir()
		defer test.MockVariableValue(&Log.RootPath, d)()
		expected := filepath.Join(d, "my.log")
		assert.Equal(t, expected, LogPrepareFilenameForWriter(expected, logDefaultFileName))
		assert.NoFileExists(t, filepath.Join(d, logBackwardCompatibleFileName))
	})

	t.Run("relative filename", func(t *testing.T) {
		d := t.TempDir()
		defer test.MockVariableValue(&Log.RootPath, d)()
		fileName := "my.log"
		assert.Equal(t, filepath.Join(d, fileName), LogPrepareFilenameForWriter(fileName, logDefaultFileName))
		assert.NoFileExists(t, filepath.Join(d, logBackwardCompatibleFileName))
	})

	t.Run("unclean absolute filename", func(t *testing.T) {
		d := t.TempDir()
		defer test.MockVariableValue(&Log.RootPath, d)()
		expected := filepath.Join(d, "my.log")
		unclean := filepath.Join(d, "..", filepath.Base(d), "my.log")
		assert.Equal(t, expected, LogPrepareFilenameForWriter(unclean, logDefaultFileName))
		assert.NoFileExists(t, filepath.Join(d, logBackwardCompatibleFileName))
	})

	t.Run("mkdir on the root path for logs panics", func(t *testing.T) {
		d := t.TempDir()
		logdir := filepath.Join(d, "logdir")
		require.NoError(t, os.MkdirAll(logdir, 0o000))
		rootPath := filepath.Join(logdir, "rootpath")
		defer test.MockVariableValue(&Log.RootPath, rootPath)()
		test.PanicErrorContains(t, func() { LogPrepareFilenameForWriter("", logDefaultFileName) }, "unable to create directory")
	})

	t.Run("gitea.log is explicitly set in the config file", func(t *testing.T) {
		d := t.TempDir()
		defer test.MockVariableValue(&Log.RootPath, d)()
		giteaLog := filepath.Join(d, "dir/gitea.log")
		assert.Equal(t, giteaLog, LogPrepareFilenameForWriter(giteaLog, logDefaultFileName))
	})

	t.Run("no backward compatibility effort if the default is not forgejo.log", func(t *testing.T) {
		d := t.TempDir()
		defer test.MockVariableValue(&Log.RootPath, d)()
		alternateDefaultFileName := "access.log"
		assert.Equal(t, filepath.Join(d, alternateDefaultFileName), LogPrepareFilenameForWriter("", alternateDefaultFileName))
	})
}

func TestLegacyLoggerMigrations(t *testing.T) {
	type Cases = []struct {
		name string
		cfg  string
		exp  string
	}

	runCases := func(t *testing.T, key string, cases Cases) {
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				cfg := initLoggerConfig(t, c.cfg)
				require.Equal(t, c.exp, cfg.Section("log").Key(key).String())
			})
		}
	}

	t.Run("default", func(t *testing.T) {
		runCases(t, "LOGGER_DEFAULT_MODE", Cases{
			{
				"uses default value for default logger",
				"",
				",",
			},
			{
				"uses logger.default.MODE for default logger",
				`[log]
logger.default.MODE = file
`,
				"file",
			},
		})
	})

	t.Run("access", func(t *testing.T) {
		runCases(t, "LOGGER_ACCESS_MODE", Cases{
			{
				"uses default value for access logger",
				"",
				"",
			},
			{
				"uses ACCESS for access logger",
				`[log]
ACCESS = file
`,
				"file",
			},
			{
				"ENABLE_ACCESS_LOG=true doesn't change access logger",
				`[log]
ENABLE_ACCESS_LOG = true
logger.access.MODE = console
`,
				"console",
			},
			{
				"ENABLE_ACCESS_LOG=false disables access logger",
				`[log]
ENABLE_ACCESS_LOG = false
logger.access.MODE = console
`,
				"",
			},
			{
				"logger.access.MODE has precedence over ACCESS for access logger",
				`[log]
ACCESS = file
logger.access.MODE = console
`,
				"console",
			},
			{
				"LOGGER_ACCESS_MODE has precedence over logger.access.MODE for access logger",
				`[log]
LOGGER_ACCESS_MODE = file
logger.access.MODE = console
`,
				"file",
			},
			{
				"ENABLE_ACCESS_LOG doesn't enable access logger",
				`[log]
ENABLE_ACCESS_LOG = true
`,
				"", // should be `,`
			},
		})
	})

	t.Run("router", func(t *testing.T) {
		runCases(t, "LOGGER_ROUTER_MODE", Cases{
			{
				"uses default value for router logger",
				"",
				",",
			},
			{
				"uses ROUTER for router logger",
				`[log]
ROUTER = file
`,
				"file",
			},
			{
				"DISABLE_ROUTER_LOG=false doesn't change router logger",
				`[log]
ROUTER = file
DISABLE_ROUTER_LOG = false
`,
				"file",
			},
			{
				"DISABLE_ROUTER_LOG=true disables router logger",
				`[log]
DISABLE_ROUTER_LOG = true
logger.router.MODE = console
`,
				"",
			},
			{
				"logger.router.MODE as precedence over ROUTER for router logger",
				`[log]
ROUTER = file
logger.router.MODE = console
`,
				"console",
			},
			{
				"LOGGER_ROUTER_MODE has precedence over logger.router.MODE for router logger",
				`[log]
LOGGER_ROUTER_MODE = file
logger.router.MODE = console
`,
				"file",
			},
		})
	})

	t.Run("xorm", func(t *testing.T) {
		runCases(t, "LOGGER_XORM_MODE", Cases{
			{
				"uses default value for xorm logger",
				"",
				",",
			},
			{
				"uses XORM for xorm logger",
				`[log]
XORM = file
`,
				"file",
			},
			{
				"ENABLE_XORM_LOG=true doesn't change xorm logger",
				`[log]
ENABLE_XORM_LOG = true
logger.xorm.MODE = console
`,
				"console",
			},
			{
				"ENABLE_XORM_LOG=false disables xorm logger",
				`[log]
ENABLE_XORM_LOG = false
logger.xorm.MODE = console
`,
				"",
			},
			{
				"logger.xorm.MODE has precedence over XORM for xorm logger",
				`[log]
XORM = file
logger.xorm.MODE = console
`,
				"console",
			},
			{
				"LOGGER_XORM_MODE has precedence over logger.xorm.MODE for xorm logger",
				`[log]
LOGGER_XORM_MODE = file
logger.xorm.MODE = console
`,
				"file",
			},
		})
	})

	t.Run("ssh", func(t *testing.T) {
		runCases(t, "LOGGER_SSH_MODE", Cases{
			{
				"uses default value for ssh logger",
				"",
				"",
			},
			{
				"deprecated config can enable logger",
				`[log]
ENABLE_SSH_LOG = true
`,
				",",
			},
			{
				"check priority",
				`[log]
LOGGER_SSH_MODE = file
ENABLE_SSH_LOG = true
`,
				"file",
			},
		})
	})
}
