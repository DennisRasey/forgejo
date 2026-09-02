// Copyright 2022 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages_test

import (
	"encoding/hex"
	"testing"

	"forgejo.org/models/db"
	packages_model "forgejo.org/models/packages"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	packages_module "forgejo.org/modules/packages"
	"forgejo.org/modules/timeutil"
	packages_service "forgejo.org/services/packages"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareExamplePackage(t *testing.T) *packages_model.Package {
	require.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	p0 := &packages_model.Package{
		OwnerID:   owner.ID,
		RepoID:    repo.ID,
		LowerName: "package",
		Type:      packages_model.TypeGeneric,
	}

	p, err := packages_model.TryInsertPackage(db.DefaultContext, p0)
	require.NotNil(t, p)
	require.NoError(t, err)
	require.Equal(t, *p0, *p)
	return p
}

func deletePackage(t *testing.T, p *packages_model.Package) {
	err := packages_model.DeletePackageByID(db.DefaultContext, p.ID)
	require.NoError(t, err)
}

func TestTryInsertPackage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	p0 := &packages_model.Package{
		OwnerID:   owner.ID,
		LowerName: "package",
	}

	// Insert package should return the package and yield no error
	p, err := packages_model.TryInsertPackage(db.DefaultContext, p0)
	require.NotNil(t, p)
	require.NoError(t, err)
	require.Equal(t, *p0, *p)

	// Insert same package again should return the same package and yield ErrDuplicatePackage
	p, err = packages_model.TryInsertPackage(db.DefaultContext, p0)
	require.NotNil(t, p)
	require.IsType(t, packages_model.ErrDuplicatePackage, err)
	require.Equal(t, *p0, *p)

	err = packages_model.DeletePackageByID(db.DefaultContext, p0.ID)
	require.NoError(t, err)
}

func TestGetPackageByID(t *testing.T) {
	p0 := prepareExamplePackage(t)

	// Get package should return package and yield no error
	p, err := packages_model.GetPackageByID(db.DefaultContext, p0.ID)
	require.NotNil(t, p)
	require.Equal(t, *p0, *p)
	require.NoError(t, err)

	// Get package with non-existing ID should yield ErrPackageNotExist
	p, err = packages_model.GetPackageByID(db.DefaultContext, 999)
	require.Nil(t, p)
	require.Error(t, err)
	require.IsType(t, packages_model.ErrPackageNotExist, err)

	deletePackage(t, p0)
}

func TestDeletePackageByID(t *testing.T) {
	p0 := prepareExamplePackage(t)

	// Delete existing package should yield no error
	err := packages_model.DeletePackageByID(db.DefaultContext, p0.ID)
	require.NoError(t, err)

	// Delete (now) non-existing package should yield ErrPackageNotExist
	err = packages_model.DeletePackageByID(db.DefaultContext, p0.ID)
	require.Error(t, err)
	require.IsType(t, packages_model.ErrPackageNotExist, err)
}

func TestSetRepositoryLink(t *testing.T) {
	p0 := prepareExamplePackage(t)

	// Set repository link to package should yield no error and package RepoID should be updated
	err := packages_model.SetRepositoryLink(db.DefaultContext, p0.ID, 5)
	require.NoError(t, err)

	p, err := packages_model.GetPackageByID(db.DefaultContext, p0.ID)
	require.NoError(t, err)
	require.EqualValues(t, 5, p.RepoID)

	// Set repository link to non-existing package should yied ErrPackageNotExist
	err = packages_model.SetRepositoryLink(db.DefaultContext, 999, 5)
	require.Error(t, err)
	require.IsType(t, packages_model.ErrPackageNotExist, err)

	deletePackage(t, p0)
}

func TestUnlinkRepositoryFromAllPackages(t *testing.T) {
	p0 := prepareExamplePackage(t)

	// Unlink repository from all packages should yield no error and package with p0.ID should have RepoID 0
	err := packages_model.UnlinkRepositoryFromAllPackages(db.DefaultContext, p0.RepoID)
	require.NoError(t, err)

	p, err := packages_model.GetPackageByID(db.DefaultContext, p0.ID)
	require.NoError(t, err)
	require.EqualValues(t, 0, p.RepoID)

	// Unlink repository again from all packages should also yield no error
	err = packages_model.UnlinkRepositoryFromAllPackages(db.DefaultContext, p0.RepoID)
	require.NoError(t, err)

	deletePackage(t, p0)
}

