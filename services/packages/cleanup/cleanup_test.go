package container

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"forgejo.org/models/db"
	"forgejo.org/models/packages"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ownerID = 2001

type testPackageCleanupRule struct {
	Type                   packages.Type
	KeepCount              int
	KeepLastDownloadDays   int
	RemoveDays             int
	RemoveLastDownloadDays int
}

func TestGetCleanupTargets(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := db.DefaultContext

	createPackageCleanupRule := func(t *testing.T, packageCleanupRule testPackageCleanupRule) *packages.PackageCleanupRule {
		t.Helper()

		pcr, err := packages.InsertCleanupRule(ctx, &packages.PackageCleanupRule{
			Enabled:                true,
			OwnerID:                ownerID,
			Type:                   packageCleanupRule.Type,
			KeepCount:              packageCleanupRule.KeepCount,
			KeepLastDownloadDays:   packageCleanupRule.KeepLastDownloadDays,
			RemoveDays:             packageCleanupRule.RemoveDays,
			RemoveLastDownloadDays: packageCleanupRule.RemoveLastDownloadDays,
		})
		require.NoError(t, err)
		return pcr
	}

	createContainerVersions := func(t *testing.T, name string, count, daysBetweenPackages int) *packages.Package {
		t.Helper()

		p, err := packages.TryInsertPackage(ctx, &packages.Package{
			OwnerID:   ownerID,
			Name:      name,
			LowerName: name,
			Type:      packages.TypeContainer,
		})
		require.NoError(t, err)

		for i := range count {
			version := fmt.Sprintf("0.%d.0", i+1)
			created := time.Now().
				Add(time.Hour * -1).
				Add(time.Duration((count-i)*daysBetweenPackages) * time.Hour * 24 * -1).
				Unix()

			// Create the package version for the amd64 variant of a multi-platform OCI image
			platformAmd64Hash := sha256.New()
			platformAmd64Hash.Write([]byte(version + "amd64"))
			platformAmd64Version := "sha256:" + hex.EncodeToString(platformAmd64Hash.Sum(nil))
			platformAmd64PackageVersion := packages.PackageVersion{
				PackageID:    p.ID,
				Version:      platformAmd64Version,
				LowerVersion: platformAmd64Version,
				CreatedUnix:  timeutil.TimeStamp(created),
			}
			_, err = db.GetEngine(ctx).NoAutoTime().Insert(&platformAmd64PackageVersion)
			require.NoError(t, err)

			// Create the package version for the arm64 variant of a multi-platform OCI image
			platformArm64Hash := sha256.New()
			platformArm64Hash.Write([]byte(version + "arm64"))
			platformArm64Version := "sha256:" + hex.EncodeToString(platformArm64Hash.Sum(nil))
			platformArm64PackageVersion := packages.PackageVersion{
				PackageID:    p.ID,
				Version:      platformArm64Version,
				LowerVersion: platformArm64Version,
				CreatedUnix:  timeutil.TimeStamp(created),
			}
			_, err = db.GetEngine(ctx).NoAutoTime().Insert(&platformArm64PackageVersion)
			require.NoError(t, err)

			// Create the package version for tagged manifest of a multi-platform OCI image
			v := packages.PackageVersion{
				PackageID:    p.ID,
				Version:      version,
				LowerVersion: version,
				CreatedUnix:  timeutil.TimeStamp(created),
			}
			_, err = db.GetEngine(ctx).NoAutoTime().Insert(&v)
			require.NoError(t, err)
		}
		return p
	}

	createPackageVersions := func(t *testing.T, packageType packages.Type, name string, count, daysBetweenPackages int) *packages.Package {
		t.Helper()

		p, err := packages.TryInsertPackage(ctx, &packages.Package{
			OwnerID:   ownerID,
			Name:      name,
			LowerName: name,
			Type:      packageType,
		})
		require.NoError(t, err)

		for i := range count {
			version := fmt.Sprintf("0.%d.0", i+1)
			created := time.Now().
				Add(time.Hour * -1).
				Add(time.Duration((count-i)*daysBetweenPackages) * time.Hour * 24 * -1).
				Unix()

			// Create the package version
			v := packages.PackageVersion{
				PackageID:    p.ID,
				Version:      version,
				LowerVersion: version,
				CreatedUnix:  timeutil.TimeStamp(created),
			}
			_, err = db.GetEngine(ctx).NoAutoTime().Insert(&v)
			require.NoError(t, err)
		}
		return p
	}

	clean := func(t *testing.T, ruleID, packageID int64) {
		t.Helper()

		err := packages.DeleteCleanupRuleByID(ctx, ruleID)
		require.NoError(t, err)

		err = packages.DeletePackageByID(ctx, packageID)
		require.NoError(t, err)
	}

	t.Run("keeps the last five versions of multi-platform container images", func(t *testing.T) {
		pcr := createPackageCleanupRule(t, testPackageCleanupRule{
			Type:      packages.TypeContainer,
			KeepCount: 5,
		})
		// Create versions 0.1.0 to 0.6.0
		p := createContainerVersions(t, "unit/test", 6, 1)

		targets, err := GetCleanupTargets(ctx, pcr, true)
		require.NoError(t, err)
		assert.Len(t, targets, 1)
		assert.Equal(t, "0.1.0", targets[0].PackageVersion.LowerVersion)

		err = ExecuteCleanupRules(ctx)
		require.NoError(t, err)

		clean(t, pcr.ID, p.ID)
	})

	t.Run("keeps the last five days of npm packages", func(t *testing.T) {
		pcr := createPackageCleanupRule(t, testPackageCleanupRule{
			Type:       packages.TypeNpm,
			RemoveDays: 5,
		})
		// Create versions 0.1.0 to 0.6.0
		p := createPackageVersions(t, packages.TypeNpm, "unit/test", 6, 2)

		targets, err := GetCleanupTargets(ctx, pcr, true)
		require.NoError(t, err)
		assert.Len(t, targets, 4)
		assert.Equal(t, "0.4.0", targets[0].PackageVersion.LowerVersion)
		assert.Equal(t, "0.1.0", targets[3].PackageVersion.LowerVersion)

		err = ExecuteCleanupRules(ctx)
		require.NoError(t, err)

		pvs, err := packages.GetVersionsByPackageName(ctx, ownerID, packages.TypeNpm, "unit/test")
		require.NoError(t, err)
		assert.Len(t, pvs, 2)
		assert.Equal(t, "0.6.0", pvs[0].LowerVersion)
		assert.Equal(t, "0.5.0", pvs[1].LowerVersion)

		clean(t, pcr.ID, p.ID)
	})

	t.Run("keeps the npm packages created in the past 5 days based on KeepLastDownloadDays", func(t *testing.T) {
		pcr := createPackageCleanupRule(t, testPackageCleanupRule{
			Type:                 packages.TypeNpm,
			KeepLastDownloadDays: 5,
		})
		// Create versions 0.1.0 to 0.6.0
		p := createPackageVersions(t, packages.TypeNpm, "unit/test", 6, 2)

		targets, err := GetCleanupTargets(ctx, pcr, true)
		require.NoError(t, err)
		assert.Len(t, targets, 4)
		assert.Equal(t, "0.4.0", targets[0].PackageVersion.LowerVersion)
		assert.Equal(t, "0.2.0", targets[2].PackageVersion.LowerVersion)

		err = ExecuteCleanupRules(ctx)
		require.NoError(t, err)

		pvs, err := packages.GetVersionsByPackageName(ctx, ownerID, packages.TypeNpm, "unit/test")
		require.NoError(t, err)
		assert.Len(t, pvs, 2)
		assert.Equal(t, "0.6.0", pvs[0].LowerVersion)
		assert.Equal(t, "0.5.0", pvs[1].LowerVersion)

		clean(t, pcr.ID, p.ID)
	})

	t.Run("keeps the npm packages created or downloaded in the past 5 days", func(t *testing.T) {
		pcr := createPackageCleanupRule(t, testPackageCleanupRule{
			Type:                 packages.TypeNpm,
			KeepLastDownloadDays: 5,
		})
		// Create versions 0.1.0 to 0.6.0
		p := createPackageVersions(t, packages.TypeNpm, "unit/test", 6, 2)
		pv, err := packages.GetVersionByNameAndVersion(ctx, ownerID, packages.TypeNpm, "unit/test", "0.1.0")
		require.NoError(t, err)
		err = packages.IncrementDownloadCounterAndSetLastDownload(ctx, pv.ID)
		require.NoError(t, err)

		targets, err := GetCleanupTargets(ctx, pcr, true)
		require.NoError(t, err)
		assert.Len(t, targets, 3)
		assert.Equal(t, "0.4.0", targets[0].PackageVersion.LowerVersion)
		assert.Equal(t, "0.2.0", targets[2].PackageVersion.LowerVersion)

		err = ExecuteCleanupRules(ctx)
		require.NoError(t, err)

		pvs, err := packages.GetVersionsByPackageName(ctx, ownerID, packages.TypeNpm, "unit/test")
		require.NoError(t, err)
		assert.Len(t, pvs, 3)
		assert.Equal(t, "0.6.0", pvs[0].LowerVersion)
		assert.Equal(t, "0.1.0", pvs[2].LowerVersion)

		clean(t, pcr.ID, p.ID)
	})

	t.Run("removes the maven packages not downloaded or created in the past 5 days", func(t *testing.T) {
		pcr := createPackageCleanupRule(t, testPackageCleanupRule{
			Type:                   packages.TypeMaven,
			RemoveLastDownloadDays: 5,
		})
		// Create versions 0.1.0 to 0.6.0
		p := createPackageVersions(t, packages.TypeMaven, "unit/test", 6, 2)
		pv, err := packages.GetVersionByNameAndVersion(ctx, ownerID, packages.TypeMaven, "unit/test", "0.2.0")
		require.NoError(t, err)
		err = packages.IncrementDownloadCounterAndSetLastDownload(ctx, pv.ID)
		require.NoError(t, err)

		targets, err := GetCleanupTargets(ctx, pcr, true)
		require.NoError(t, err)
		assert.Len(t, targets, 3)
		assert.Equal(t, "0.4.0", targets[0].PackageVersion.LowerVersion)
		assert.Equal(t, "0.1.0", targets[2].PackageVersion.LowerVersion)

		err = ExecuteCleanupRules(ctx)
		require.NoError(t, err)

		pvs, err := packages.GetVersionsByPackageName(ctx, ownerID, packages.TypeMaven, "unit/test")
		require.NoError(t, err)
		assert.Len(t, pvs, 3)
		assert.Equal(t, "0.6.0", pvs[0].LowerVersion)
		assert.Equal(t, "0.2.0", pvs[2].LowerVersion)

		clean(t, pcr.ID, p.ID)
	})
}
