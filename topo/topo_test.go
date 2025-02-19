// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package topo

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/h-fam/errdiff"
	tfake "github.com/openconfig/kne/api/clientset/v1beta1/fake"
	topologyv1 "github.com/openconfig/kne/api/types/v1beta1"
	cpb "github.com/openconfig/kne/proto/controller"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/kne/topo/node"
	nd "github.com/openconfig/kne/topo/node"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestLoad(t *testing.T) {
	type args struct {
		fName string
	}

	invalidPb, err := os.CreateTemp(".", "invalid*.pb.txt")
	if err != nil {
		t.Errorf("failed creating tmp pb: %v", err)
	}
	defer os.Remove(invalidPb.Name())

	invalidYaml, err := os.CreateTemp(".", "invalid*.yaml")
	if err != nil {
		t.Errorf("failed creating tmp yaml: %v", err)
	}
	defer os.Remove(invalidYaml.Name())

	invalidPb.WriteString(`
	name: "2node-ixia"
	nodes: {
		nme: "ixia-c-port1"
	}
	`)

	invalidYaml.WriteString(`
	name: 2node-ixia
	nodes:
	  - name: ixia-c-port1
	`)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "pb", args: args{fName: "../examples/2node-ixia-ceos.pb.txt"}, wantErr: false},
		{name: "yaml", args: args{fName: "../examples/2node-ixia-ceos.yaml"}, wantErr: false},
		{name: "invalid-pb", args: args{fName: invalidPb.Name()}, wantErr: true},
		{name: "invalid-yaml", args: args{fName: invalidYaml.Name()}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.args.fName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

var (
	validPbTxt = `
name: "test-data-topology"
nodes: {
  name: "r1"
  type: ARISTA_CEOS
  services: {
	key: 1002
	value: {
  	  name: "ssh"
	  inside: 1002
	  outside: 22
	  inside_ip: "1.1.1.2"
	  outside_ip: "100.100.100.101"
	  node_port: 22
	}
  }
}
nodes: {
    name: "otg"
    type: IXIA_TG
    version: "0.0.1-9999"
    services: {
        key: 40051
        value: {
            name: "grpc"
            inside: 40051
			inside_ip: "1.1.1.1"
			outside_ip: "100.100.100.100"
			node_port: 20001
        }
    }
    services: {
        key: 50051
        value: {
            name: "gnmi"
            inside: 50051
			inside_ip: "1.1.1.1"
			outside_ip: "100.100.100.100"
			node_port: 20000
        }
    }
}
links: {
  a_node: "r1"
  a_int: "eth9"
  z_node: "otg"
  z_int: "eth1"
}
`
)

// defaultFakeTopology serves as a testing fake with default implementation.
type defaultFakeTopology struct{}

func (f *defaultFakeTopology) Load(context.Context) error {
	return nil
}

func (f *defaultFakeTopology) Topology(context.Context) ([]topologyv1.Topology, error) {
	return nil, nil
}

func (f *defaultFakeTopology) TopologyProto() *tpb.Topology {
	return nil
}

func (f *defaultFakeTopology) Push(context.Context) error {
	return nil
}

func (f *defaultFakeTopology) CheckNodeStatus(context.Context, time.Duration) error {
	return nil
}

func (f *defaultFakeTopology) Delete(context.Context) error {
	return nil
}

func (f *defaultFakeTopology) Nodes() []node.Node {
	return nil
}

func (f *defaultFakeTopology) Resources(context.Context) (*Resources, error) {
	return nil, nil
}

func (f *defaultFakeTopology) Watch(context.Context) error {
	return nil
}

func (f *defaultFakeTopology) ConfigPush(context.Context, string, io.Reader) error {
	return nil
}

func (f *defaultFakeTopology) Node(string) (node.Node, error) {
	return nil, nil
}

func (f *defaultFakeTopology) TopologySpecs(context.Context) ([]*topologyv1.Topology, error) {
	return nil, nil
}

func (f *defaultFakeTopology) TopologyResources(context.Context) ([]*topologyv1.Topology, error) {
	return nil, nil
}

func TestCreateTopology(t *testing.T) {
	tf, err := tfake.NewSimpleClientset()
	if err != nil {
		t.Fatalf("cannot create fake topology clientset")
	}
	opts := []Option{
		WithClusterConfig(&rest.Config{}),
		WithKubeClient(kfake.NewSimpleClientset()),
		WithTopoClient(tf),
	}

	tests := []struct {
		desc       string
		inputParam TopologyParams
		wantErr    string
	}{{
		desc: "create with valid topology file",
		inputParam: TopologyParams{
			TopoName:       "testdata/valid_topo.pb.txt",
			TopoNewOptions: opts,
			DryRun:         true,
		},
		wantErr: "",
	}, {
		desc: "create with non-existent topology file",
		inputParam: TopologyParams{
			TopoName:       "testdata/non_existing.pb.txt",
			TopoNewOptions: opts,
			DryRun:         true,
		},
		wantErr: "no such file or directory",
	}, {
		desc: "create with invalid topology",
		inputParam: TopologyParams{
			TopoName:       "testdata/invalid_topo.pb.txt",
			TopoNewOptions: opts,
			DryRun:         true,
		},
		wantErr: "invalid topology",
	},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := CreateTopology(context.Background(), tc.inputParam)
			if diff := errdiff.Check(err, tc.wantErr); diff != "" {
				t.Fatalf("failed: %+v", err)
			}
		})
	}
}

