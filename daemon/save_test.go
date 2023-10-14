// Copyright (c) Huawei Technologies Co., Ltd. 2020. All rights reserved.
// isula-build licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Weizheng Xing
// Create: 2020-02-03
// Description: This file tests Save interface

package daemon

import (
	"context"
	"testing"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/stringid"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	constant "isula.org/isula-build"
	pb "isula.org/isula-build/api/services"
	_ "isula.org/isula-build/exporter/register"
	"isula.org/isula-build/pkg/logger"
)

type controlSaveServer struct {
	grpc.ServerStream
}

func (c *controlSaveServer) Context() context.Context {
	return context.Background()
}

func (c *controlSaveServer) Send(response *pb.SaveResponse) error {
	if response.Log == "error" {
		return errors.New("error happened")
	}
	return nil
}

func init() {
	reexec.Init()
}

func TestSave(t *testing.T) {
	d := prepare(t)
	defer tmpClean(d)

	// nolint:godox TODO: create image manually and save
	options := &storage.ImageOptions{}
	img, err := d.Daemon.localStore.CreateImage(stringid.GenerateRandomID(), []string{"image:latest"}, "", "", options)
	if err != nil {
		t.Fatalf("create image with error: %v", err)
	}

	_, err = d.Daemon.localStore.CreateImage(stringid.GenerateRandomID(), []string{"image2:test"}, "", "", options)
	if err != nil {
		t.Fatalf("create image with error: %v", err)
	}

	tempTarfileDir := fs.NewDir(t, t.Name())
	defer tempTarfileDir.Remove()

	testcases := []struct {
		name      string
		req       *pb.SaveRequest
		wantErr   bool
		errString string
	}{
		{
			name: "normal case save with repository[:tag]",
			req: &pb.SaveRequest{
				SaveID: stringid.GenerateNonCryptoID()[:constant.DefaultIDLen],
				Images: []string{"image:latest"},
				Path:   tempTarfileDir.Join("repotag.tar"),
				Format: "docker",
			},
			wantErr:   true,
			errString: "file does not exist",
		},
		{
			name: "normal case save with repository add default latest",
			req: &pb.SaveRequest{
				SaveID: stringid.GenerateNonCryptoID()[:constant.DefaultIDLen],
				Images: []string{"image"},
				Path:   tempTarfileDir.Join("repolatest.tar"),
				Format: "oci",
			},
			wantErr:   true,
			errString: "file does not exist",
		},
		{
			name: "normal case with imageid",
			req: &pb.SaveRequest{
				SaveID: stringid.GenerateNonCryptoID()[:constant.DefaultIDLen],
				Images: []string{img.ID},
				Path:   tempTarfileDir.Join("imageid.tar"),
				Format: "docker",
			},
			wantErr:   true,
			errString: "file does not exist",
		},
		{
			name: "normal case save multiple images with repository and ID",
			req: &pb.SaveRequest{
				SaveID: stringid.GenerateNonCryptoID()[:constant.DefaultIDLen],
				Images: []string{"image2:test", img.ID},
				Path:   tempTarfileDir.Join("double.tar"),
				Format: "docker",
			},
			wantErr:   true,
			errString: "file does not exist",
		},
		{
			name: "abnormal case save image that not exist in local store",
			req: &pb.SaveRequest{
				SaveID: stringid.GenerateNonCryptoID()[:constant.DefaultIDLen],
				Images: []string{"noexist", img.ID},
				Path:   tempTarfileDir.Join("notexist.tar"),
				Format: "docker",
			},
			wantErr:   true,
			errString: "not found in local store",
		},
		{
			name: "abnormal case wrong image format",
			req: &pb.SaveRequest{
				SaveID: stringid.GenerateNonCryptoID()[:constant.DefaultIDLen],
				Images: []string{"image", img.ID},
				Path:   tempTarfileDir.Join("image.tar"),
				Format: "dock",
			},
			wantErr:   true,
			errString: "wrong image format provided",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			stream := &controlSaveServer{}

			err := d.Daemon.backend.Save(tc.req, stream)
			if tc.wantErr == true {
				assert.ErrorContains(t, err, tc.errString)
			}
			if tc.wantErr == false {
				assert.NilError(t, err)
			}
		})
	}

}

func TestSaveHandler(t *testing.T) {
	ctx := context.TODO()
	eg, _ := errgroup.WithContext(ctx)

	eg.Go(saveHandlerPrint("Push Response"))
	eg.Go(saveHandlerPrint(""))
	eg.Go(saveHandlerPrint("error"))

	eg.Wait()
}

func saveHandlerPrint(message string) func() error {
	return func() error {
		stream := &controlSaveServer{}
		cliLogger := logger.NewCliLogger(constant.CliLogBufferLen)

		ctx := context.TODO()
		eg, _ := errgroup.WithContext(ctx)

		eg.Go(messageHandler(stream, cliLogger))
		eg.Go(func() error {
			cliLogger.Print(message)
			cliLogger.CloseContent()
			return nil
		})

		eg.Wait()

		return nil
	}
}
