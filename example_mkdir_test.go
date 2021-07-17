// Copyright 2020 Marko Kungla. All rights reserved.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// +build !windows,!plan9

package bexp_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mkungla/bexp/v2"
)

func ExampleMkdirAll() {
	const (
		rootdir = "/tmp/bexp"
		treeexp = rootdir + "/{dir1,dir2,dir3/{subdir1,subdir2}}"
	)
	if err := bexp.MkdirAll(treeexp, 0750); err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(rootdir)

	err := filepath.Walk(rootdir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		})
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// /tmp/bexp
	// /tmp/bexp/dir1
	// /tmp/bexp/dir2
	// /tmp/bexp/dir3
	// /tmp/bexp/dir3/subdir1
	// /tmp/bexp/dir3/subdir2
}
