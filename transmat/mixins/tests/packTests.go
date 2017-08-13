package tests

import (
	"context"
	"fmt"

	. "github.com/smartystreets/goconvey/convey"

	"go.polydawn.net/rio/fs"
	"go.polydawn.net/rio/fs/osfs"
	"go.polydawn.net/rio/testutil"
	"go.polydawn.net/timeless-api"
	"go.polydawn.net/timeless-api/rio"
)

func CheckPackProducesConsistentHash(pack rio.PackFunc) {
	Convey("SPEC: Applying the PackFunc to a filesystem twice should produce the same hash", func() {
		for _, fixture := range AllFixtures {
			Convey(fmt.Sprintf("- Fixture %q", fixture.Name), func() {
				testutil.WithTmpdir(func(tmpDir fs.AbsolutePath) {
					afs := osfs.New(tmpDir)
					// Set up fixture.
					PlaceFixture(afs, fixture.Files)
					// Pack (to /dev/null) once.
					wareID1, err := pack(
						context.Background(),
						tmpDir.String(),
						api.FilesetFilters{},
						"",
						rio.Monitor{},
					)
					So(err, ShouldBeNil)
					// Pack (to /dev/null) from the same path a second time.
					wareID2, err := pack(
						context.Background(),
						tmpDir.String(),
						api.FilesetFilters{},
						"",
						rio.Monitor{},
					)
					So(err, ShouldBeNil)
					// Should be same output.
					//  This is both an assertion that the pack hasher is consistent,
					//  and that it's not making arbitrary mutations during its passage.
					So(wareID1, ShouldResemble, wareID2)
				})
			})
		}
	})
}

func CheckPackHashVariesOnVariations(pack rio.PackFunc) {
	// Compute the alpha fixture hash once up front; we compare to it
	//  for each other variation fixture.
	var wareIDAlpha api.WareID
	testutil.WithTmpdir(func(tmpDir fs.AbsolutePath) {
		afs := osfs.New(tmpDir)
		PlaceFixture(afs, FixtureAlpha)
		wareIDAlpha, _ = pack(
			context.Background(),
			tmpDir.String(),
			api.FilesetFilters{
				Mtime: "keep",
			},
			"",
			rio.Monitor{},
		)
	})
	Convey("SPEC: Applying the PackFunc to a fileset with different attributes should vary in result hash", func() {
		for _, fixture := range []struct {
			Name  string
			Files []FixtureFile
		}{
			{"AlphaDiffContent", FixtureAlphaDiffContent},
			{"AlphaDiffTime", FixtureAlphaDiffTime},
			{"AlphaDiffPerm", FixtureAlphaDiffPerm},
			{"AlphaDiffPerm2", FixtureAlphaDiffPerm2},
			{"AlphaDiffPerm3", FixtureAlphaDiffPerm3},
			{"AlphaDiffUidGid", FixtureAlphaDiffUidGid},
		} {
			Convey(fmt.Sprintf("- Fixture %q vs %q", "Alpha", fixture.Name), func() {
				testutil.WithTmpdir(func(tmpDir fs.AbsolutePath) {
					afs := osfs.New(tmpDir)
					// Set up fixture.
					PlaceFixture(afs, fixture.Files)
					// Pack (to /dev/null).
					wareID, err := pack(
						context.Background(),
						tmpDir.String(),
						api.FilesetFilters{
							Mtime: "keep",
						},
						"",
						rio.Monitor{},
					)
					So(err, ShouldBeNil)
					// All of these filesystems vary, so they should have different hashes.
					So(wareID, ShouldNotResemble, wareIDAlpha)
				})
			})
		}
	})
}
