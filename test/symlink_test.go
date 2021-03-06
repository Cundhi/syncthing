// Copyright (C) 2014 The Syncthing Authors.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for
// more details.
//
// You should have received a copy of the GNU General Public License along
// with this program. If not, see <http://www.gnu.org/licenses/>.

// +build integration

package integration

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/syncthing/syncthing/internal/config"
	"github.com/syncthing/syncthing/internal/protocol"
	"github.com/syncthing/syncthing/internal/symlinks"
)

func symlinksSupported() bool {
	tmp, err := ioutil.TempDir("", "symlink-test")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tmp)
	err = os.Symlink("tmp", filepath.Join(tmp, "link"))
	return err == nil
}

func TestSymlinks(t *testing.T) {
	if !symlinksSupported() {
		t.Skip("symlinks unsupported")
	}

	// Use no versioning
	id, _ := protocol.DeviceIDFromString(id2)
	cfg, _ := config.Load("h2/config.xml", id)
	fld := cfg.Folders()["default"]
	fld.Versioning = config.VersioningConfiguration{}
	cfg.SetFolder(fld)
	cfg.Save()

	testSymlinks(t)
}

func TestSymlinksSimpleVersioning(t *testing.T) {
	if !symlinksSupported() {
		t.Skip("symlinks unsupported")
	}

	// Use simple versioning
	id, _ := protocol.DeviceIDFromString(id2)
	cfg, _ := config.Load("h2/config.xml", id)
	fld := cfg.Folders()["default"]
	fld.Versioning = config.VersioningConfiguration{
		Type:   "simple",
		Params: map[string]string{"keep": "5"},
	}
	cfg.SetFolder(fld)
	cfg.Save()

	testSymlinks(t)
}

func TestSymlinksStaggeredVersioning(t *testing.T) {
	if !symlinksSupported() {
		t.Skip("symlinks unsupported")
	}

	// Use staggered versioning
	id, _ := protocol.DeviceIDFromString(id2)
	cfg, _ := config.Load("h2/config.xml", id)
	fld := cfg.Folders()["default"]
	fld.Versioning = config.VersioningConfiguration{
		Type: "staggered",
	}
	cfg.SetFolder(fld)
	cfg.Save()

	testSymlinks(t)
}

func testSymlinks(t *testing.T) {
	log.Println("Cleaning...")
	err := removeAll("s1", "s2", "h1/index", "h2/index")
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Generating files...")
	err = generateFiles("s1", 100, 20, "../LICENSE")
	if err != nil {
		t.Fatal(err)
	}

	// A file that we will replace with a symlink later

	fd, err := os.Create("s1/fileToReplace")
	if err != nil {
		t.Fatal(err)
	}
	fd.Close()

	// A directory that we will replace with a symlink later

	err = os.Mkdir("s1/dirToReplace", 0755)
	if err != nil {
		t.Fatal(err)
	}

	// A file and a symlink to that file

	fd, err = os.Create("s1/file")
	if err != nil {
		t.Fatal(err)
	}
	fd.Close()
	err = symlinks.Create("s1/fileLink", "file", 0)
	if err != nil {
		log.Fatal(err)
	}

	// A directory and a symlink to that directory

	err = os.Mkdir("s1/dir", 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = symlinks.Create("s1/dirLink", "dir", 0)
	if err != nil {
		log.Fatal(err)
	}

	// A link to something in the repo that does not exist

	err = symlinks.Create("s1/noneLink", "does/not/exist", 0)
	if err != nil {
		log.Fatal(err)
	}

	// A link we will replace with a file later

	err = symlinks.Create("s1/repFileLink", "does/not/exist", 0)
	if err != nil {
		log.Fatal(err)
	}

	// A link we will replace with a directory later

	err = symlinks.Create("s1/repDirLink", "does/not/exist", 0)
	if err != nil {
		log.Fatal(err)
	}

	// A link we will remove later

	err = symlinks.Create("s1/removeLink", "does/not/exist", 0)
	if err != nil {
		log.Fatal(err)
	}

	// Verify that the files and symlinks sync to the other side

	log.Println("Syncing...")

	sender := syncthingProcess{ // id1
		instance: "1",
		argv:     []string{"-home", "h1"},
		port:     8081,
		apiKey:   apiKey,
	}
	err = sender.start()
	if err != nil {
		t.Fatal(err)
	}

	receiver := syncthingProcess{ // id2
		instance: "2",
		argv:     []string{"-home", "h2"},
		port:     8082,
		apiKey:   apiKey,
	}
	err = receiver.start()
	if err != nil {
		_ = sender.stop()
		t.Fatal(err)
	}

	for {
		comp, err := sender.peerCompletion()
		if err != nil {
			if isTimeout(err) {
				time.Sleep(time.Second)
				continue
			}
			_ = sender.stop()
			_ = receiver.stop()
			t.Fatal(err)
		}

		curComp := comp[id2]

		if curComp == 100 {
			break
		}

		time.Sleep(time.Second)
	}

	err = sender.stop()
	if err != nil {
		t.Fatal(err)
	}
	err = receiver.stop()
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Comparing directories...")
	err = compareDirectories("s1", "s2")
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Making some changes...")

	// Remove one symlink

	err = os.Remove("s1/fileLink")
	if err != nil {
		log.Fatal(err)
	}

	// Change the target of another

	err = os.Remove("s1/dirLink")
	if err != nil {
		log.Fatal(err)
	}
	err = symlinks.Create("s1/dirLink", "file", 0)
	if err != nil {
		log.Fatal(err)
	}

	// Replace one with a file

	err = os.Remove("s1/repFileLink")
	if err != nil {
		log.Fatal(err)
	}

	fd, err = os.Create("s1/repFileLink")
	if err != nil {
		log.Fatal(err)
	}
	fd.Close()

	// Replace one with a directory

	err = os.Remove("s1/repDirLink")
	if err != nil {
		log.Fatal(err)
	}

	err = os.Mkdir("s1/repDirLink", 0755)
	if err != nil {
		log.Fatal(err)
	}

	// Replace a file with a symlink

	err = os.Remove("s1/fileToReplace")
	if err != nil {
		log.Fatal(err)
	}
	err = symlinks.Create("s1/fileToReplace", "somewhere/non/existent", 0)
	if err != nil {
		log.Fatal(err)
	}

	// Replace a directory with a symlink

	err = os.RemoveAll("s1/dirToReplace")
	if err != nil {
		log.Fatal(err)
	}
	err = symlinks.Create("s1/dirToReplace", "somewhere/non/existent", 0)
	if err != nil {
		log.Fatal(err)
	}

	// Remove a broken symlink

	err = os.Remove("s1/removeLink")
	if err != nil {
		log.Fatal(err)
	}

	// Sync these changes and recheck

	log.Println("Syncing...")

	err = sender.start()
	if err != nil {
		t.Fatal(err)
	}

	err = receiver.start()
	if err != nil {
		_ = sender.stop()
		t.Fatal(err)
	}

	for {
		comp, err := sender.peerCompletion()
		if err != nil {
			if isTimeout(err) {
				time.Sleep(time.Second)
				continue
			}
			_ = sender.stop()
			_ = receiver.stop()
			t.Fatal(err)
		}

		curComp := comp[id2]

		if curComp == 100 {
			break
		}

		time.Sleep(time.Second)
	}

	err = sender.stop()
	if err != nil {
		t.Fatal(err)
	}
	err = receiver.stop()
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Comparing directories...")
	err = compareDirectories("s1", "s2")
	if err != nil {
		t.Fatal(err)
	}
}