func TestDeleteTopology(t *testing.T) {
	tf, err := tfake.NewSimpleClientset()
	if err != nil {
		t.Fatalf("cannot create fake topology clientset")
	}
	opts := []Option{
		WithClusterConfig(&rest.Config{}),
		WithKubeClient(kfake.NewSimpleClientset()),
		WithTopoClient(tf),
	}

	tests := []struct {
		desc       string
		inputParam TopologyParams
		wantErr    string
	}{{
		desc: "delete a non-existing topology with valid topology file",
		inputParam: TopologyParams{
			TopoName:       "testdata/valid_topo.pb.txt",
			TopoNewOptions: opts,
			DryRun:         true,
		},
		wantErr: "does not exist in cluster",
	}, {
		desc: "delete with non-existent topology file",
		inputParam: TopologyParams{
			TopoName:       "testdata/non_existing.pb.txt",
			TopoNewOptions: opts,
			DryRun:         true,
		},
		wantErr: "no such file or directory",
	}, {
		desc: "delete with invalid topology",
		inputParam: TopologyParams{
			TopoName:       "testdata/invalid_topo.pb.txt",
			TopoNewOptions: opts,
			DryRun:         true,
		},
		wantErr: "invalid topology",
	},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := DeleteTopology(context.Background(), tc.inputParam)
			if diff := errdiff.Check(err, tc.wantErr); diff != "" {
				t.Fatalf("failed: %+v", err)
			}
		})
	}
}

// fakeTopology is used to test GetTopologyServices().
type fakeTopology struct {
	defaultFakeTopology
	resources *Resources
	proto     *tpb.Topology
	rErr      error
	lErr      error
}

func (f *fakeTopology) Load(context.Context) error {
	return f.lErr
}

func (f *fakeTopology) TopologyProto() *tpb.Topology {
	return f.proto
}

func (f *fakeTopology) Resources(context.Context) (*Resources, error) {
	return f.resources, f.rErr
}

