// Copyright 2022 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ipam

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	argtypes "antrea.io/antrea/pkg/agent/cniserver/types"
	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/stretchr/testify/assert"
)

type networkConf struct {
	CNIVersion string `json:"cniVersion"`
}

var (
	testNetworkConfig, _ = json.Marshal(networkConf{CNIVersion: "0.4.0"})
	fakeExecWithResult   = func(ctx context.Context, pluginPath string, netconf []byte, args invoke.CNIArgs, exec invoke.Exec) (types.Result, error) {
		return &current.Result{
			CNIVersion: "0.4.0",
		}, nil
	}
	fakeExecWithResultReturnErr = func(ctx context.Context, pluginPath string, netconf []byte, args invoke.CNIArgs, exec invoke.Exec) (types.Result, error) {
		return &current.Result{
			CNIVersion: "0.4.0",
		}, fmt.Errorf("error")
	}
	fakeExecNoResult = func(ctx context.Context, pluginPath string, netconf []byte, args invoke.CNIArgs, exec invoke.Exec) error {
		return nil
	}
)

func TestAdd(t *testing.T) {
	testCases := []struct {
		name               string
		args               invoke.Args
		k8sArgs            argtypes.K8sArgs
		execWithResultFunc func(ctx context.Context, pluginPath string, netconf []byte, args invoke.CNIArgs, exec invoke.Exec) (types.Result, error)
		execNoResultFunc   func(ctx context.Context, pluginPath string, netconf []byte, args invoke.CNIArgs, exec invoke.Exec) error
		expectedRes        error
	}{
		{
			name:               "Test Add",
			args:               invoke.Args{Path: defaultCNIPath},
			execWithResultFunc: fakeExecWithResult,
			execNoResultFunc:   fakeExecNoResult,
			expectedRes:        nil,
		},
		{
			name:               "Test Add no success",
			args:               invoke.Args{Path: defaultCNIPath},
			execWithResultFunc: fakeExecWithResultReturnErr,
			execNoResultFunc:   fakeExecNoResult,
			expectedRes:        fmt.Errorf("error"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			execPluginWithResultFunc = testCase.execWithResultFunc
			execPluginNoResultFunc = testCase.execNoResultFunc
			d := &IPAMDelegator{pluginType: ipamHostLocal}
			_, _, err := d.Add(&testCase.args, &testCase.k8sArgs, testNetworkConfig)
			assert.Equal(t, testCase.expectedRes, err)
		})
	}
}

func TestDel(t *testing.T) {
	testCases := []struct {
		name        string
		args        invoke.Args
		k8sArgs     argtypes.K8sArgs
		expectedRes error
	}{
		{
			name:        "Test Del",
			args:        invoke.Args{Path: defaultCNIPath},
			expectedRes: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			execPluginWithResultFunc = fakeExecWithResult
			execPluginNoResultFunc = fakeExecNoResult
			d := &IPAMDelegator{pluginType: ipamHostLocal}
			_, err := d.Del(&testCase.args, &testCase.k8sArgs, testNetworkConfig)
			assert.Equal(t, testCase.expectedRes, err)
		})
	}
}

func TestCheck(t *testing.T) {
	testCases := []struct {
		name        string
		args        invoke.Args
		k8sArgs     argtypes.K8sArgs
		expectedRes error
	}{
		{
			name:        "Test Check",
			args:        invoke.Args{Path: defaultCNIPath},
			expectedRes: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			execPluginWithResultFunc = fakeExecWithResult
			execPluginNoResultFunc = fakeExecNoResult
			d := &IPAMDelegator{pluginType: ipamHostLocal}
			_, err := d.Check(&testCase.args, &testCase.k8sArgs, testNetworkConfig)
			assert.Equal(t, testCase.expectedRes, err)
		})
	}
}

func TestDelegateWithResult(t *testing.T) {
	testCases := []struct {
		name        string
		args        invoke.Args
		expectedRes error
	}{
		{
			name:        "Test delegateWithResult",
			args:        invoke.Args{Path: defaultCNIPath},
			expectedRes: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			execPluginWithResultFunc = fakeExecWithResult
			_, err := delegateWithResult(ipamHostLocal, testNetworkConfig, &testCase.args)
			assert.Equal(t, testCase.expectedRes, err)
		})
	}
}

func TestDelegateNoResult(t *testing.T) {
	testCases := []struct {
		name        string
		args        invoke.Args
		expectedRes error
	}{
		{
			name:        "Test delegateNoResult",
			args:        invoke.Args{Path: defaultCNIPath},
			expectedRes: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			execPluginNoResultFunc = fakeExecNoResult
			assert.Equal(t, testCase.expectedRes, delegateNoResult(ipamHostLocal, testNetworkConfig, &testCase.args))
		})
	}
}