func TestGetPackageByName(t *testing.T) {
	p0 := prepareExamplePackage(t)

	// Get package should return package and yield no error
	p, err := packages_model.GetPackageByName(db.DefaultContext, p0.OwnerID, p0.Type, p0.LowerName)
	require.NotNil(t, p)
	require.Equal(t, *p0, *p)
	require.NoError(t, err)

	// Get package with uppercase name should return package and yield no error
	p, err = packages_model.GetPackageByName(db.DefaultContext, p0.OwnerID, p0.Type, "Package")
	require.NotNil(t, p)
	require.Equal(t, *p0, *p)
	require.NoError(t, err)

	// Get package with wrong owner ID, type or name should return no package and yield ErrPackageNotExist
	p, err = packages_model.GetPackageByName(db.DefaultContext, 999, p0.Type, p0.LowerName)
	require.Nil(t, p)
	require.Error(t, err)
	require.IsType(t, packages_model.ErrPackageNotExist, err)
	p, err = packages_model.GetPackageByName(db.DefaultContext, p0.OwnerID, packages_model.TypeDebian, p0.LowerName)
	require.Nil(t, p)
	require.Error(t, err)
	require.IsType(t, packages_model.ErrPackageNotExist, err)
	p, err = packages_model.GetPackageByName(db.DefaultContext, p0.OwnerID, p0.Type, "package1")
	require.Nil(t, p)
	require.Error(t, err)
	require.IsType(t, packages_model.ErrPackageNotExist, err)

	deletePackage(t, p0)
}

