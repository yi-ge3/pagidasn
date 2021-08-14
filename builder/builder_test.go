// Copyright (c) Huawei Technologies Co., Ltd. 2020. All rights reserved.
// isula-build licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Jingxiao Lu
// Create: 2020-07-24
// Description: Test cases for Builder package

package builder

import (
	"context"
	"reflect"
	"testing"

	"gotest.tools/fs"

	constant "isula.org/isula-build"
	pb "isula.org/isula-build/api/services"
	"isula.org/isula-build/builder/dockerfile"
	"isula.org/isula-build/store"
)

func TestNewBuilder(t *testing.T) {
	tmpDir := fs.NewDir(t, t.Name())
	defer tmpDir.Remove()

	type args struct {
		ctx         context.Context
		store       store.Store
		req         *pb.BuildRequest
		runtimePath string
		buildDir    string
		runDir      string
	}
	tests := []struct {
		name    string
		args    args
		want    Builder
		wantErr bool
	}{
		{
			name: "ctr-img",
			args: args{
				ctx:      context.Background(),
				store:    store.Store{},
				req:      &pb.BuildRequest{BuildType: constant.BuildContainerImageType},
				buildDir: tmpDir.Path(),
				runDir:   tmpDir.Path(),
			},
			want:    &dockerfile.Builder{},
			wantErr: false,
		},
		{
			name: "No supported type",
			args: args{
				ctx:   context.Background(),
				store: store.Store{},
				req:   &pb.BuildRequest{BuildType: "Unknown"},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewBuilder(tt.args.ctx, tt.args.store, tt.args.req, tt.args.runtimePath, tt.args.buildDir, tt.args.runDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBuilder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("NewBuilder() got = %v, want %v", reflect.TypeOf(got), reflect.TypeOf(tt.want))
			}
		})
	}
}
