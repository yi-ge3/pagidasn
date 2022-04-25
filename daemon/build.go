// Copyright (c) Huawei Technologies Co., Ltd. 2020. All rights reserved.
// isula-build licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: iSula Team
// Create: 2020-01-20
// Description: This file is "build" command for backend

package daemon

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	constant "isula.org/isula-build"
	pb "isula.org/isula-build/api/services"
	"isula.org/isula-build/exporter"
	"isula.org/isula-build/util"
)

// Build receives a build request and build an image
func (b *Backend) Build(req *pb.BuildRequest, stream pb.Control_BuildServer) error { // nolint:gocyclo
	b.wg.Add(1)
	defer b.wg.Done()
	logrus.WithFields(logrus.Fields{
		"BuildType": req.GetBuildType(),
		"BuildID":   req.GetBuildID(),
	}).Info("BuildRequest received")

	ctx := context.WithValue(stream.Context(), util.LogFieldKey(util.LogKeySessionID), req.BuildID)
	builder, nerr := b.daemon.NewBuilder(ctx, req)
	if nerr != nil {
		return nerr
	}

	defer func() {
		if cerr := builder.CleanResources(); cerr != nil {
			logrus.Warnf("defer builder clean build resources failed: %v", cerr)
		}
		b.daemon.deleteBuilder(req.BuildID)
		b.deleteStatus(req.BuildID)
	}()

	var (
		imageID string
		errChan = make(chan error, 1)
	)

	pipeWrapper := builder.OutputPipeWrapper()
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		b.syncBuildStatus(req.BuildID) <- struct{}{}
		close(b.status[req.BuildID].startBuild)
		var berr error
		imageID, berr = builder.Build()

		// in case there is error during Build stage, the backend will always waiting for content write into
		// the pipeFile, which will cause frontend hangs forever.
		// so if the output type is archive(pipeFile is not empty string) and any error occurred, we write the error
		// message into the pipe to make the goroutine move on instead of hangs.
		if berr != nil && pipeWrapper != nil {
			pipeWrapper.Close()
			if perr := ioutil.WriteFile(pipeWrapper.PipeFile, []byte(berr.Error()), constant.DefaultRootFileMode); perr != nil {
				logrus.WithField(util.LogKeySessionID, req.BuildID).Warnf("Write error [%v] in to pipe file failed: %v", berr, perr)
			}
		}

		return berr
	})

	eg.Go(func() error {
		if pipeWrapper == nil {
			return nil
		}
		f, perr := exporter.PipeArchiveStream(pipeWrapper)
		if perr != nil {
			return perr
		}
		defer func() {
			if cErr := f.Close(); cErr != nil {
				logrus.WithField(util.LogKeySessionID, req.BuildID).Warnf("Closing archive stream pipe %q failed: %v", pipeWrapper.PipeFile, cErr)
			}
		}()

		reader := bufio.NewReader(f)
		buf := make([]byte, constant.BufferSize, constant.BufferSize)
		for {
			length, rerr := reader.Read(buf)
			if rerr == io.EOF || pipeWrapper.Done {
				break
			}
			if rerr != nil {
				return rerr
			}

			if serr := stream.Send(&pb.BuildResponse{Data: buf[0:length]}); serr != nil {
				return serr
			}
		}
		logrus.WithField(util.LogKeySessionID, req.BuildID).Debugf("Piping build archive stream done")
		return nil
	})

	go func() {
		errChan <- eg.Wait()
	}()

	select {
	case chErr := <-errChan:
		close(errChan)
		if chErr != nil {
			return chErr
		}
		// export done, send client the imageID
		if serr := stream.Send(&pb.BuildResponse{
			Data:    nil,
			ImageID: imageID,
		}); serr != nil {
			return serr
		}
	case <-stream.Context().Done():
		ctxErr := ctx.Err()
		if ctxErr != nil && ctxErr != context.Canceled {
			logrus.WithField(util.LogKeySessionID, req.BuildID).Warnf("Stream closed with: %v", ctxErr)
		}
	}

	return nil
}