func TestHasCountPackages(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	p, err := packages_model.TryInsertPackage(db.DefaultContext, &packages_model.Package{
		OwnerID:   owner.ID,
		RepoID:    repo.ID,
		LowerName: "package",
	})
	require.NotNil(t, p)
	require.NoError(t, err)

	// A package without package versions gets automatically cleaned up and should return false for owner
	has, err := packages_model.HasOwnerPackages(db.DefaultContext, owner.ID)
	require.False(t, has)
	require.NoError(t, err)
	count, err := packages_model.CountOwnerPackages(db.DefaultContext, owner.ID)
	require.EqualValues(t, 0, count)
	require.NoError(t, err)

	// A package without package versions gets automatically cleaned up and should return false for repository
	has, err = packages_model.HasRepositoryPackages(db.DefaultContext, repo.ID)
	require.False(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountRepositoryPackages(db.DefaultContext, repo.ID)
	require.EqualValues(t, 0, count)
	require.NoError(t, err)

	pv, err := packages_model.GetOrInsertVersion(db.DefaultContext, &packages_model.PackageVersion{
		PackageID:    p.ID,
		LowerVersion: "internal",
		IsInternal:   true,
	})
	require.NotNil(t, pv)
	require.NoError(t, err)

	// A package with an internal package version gets automatically cleaned up and should return false
	has, err = packages_model.HasOwnerPackages(db.DefaultContext, owner.ID)
	require.False(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountOwnerPackages(db.DefaultContext, owner.ID)
	require.EqualValues(t, 0, count)
	require.NoError(t, err)
	has, err = packages_model.HasRepositoryPackages(db.DefaultContext, repo.ID)
	require.False(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountRepositoryPackages(db.DefaultContext, repo.ID)
	require.EqualValues(t, 0, count)
	require.NoError(t, err)

	pv, err = packages_model.GetOrInsertVersion(db.DefaultContext, &packages_model.PackageVersion{
		PackageID:    p.ID,
		LowerVersion: "normal",
		IsInternal:   false,
	})
	require.NotNil(t, pv)
	require.NoError(t, err)

	// A package with a normal package version should return true
	has, err = packages_model.HasOwnerPackages(db.DefaultContext, owner.ID)
	require.True(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountOwnerPackages(db.DefaultContext, owner.ID)
	require.EqualValues(t, 1, count)
	require.NoError(t, err)
	has, err = packages_model.HasRepositoryPackages(db.DefaultContext, repo.ID)
	require.True(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountRepositoryPackages(db.DefaultContext, repo.ID)
	require.EqualValues(t, 1, count)
	require.NoError(t, err)

	pv2, err := packages_model.GetOrInsertVersion(db.DefaultContext, &packages_model.PackageVersion{
		PackageID:    p.ID,
		LowerVersion: "normal2",
		IsInternal:   false,
	})
	require.NotNil(t, pv2)
	require.NoError(t, err)

	// A package with multiple package versions should be counted only once
	has, err = packages_model.HasOwnerPackages(db.DefaultContext, owner.ID)
	require.True(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountOwnerPackages(db.DefaultContext, owner.ID)
	require.EqualValues(t, 1, count)
	require.NoError(t, err)
	has, err = packages_model.HasRepositoryPackages(db.DefaultContext, repo.ID)
	require.True(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountRepositoryPackages(db.DefaultContext, repo.ID)
	require.EqualValues(t, 1, count)
	require.NoError(t, err)

	// For owner ID 0 there should be no packages
	has, err = packages_model.HasOwnerPackages(db.DefaultContext, 0)
	require.False(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountOwnerPackages(db.DefaultContext, 0)
	require.EqualValues(t, 0, count)
	require.NoError(t, err)

	// For repo ID 0 there should be no packages
	has, err = packages_model.HasRepositoryPackages(db.DefaultContext, 0)
	require.False(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountRepositoryPackages(db.DefaultContext, 0)
	require.EqualValues(t, 0, count)
	require.NoError(t, err)

	p1, err := packages_model.TryInsertPackage(db.DefaultContext, &packages_model.Package{
		OwnerID:   owner.ID,
		LowerName: "package0",
	})
	require.NotNil(t, p1)
	require.NoError(t, err)
	p1v, err := packages_model.GetOrInsertVersion(db.DefaultContext, &packages_model.PackageVersion{
		PackageID:    p1.ID,
		LowerVersion: "normal",
		IsInternal:   false,
	})
	require.NotNil(t, p1v)
	require.NoError(t, err)

	// Owner owner.ID should have two packages now
	has, err = packages_model.HasOwnerPackages(db.DefaultContext, owner.ID)
	require.True(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountOwnerPackages(db.DefaultContext, owner.ID)
	require.EqualValues(t, 2, count)
	require.NoError(t, err)

	// For repo ID 0 there should be now one package, because p1 is not assigned to a repo
	has, err = packages_model.HasRepositoryPackages(db.DefaultContext, 0)
	require.True(t, has)
	require.NoError(t, err)
	count, err = packages_model.CountRepositoryPackages(db.DefaultContext, 0)
	require.EqualValues(t, 1, count)
	require.NoError(t, err)
}

func TestPackageTotalSize(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	data, err := packages_module.NewHashedBuffer()
	require.NoError(t, err)

	pv, _, err := packages_service.CreatePackageAndAddFile(ctx, &packages_service.PackageCreationInfo{
		PackageInfo: packages_service.PackageInfo{
			Owner:       owner,
			PackageType: packages_model.TypeGeneric,
			Name:        "Bleihai",
			Version:     "1337",
		},
		Creator: owner,
	}, &packages_service.PackageFileCreationInfo{
		PackageFileInfo: packages_service.PackageFileInfo{Filename: "uwu"},
		Data:            data,
		Creator:         owner,
		IsLead:          true,
	})

	require.NoError(t, err)

	blobs := []*packages_model.PackageBlob{
		{
			Size:        10,
			CreatedUnix: timeutil.TimeStampNow(),
		},
		{
			Size:        20,
			CreatedUnix: timeutil.TimeStampNow().Add(1),
		},
		{
			Size:        40,
			CreatedUnix: timeutil.TimeStampNow().Add(2),
		},
	}

	for i, b := range blobs {
		timeBuf := []byte(b.CreatedUnix.AsTime().String())
		hsr := packages_module.NewMultiHasher()
		_, err := hsr.Write(timeBuf)
		require.NoError(t, err)

		md5hash, sha1hash, sha256hash, sha512hash, blake2bhash := hsr.Sums()
		b.HashMD5 = hex.EncodeToString(md5hash)
		b.HashSHA1 = hex.EncodeToString(sha1hash)
		b.HashSHA256 = hex.EncodeToString(sha256hash)
		b.HashSHA512 = hex.EncodeToString(sha512hash)
		b.HashBlake2b = hex.EncodeToString(blake2bhash)

		b, _, err := packages_model.GetOrInsertBlob(ctx, b)
		require.NoError(t, err)
		blobs[i] = b
	}

	files := []struct {
		Size int64
		File *packages_model.PackageFile
	}{
		{
			Size: 10,
			File: &packages_model.PackageFile{
				VersionID: pv.ID,
				BlobID:    blobs[0].ID,
				Name:      "shark",
				LowerName: "shark",
			},
		},
		{
			Size: 20,
			File: &packages_model.PackageFile{
				VersionID: pv.ID,
				BlobID:    blobs[1].ID,
				Name:      "shark1",
				LowerName: "shark1",
			},
		},
		{
			Size: 40,
			File: &packages_model.PackageFile{
				VersionID: pv.ID,
				BlobID:    blobs[2].ID,
				Name:      "shark2",
				LowerName: "shark2",
			},
		},
	}

	totalSize := int64(0)
	for _, f := range files {
		_, err := packages_model.TryInsertFile(ctx, f.File)
		require.NoError(t, err)

		pv, err = packages_model.GetVersionByID(ctx, pv.ID)
		require.NoError(t, err)

		totalSize += f.Size
		assert.Equal(t, totalSize, pv.TotalSize)
	}

	for _, f := range files {
		err := packages_model.DeleteFileByID(ctx, f.File.ID)
		require.NoError(t, err)

		pv, err = packages_model.GetVersionByID(ctx, pv.ID)
		require.NoError(t, err)

		totalSize -= f.Size
		assert.Equal(t, totalSize, pv.TotalSize)
	}
}

func TestSortPackages(t *testing.T) {
	defer unittest.OverrideFixtures("models/packages/fixtures/TestSortPackages")()
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()

	// default sort options (unix desc)
	pvs, count, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{})
	require.NoError(t, err)
	assert.EqualValues(t, 4, count)

	expectedOrder := []int64{77, 76, 75, 74}
	for i, pv := range pvs {
		assert.Equal(t, expectedOrder[i], pv.ID)
	}

	// sort by name
	pvs, count, err = packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		Sort: packages_model.SortNameAsc,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 4, count)

	expectedOrder = []int64{74, 76, 75, 77}
	for i, pv := range pvs {
		assert.Equal(t, expectedOrder[i], pv.ID)
	}

	// sort by version
	pvs, count, err = packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		Sort: packages_model.SortVersionAsc,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 4, count)

	expectedOrder = []int64{74, 75, 76, 77}
	for i, pv := range pvs {
		assert.Equal(t, expectedOrder[i], pv.ID)
	}

	// sort by total size
	pvs, count, err = packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		Sort: packages_model.SortSizeAsc,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 4, count)

	expectedOrder = []int64{77, 75, 74, 76}
	for i, pv := range pvs {
		assert.Equal(t, expectedOrder[i], pv.ID)
	}
}

func TestDownloadCounterAndLastDownload(t *testing.T) {
	p := prepareExamplePackage(t)
	pv0, err := packages_model.GetOrInsertVersion(db.DefaultContext, &packages_model.PackageVersion{
		PackageID:    p.ID,
		LowerVersion: "normal",
		IsInternal:   false,
	})
	require.NotNil(t, pv0)
	require.NoError(t, err)

	referenceTime := timeutil.TimeStamp(0)
	// There should be no downloads, yet.
	pv, err := packages_model.GetVersionByID(db.DefaultContext, pv0.ID)
	require.NoError(t, err)
	require.EqualValues(t, 0, pv.DownloadCount)
	require.Equal(t, referenceTime, pv.LastDownloadUnix)

	// Increment Download Counter And Set Last Download should yield no error
	err = packages_model.IncrementDownloadCounterAndSetLastDownload(db.DefaultContext, pv.ID)
	require.NoError(t, err)

	referenceTime = timeutil.TimeStampNow()
	// The download count should be incremented and the last download time not null.
	pv, err = packages_model.GetVersionByID(db.DefaultContext, pv.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, pv.DownloadCount)
	require.GreaterOrEqual(t, pv.LastDownloadUnix.AsLocalTime().Compare(referenceTime.AsLocalTime()), 0)

	deletePackage(t, p)
}
