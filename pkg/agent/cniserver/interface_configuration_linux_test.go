//go:build linux
// +build linux

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

package cniserver

import (
	"fmt"
	"net"
	"testing"

	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

var (
	fakeGetNSDevInterfaceFunc = func(nsPath, dev string) (*net.Interface, error) {
		return &net.Interface{}, nil
	}
	fakeGetNSPeerDevBridgeFunc = func(nsPath, dev string) (*net.Interface, string, error) {
		return &net.Interface{}, "", nil
	}
	fakeGetUplinkRepresentorFunc = func(pciAddress string) (string, error) {
		return "", nil
	}
	fakeGetVfIndexByPciAddressFunc = func(vfPciAddress string) (int, error) {
		return 0, nil
	}
	fakeGetVfRepresentorFunc = func(uplink string, vfIndex int) (string, error) {
		return "", nil
	}
)

func TestValidateVFRepInterface(t *testing.T) {
	testCases := []struct {
		name                       string
		getUplinkRepresentorFunc   func(pciAddress string) (string, error)
		getVfIndexByPciAddressFunc func(vfPciAddress string) (int, error)
		getVfRepresentorFunc       func(uplink string, vfIndex int) (string, error)
		expectedRes                error
	}{
		{
			name:        "Failed to get uplink representor",
			expectedRes: fmt.Errorf("failed to get uplink representor for PCI Address deviceID"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.getUplinkRepresentorFunc != nil {
				getUplinkRepresentorFunc = tt.getUplinkRepresentorFunc
			}
			if tt.getVfIndexByPciAddressFunc != nil {
				getVfIndexByPciAddressFunc = tt.getVfIndexByPciAddressFunc
			}
			if tt.getVfRepresentorFunc != nil {
				getVfRepresentorFunc = tt.getVfRepresentorFunc
			}
		})

		ic := &ifConfigurator{}
		_, err := ic.validateVFRepInterface("deviceID")
		assert.Equal(t, tt.expectedRes, err)
	}
}

func TestValidateContainerPeerInterface(t *testing.T) {
	testCases := []struct {
		name          string
		interfaces    []*current.Interface
		containerVeth *vethPair
		expectedRes   error
	}{
		{
			name: "No interface for containerVeth",
			containerVeth: &vethPair{
				name: "veth",
			},
			expectedRes: fmt.Errorf("peer veth interface not found for container interface %s", "veth"),
		},
		{
			name: "Host interface's sandbox is missing",
			interfaces: []*current.Interface{
				&current.Interface{},
			},
			containerVeth: &vethPair{
				name: "veth",
			},
			expectedRes: fmt.Errorf("peer veth interface not found for container interface %s", "veth"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ic := &ifConfigurator{}
			_, err := ic.validateContainerPeerInterface(tt.interfaces, tt.containerVeth)
			assert.Equal(t, tt.expectedRes, err)
		})
	}
}

func TestGetInterceptedInterfaces(t *testing.T) {
	testCases := []struct {
		name                   string
		getNSDevInterfaceFunc  func(nsPath, dev string) (*net.Interface, error)
		getNSPeerDevBridgeFunc func(nsPath, dev string) (*net.Interface, string, error)
		expectedRes            error
	}{
		{
			name:                   "Test get intercepted interfaces",
			getNSDevInterfaceFunc:  fakeGetNSDevInterfaceFunc,
			getNSPeerDevBridgeFunc: fakeGetNSPeerDevBridgeFunc,
			expectedRes:            nil,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ic := &ifConfigurator{}
			getNSDevInterfaceFunc = tt.getNSDevInterfaceFunc
			getNSPeerDevBridgeFunc = tt.getNSPeerDevBridgeFunc
			_, _, err := ic.getInterceptedInterfaces("sandbox", "netns", "ifdev")
			assert.Equal(t, tt.expectedRes, err)
		})
	}
}

func TestValidateInterface(t *testing.T) {
	testCases := []struct {
		name           string
		intf           *current.Interface
		inNetns        bool
		ifType         string
		linkByNameFunc func(name string) (netlink.Link, error)
		expectedRes    error
	}{
		{
			name:        "Test interface name is empty",
			intf:        &current.Interface{},
			expectedRes: fmt.Errorf("interface name is missing"),
		},
		{
			name: "Test interface sandbox is empty and inNetns is true",
			intf: &current.Interface{
				Name: "interface",
			},
			inNetns:     true,
			expectedRes: fmt.Errorf("interface interface is expected in netns"),
		},
		{
			name: "Test interface sandbox is not empty and inNetns is false",
			intf: &current.Interface{
				Name:    "interface",
				Sandbox: "sandbox",
			},
			inNetns:     false,
			expectedRes: fmt.Errorf("interface interface is expected not in netns"),
		},
		{
			name: "Test cannot find interface",
			intf: &current.Interface{
				Name: "interface",
			},
			inNetns:     false,
			expectedRes: fmt.Errorf("failed to find link for interface interface"),
		},
		{
			name: "Test interface type is netDeviceTypeVeth",
			intf: &current.Interface{
				Name: "interface",
			},
			inNetns: false,
			ifType:  netDeviceTypeVeth,
			linkByNameFunc: func(name string) (netlink.Link, error) {
				return &netlink.Veth{}, nil
			},
			expectedRes: fmt.Errorf("unknown device type %s", "veth"),
		},
		{
			name: "Test interface type is vf",
			intf: &current.Interface{
				Name: "interface",
			},
			ifType:  netDeviceTypeVF,
			inNetns: false,
			linkByNameFunc: func(name string) (netlink.Link, error) {
				return &netlink.Veth{}, nil
			},
			expectedRes: nil,
		},
		{
			name: "Test known interface type",
			intf: &current.Interface{
				Name: "interface",
			},
			inNetns: false,
			ifType:  "known",
			linkByNameFunc: func(name string) (netlink.Link, error) {
				return &netlink.Veth{}, nil
			},
			expectedRes: fmt.Errorf("unknown device type %s", "known"),
		},
	}

	for _, tt := range testCases {
		if tt.linkByNameFunc != nil {
			linkByNameFunc = tt.linkByNameFunc
		}
		_, err := validateInterface(tt.intf, tt.inNetns, tt.ifType)
		assert.Equal(t, tt.expectedRes, err)
	}
}

func TestIsVeth(t *testing.T) {
	testCases := []struct {
		name        string
		link        netlink.Link
		exceptedRes bool
	}{
		{
			name:        "Test is veth",
			link:        &netlink.Veth{},
			exceptedRes: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.exceptedRes, isVeth(tt.link))
		})
	}
}
func TestGetOVSInterfaceType(t *testing.T) {
	testCases := []struct {
		name        string
		expectedRes int
	}{
		{
			name:        "Test getOVSInterfaceType",
			expectedRes: 0,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ic := &ifConfigurator{}
			assert.Equal(t, tt.expectedRes, ic.getOVSInterfaceType("name"))
		})
	}
}
