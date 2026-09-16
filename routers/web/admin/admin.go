// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"time"

	activities_model "forgejo.org/models/activities"
	"forgejo.org/models/db"
	"forgejo.org/modules/base"
	"forgejo.org/modules/cache"
	"forgejo.org/modules/graceful"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/updatechecker"
	"forgejo.org/modules/web"
	"forgejo.org/services/context"
	"forgejo.org/services/cron"
	"forgejo.org/services/forms"
	release_service "forgejo.org/services/release"
	repo_service "forgejo.org/services/repository"
)

const (
	tplDashboard    base.TplName = "admin/dashboard"
	tplSystemStatus base.TplName = "admin/system_status"
	tplSelfCheck    base.TplName = "admin/self_check"
	tplCron         base.TplName = "admin/cron"
	tplQueue        base.TplName = "admin/queue"
	tplStacktrace   base.TplName = "admin/stacktrace"
	tplQueueManage  base.TplName = "admin/queue_manage"
	tplStats        base.TplName = "admin/stats"
)

var sysStatus struct {
	StartTime    string
	NumGoroutine int

	// General statistics.
	MemAllocated int64  // bytes allocated and still in use
	MemTotal     int64  // bytes allocated (even if freed)
	MemSys       int64  // bytes obtained from system (sum of XxxSys below)
	Lookups      uint64 // number of pointer lookups
	MemMallocs   uint64 // number of mallocs
	MemFrees     uint64 // number of frees

	// Main allocation heap statistics.
	HeapAlloc    int64  // bytes allocated and still in use
	HeapSys      int64  // bytes obtained from system
	HeapIdle     int64  // bytes in idle spans
	HeapInuse    int64  // bytes in non-idle span
	HeapReleased int64  // bytes released to the OS
	HeapObjects  uint64 // total number of allocated objects

	// Low-level fixed-size structure allocator statistics.
	//	Inuse is bytes used now.
	//	Sys is bytes obtained from system.
	StackInuse  int64 // bootstrap stacks
	StackSys    int64
	MSpanInuse  int64 // mspan structures
	MSpanSys    int64
	MCacheInuse int64 // mcache structures
	MCacheSys   int64
	BuckHashSys int64 // profiling bucket hash table
	GCSys       int64 // GC metadata
	OtherSys    int64 // other system allocations

	// Garbage collector statistics.
	NextGC       int64  // next run in HeapAlloc time (bytes)
	LastGCTime   string // last run time
	PauseTotalNs string
	PauseNs      string // circular buffer of recent GC pause times, most recent at [(NumGC+255)%256]
	NumGC        uint32
}

func updateSystemStatus() {
	sysStatus.StartTime = setting.AppStartTime.Format(time.RFC3339)

	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)
	sysStatus.NumGoroutine = runtime.NumGoroutine()

	sysStatus.MemAllocated = int64(m.Alloc)
	sysStatus.MemTotal = int64(m.TotalAlloc)
	sysStatus.MemSys = int64(m.Sys)
	sysStatus.Lookups = m.Lookups
	sysStatus.MemMallocs = m.Mallocs
	sysStatus.MemFrees = m.Frees

	sysStatus.HeapAlloc = int64(m.HeapAlloc)
	sysStatus.HeapSys = int64(m.HeapSys)
	sysStatus.HeapIdle = int64(m.HeapIdle)
	sysStatus.HeapInuse = int64(m.HeapInuse)
	sysStatus.HeapReleased = int64(m.HeapReleased)
	sysStatus.HeapObjects = m.HeapObjects

	sysStatus.StackInuse = int64(m.StackInuse)
	sysStatus.StackSys = int64(m.StackSys)
	sysStatus.MSpanInuse = int64(m.MSpanInuse)
	sysStatus.MSpanSys = int64(m.MSpanSys)
	sysStatus.MCacheInuse = int64(m.MCacheInuse)
	sysStatus.MCacheSys = int64(m.MCacheSys)
	sysStatus.BuckHashSys = int64(m.BuckHashSys)
	sysStatus.GCSys = int64(m.GCSys)
	sysStatus.OtherSys = int64(m.OtherSys)

	sysStatus.NextGC = int64(m.NextGC)
	sysStatus.LastGCTime = time.Unix(0, int64(m.LastGC)).Format(time.RFC3339)
	sysStatus.PauseTotalNs = fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/1000/1000/1000)
	sysStatus.PauseNs = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC+255)%256])/1000/1000/1000)
	sysStatus.NumGC = m.NumGC
}

func prepareDeprecatedWarningsAlert(ctx *context.Context) {
	if len(setting.DeprecatedWarnings) > 0 {
		content := setting.DeprecatedWarnings[0]
		if len(setting.DeprecatedWarnings) > 1 {
			content += fmt.Sprintf(" (and %d more)", len(setting.DeprecatedWarnings)-1)
		}
		ctx.Flash.Error(content, true)
	}
}

