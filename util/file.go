// Copyright (c) Huawei Technologies Co., Ltd. 2020. All rights reserved.
// isula-build licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Xiang Li
// Create: 2021-08-24
// Description: file manipulation related common functions

package util

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
)

const (
	fileMaxSize = 10 * 1024 * 1024 // 10MB
)

// ReadSmallFile read small file less than 10MB
func ReadSmallFile(path string) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > fileMaxSize {
		return nil, errors.Errorf("file %q too big", path)
	}
	return ioutil.ReadFile(path) // nolint: gosec
}

// LoadJSONFile load json files and store it into v
func LoadJSONFile(file string, v interface{}) error {
	f, err := ReadSmallFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(f, v)
}

// ChangeDirModifyTime changes modify time of directory
func ChangeDirModifyTime(dir string) error {
	fs, rErr := ioutil.ReadDir(dir)
	if rErr != nil {
		return rErr
	}
	for _, f := range fs {
		src := filepath.Join(dir, f.Name())
		if err := ChangeFileModifyTime(src); err != nil {
			return err
		}
		if f.IsDir() {
			if err := ChangeDirModifyTime(src); err != nil {
				return err
			}
		}
	}
	return nil
}

// ChangeFileModifyTime changes modify time of file by fixing time at 2017-01-01 00:00:00
func ChangeFileModifyTime(path string) error {
	mtime := time.Date(2017, time.January, 0, 0, 0, 0, 0, time.UTC)
	atime := time.Date(2017, time.January, 0, 0, 0, 0, 0, time.UTC)
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		return err
	}
	return nil
}

// PackFiles will pack files in "src" directory to "dest" file
// by using different compression method defined by "com"
// the files' modify time attribute will be set to a fix time "2017-01-01 00:00:00"
// if set "modifyTime" to true
func PackFiles(src, dest string, com archive.Compression, modifyTime bool) (err error) {
	if modifyTime {
		if err = ChangeDirModifyTime(src); err != nil {
			return err
		}
	}

	reader, err := archive.Tar(src, com)
	if err != nil {
		return err
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer func() {
		cErr := f.Close()
		if cErr != nil && err == nil {
			err = cErr
		}
	}()

	if _, err = io.Copy(f, reader); err != nil {
		return err
	}

	return nil
}

// UnpackFile will unpack "src" file to "dest" directory
// by using different compression method defined by "com"
// The src file will be remove if set "rm" to true
func UnpackFile(src, dest string, com archive.Compression, rm bool) (err error) {
	cleanPath := filepath.Clean(src)
	f, err := os.Open(cleanPath) // nolint:gosec
	if err != nil {
		return errors.Wrapf(err, "unpack: open %q failed", src)
	}

	defer func() {
		cErr := f.Close()
		if cErr != nil && err == nil {
			err = cErr
		}
	}()

	if err = archive.Untar(f, dest, &archive.TarOptions{Compression: com}); err != nil {
		return errors.Wrapf(err, "unpack file %q failed", src)
	}

	if err = ChangeDirModifyTime(dest); err != nil {
		return errors.Wrapf(err, "change modify time for directory %q failed", dest)
	}

	if rm {
		if err = os.RemoveAll(src); err != nil {
			return errors.Errorf("unpack: remove %q failed: %v ", src, err)
		}
	}

	return nil
}