func TestGetTopologyServices(t *testing.T) {
	tf, err := tfake.NewSimpleClientset()
	if err != nil {
		t.Fatalf("cannot create fake topology clientset")
	}
	opts := []Option{
		WithClusterConfig(&rest.Config{}),
		WithKubeClient(kfake.NewSimpleClientset()),
		WithTopoClient(tf),
	}

	validTopoIn, err := Load("testdata/valid_topo.pb.txt")
	if err != nil {
		t.Fatalf("cannot load the valid topoplogy proto as input: %v", err)
	}
	validTopoOut := &tpb.Topology{}
	if err := prototext.Unmarshal([]byte(validPbTxt), validTopoOut); err != nil {
		t.Fatalf("cannot Unmarshal validTopo: %v", err)
	}
	tests := []struct {
		desc        string
		inputParam  TopologyParams
		topoNewFunc func(string, *tpb.Topology, ...Option) (TopologyManager, error)
		want        *tpb.Topology
		wantErr     string
	}{
		{
			desc: "load topology error",
			inputParam: TopologyParams{
				TopoName:       "testdata/not_there.pb.txt",
				TopoNewOptions: opts,
			},
			wantErr: "no such file or directory",
		},
		{
			desc: "empty resources",
			topoNewFunc: func(string, *tpb.Topology, ...Option) (TopologyManager, error) {
				return &fakeTopology{
					proto:     validTopoIn,
					resources: &Resources{},
				}, nil
			},
			wantErr: "not found",
		},
		{
			desc: "load fail",
			topoNewFunc: func(string, *tpb.Topology, ...Option) (TopologyManager, error) {
				return &fakeTopology{
					lErr:      fmt.Errorf("load failed"),
					resources: &Resources{},
				}, nil
			},
			wantErr: "load failed",
		},
		{
			desc: "valid case",
			topoNewFunc: func(string, *tpb.Topology, ...Option) (TopologyManager, error) {
				return &fakeTopology{
					proto: validTopoIn,
					resources: &Resources{
						Services: map[string][]*corev1.Service{
							"otg": {
								{
									Spec: corev1.ServiceSpec{
										ClusterIP: "1.1.1.1",
										Ports: []corev1.ServicePort{{
											Port:     40051,
											NodePort: 20001,
											Name:     "grpc",
										}},
									},
									Status: corev1.ServiceStatus{
										LoadBalancer: corev1.LoadBalancerStatus{
											Ingress: []corev1.LoadBalancerIngress{{IP: "100.100.100.100"}},
										},
									},
								},
								{
									Spec: corev1.ServiceSpec{
										ClusterIP: "1.1.1.1",
										Ports: []corev1.ServicePort{{
											Port:     50051,
											NodePort: 20000,
											Name:     "gnmi",
										}},
									},
									Status: corev1.ServiceStatus{
										LoadBalancer: corev1.LoadBalancerStatus{
											Ingress: []corev1.LoadBalancerIngress{{IP: "100.100.100.100"}},
										},
									},
								},
							},
							"r1": {
								{
									Spec: corev1.ServiceSpec{
										ClusterIP: "1.1.1.2",
										Ports: []corev1.ServicePort{{
											Port:       1002,
											NodePort:   22,
											Name:       "ssh",
											TargetPort: intstr.IntOrString{IntVal: 22},
										}},
									},
									Status: corev1.ServiceStatus{
										LoadBalancer: corev1.LoadBalancerStatus{
											Ingress: []corev1.LoadBalancerIngress{{IP: "100.100.100.101"}},
										},
									},
								},
							},
						},
					},
				}, nil
			},
			wantErr: "",
			want:    validTopoOut,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.topoNewFunc != nil {
				origNew := new
				new = tc.topoNewFunc
				defer func() {
					new = origNew
				}()
			}
			got, err := GetTopologyServices(context.Background(), tc.inputParam)
			if diff := errdiff.Check(err, tc.wantErr); diff != "" {
				t.Fatalf("get topology service returned unexpected error: gotErr: %+v, wantErr: %+v", err, tc.wantErr)
			}
			if tc.wantErr != "" {
				return
			}
			if s := cmp.Diff(got.Topology, tc.want, protocmp.Transform()); s != "" {
				t.Fatalf("get topology service failed: %s", s)
			}
		})
	}
}

func TestStateMap(t *testing.T) {
	type node struct {
		name  string
		phase nd.Status
	}

	tests := []struct {
		desc  string
		nodes []*node
		want  cpb.TopologyState
	}{{
		desc: "no nodes",
		want: cpb.TopologyState_TOPOLOGY_STATE_UNSPECIFIED,
	}, {
		desc: "one node failed",
		nodes: []*node{
			{"n1", nd.StatusFailed},
			{"n2", nd.StatusRunning},
			{"n3", nd.StatusRunning},
		},
		want: cpb.TopologyState_TOPOLOGY_STATE_ERROR,
	}, {
		desc: "one node failed with one node pending",
		nodes: []*node{
			{"n1", nd.StatusFailed},
			{"n2", nd.StatusRunning},
			{"n3", nd.StatusRunning},
		},
		want: cpb.TopologyState_TOPOLOGY_STATE_ERROR,
	}, {
		desc: "one node failed, one node pending, one node unknown",
		nodes: []*node{
			{"n1", nd.StatusFailed},
			{"n2", nd.StatusPending},
			{"n3", nd.StatusUnknown},
		},
		want: cpb.TopologyState_TOPOLOGY_STATE_ERROR,
	}, {
		desc: "all nodes failed",
		nodes: []*node{
			{"n1", nd.StatusFailed},
			{"n2", nd.StatusFailed},
			{"n3", nd.StatusFailed},
		},
		want: cpb.TopologyState_TOPOLOGY_STATE_ERROR,
	}, {
		desc: "one node pending",
		nodes: []*node{
			{"n1", nd.StatusPending},
			{"n2", nd.StatusRunning},
			{"n3", nd.StatusRunning},
		},
		want: cpb.TopologyState_TOPOLOGY_STATE_CREATING,
	},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			sm := &sMap{}
			for _, n := range tc.nodes {
				sm.SetNodeState(n.name, n.phase)
			}
			got := sm.TopoState()
			if tc.want != got {
				t.Fatalf("want: %+v, got: %+v", tc.want, got)
			}
		})
	}
}