// Dashboard show admin panel dashboard
func Dashboard(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdminDashboard"] = true
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus

	update, err := updatechecker.GetReleaseState(ctx)
	if err != nil {
		ctx.Data["UpdateCheckerError"] = err.Error()
	} else {
		if update.MajorReleaseState == updatechecker.MajorReleaseUnsupported {
			ctx.Data["NeedMajorUpdate"] = true
		} else if update.MinorReleaseState == updatechecker.MinorReleaseOutOfDate {
			ctx.Data["NeedMinorUpdate"] = true
		}
		if hasRec, rec := update.RecommendedUpgrade.Get(); hasRec {
			ctx.Data["RemoteVersion"] = rec
		}
	}

	entries := []string{
		"delete_inactive_accounts",
		"delete_repo_archives",
		"delete_missing_repos",
		"git_gc_repos",
	}
	if !setting.SSH.Disabled && !setting.SSH.StartBuiltinServer {
		entries = append(entries, "resync_all_sshkeys", "resync_all_sshprincipals")
	}
	entries = append(entries, []string{
		"resync_all_hooks",
		"reinit_missing_repos",
		"sync_external_users",
		"repo_health_check",
		"delete_generated_repository_avatars",
		"sync_repo_branches",
		"sync_repo_tags",
	}...)
	ctx.Data["Entries"] = entries

	prepareDeprecatedWarningsAlert(ctx)
	ctx.HTML(http.StatusOK, tplDashboard)
}

func SystemStatus(ctx *context.Context) {
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus
	ctx.HTML(http.StatusOK, tplSystemStatus)
}

// DashboardPost run an admin operation
func DashboardPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminDashboardForm)
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdminDashboard"] = true
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus

	// Run operation.
	if form.Op != "" {
		switch form.Op {
		case "sync_repo_branches":
			go func() {
				if err := repo_service.AddAllRepoBranchesToSyncQueue(graceful.GetManager().ShutdownContext()); err != nil {
					log.Error("AddAllRepoBranchesToSyncQueue: %v: %v", ctx.Doer.ID, err)
				}
			}()
			ctx.Flash.Success(ctx.Tr("admin.dashboard.sync_branch.started"))
		case "sync_repo_tags":
			go func() {
				if err := release_service.AddAllRepoTagsToSyncQueue(graceful.GetManager().ShutdownContext()); err != nil {
					log.Error("AddAllRepoTagsToSyncQueue: %v: %v", ctx.Doer.ID, err)
				}
			}()
			ctx.Flash.Success(ctx.Tr("admin.dashboard.sync_tag.started"))
		default:
			task := cron.GetTask(form.Op)
			if task != nil {
				go task.RunWithUser(ctx.Doer, nil)
				ctx.Flash.Success(ctx.Tr("admin.dashboard.task.started", ctx.Tr("admin.dashboard."+form.Op)))
			} else {
				ctx.Flash.Error(ctx.Tr("admin.dashboard.task.unknown", form.Op))
			}
		}
	}
	if form.From == "monitor" {
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/cron")
	} else {
		ctx.Redirect(setting.AppSubURL + "/admin")
	}
}

func SelfCheck(ctx *context.Context) {
	ctx.Data["PageIsAdminSelfCheck"] = true
	r, err := db.CheckCollationsDefaultEngine()
	if err != nil {
		ctx.Flash.Error(fmt.Sprintf("CheckCollationsDefaultEngine: %v", err), true)
	}

	if r != nil {
		ctx.Data["DatabaseType"] = setting.Database.Type
		ctx.Data["DatabaseCheckResult"] = r
		hasProblem := false
		if !r.CollationEquals(r.DatabaseCollation, r.ExpectedCollation) {
			ctx.Data["DatabaseCheckCollationMismatch"] = true
			hasProblem = true
		}
		if !r.IsCollationCaseSensitive(r.DatabaseCollation) {
			ctx.Data["DatabaseCheckCollationCaseInsensitive"] = true
			hasProblem = true
		}
		ctx.Data["DatabaseCheckInconsistentCollationColumns"] = r.InconsistentCollationColumns
		hasProblem = hasProblem || len(r.InconsistentCollationColumns) > 0

		ctx.Data["DatabaseCheckHasProblems"] = hasProblem
	}

	_, err = cache.Test()
	if err != nil {
		ctx.Data["CacheError"] = err
	}

	ctx.HTML(http.StatusOK, tplSelfCheck)
}

func CronTasks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.cron")
	ctx.Data["PageIsAdminMonitorCron"] = true
	ctx.Data["Entries"] = cron.ListTasks()
	ctx.HTML(http.StatusOK, tplCron)
}

func MonitorStats(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.stats")
	ctx.Data["PageIsAdminMonitorStats"] = true
	modelStats := activities_model.GetStatistic(ctx).Counter
	stats := map[string]any{}

	// To avoid manually converting the values of the stats struct to an map,
	// and to avoid using JSON to do this for us (JSON encoder converts numbers to
	// scientific notation). Use reflect to convert the struct to an map.
	rv := reflect.ValueOf(modelStats)
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		// Preserve old behavior, do not show arrays that are empty.
		if field.Kind() == reflect.Slice && field.Len() == 0 {
			continue
		}
		stats[rv.Type().Field(i).Name] = field.Interface()
	}

	ctx.Data["Stats"] = stats
	ctx.HTML(http.StatusOK, tplStats)
}
