// Copyright 2020 The Lokomotive Authors
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

package k8sutil

import (
	"context"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kinvolk/lokomotive/internal"
)

func TestParseManifests(t *testing.T) {
	t.Parallel()

	networkPolicy := map[string]string{
		"templates/test-deny-metadata.yml": `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-deny-metadata
  namespace: test-namespace
  spec:
    podSelector: {}
    policyTypes:
    - Egress
    egress:
    - to:
      - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - 169.254.142.0/24
`,
	}
	networkPolicyManifest := []manifest{
		{
			kind:       "NetworkPolicy",
			apiVersion: "networking.k8s.io/v1",
			namespace:  "test-namespace",
			name:       "test-deny-metadata",
		},
	}

	twoResources := map[string]string{
		"templates/test-two-resources.yml": `
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: test-ingress
  namespace: test-namespace
spec:
  rules:
  - http:
      paths:
      - path: /testpath
        backend:
          serviceName: test
          servicePort: 80
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: a-config
  namespace: default
  data:
    color: \"red\"
    multi-line: |
      hello world
      how are you?
`,
	}
	twoResourcesManifest := []manifest{
		{
			kind:       "Ingress",
			apiVersion: "networking.k8s.io/v1beta1",
			namespace:  "test-namespace",
			name:       "test-ingress",
		},
		{
			kind:       "ConfigMap",
			apiVersion: "v1",
			namespace:  "default",
			name:       "a-config",
		},
	}

	tests := []struct {
		name string
		raw  map[string]string
		want []manifest
	}{
		{
			name: "ingress",
			raw:  networkPolicy,
			want: networkPolicyManifest,
		},
		{
			name: "two-resources",
			raw:  twoResources,
			want: twoResourcesManifest,
		},
		{
			name: "empty-file",
			raw: map[string]string{
				"foo.yaml": ``,
			},
			want: nil,
		},
		{
			name: "file-with-whitespace",
			raw: map[string]string{
				"foo.yaml": `   `,
			},
			want: nil,
		},
		{
			name: "empty-yaml-with-comments",
			raw: map[string]string{
				"foo.yaml": `# Optional deployment from helm chart`,
			},
			want: nil,
		},
		{
			name: "List of resources",
			raw: map[string]string{
				"prometheus-operator/templates/prometheus/rolebinding-specificNamespace.yaml": `
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBindingList
items:
- apiVersion: rbac.authorization.k8s.io/v1
  kind: RoleBinding
  metadata:
    name: prometheus-operator-prometheus
    labels:
      app: prometheus-operator-prometheus
    namespace: "kube-system"
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: Role
    name: prometheus-operator-prometheus
  subjects:
  - kind: ServiceAccount
    name: prometheus-operator-prometheus
    namespace: default
- apiVersion: rbac.authorization.k8s.io/v1
  kind: RoleBinding
  metadata:
    name: prometheus-operator-prometheus
    labels:
      app: prometheus-operator-prometheus
    namespace: "default"
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: Role
    name: prometheus-operator-prometheus
  subjects:
  - kind: ServiceAccount
    name: prometheus-operator-prometheus
    namespace: default
`,
			},
			want: []manifest{
				{
					kind:       "RoleBinding",
					apiVersion: "rbac.authorization.k8s.io/v1",
					namespace:  "kube-system",
					name:       "prometheus-operator-prometheus",
				},
				{
					kind:       "RoleBinding",
					apiVersion: "rbac.authorization.k8s.io/v1",
					namespace:  "default",
					name:       "prometheus-operator-prometheus",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := LoadManifests(test.raw)
			if err != nil {
				t.Fatalf("failed to parse manifest: %v", err)
			}
			for i := range got {
				got[i].raw = nil
			}
			if !reflect.DeepEqual(test.want, got) {
				t.Errorf("wanted %#v, got %#v", test.want, got)
			}
		})
	}

}

func TestCreateOrUpdateNamespaceEmptyName(t *testing.T) {
	nsclient := fake.NewSimpleClientset().CoreV1().Namespaces()

	name := ""
	ns := Namespace{
		Name:   name,
		Labels: internal.AppendNamespaceLabel(name, map[string]string{}),
	}

	if err := CreateOrUpdateNamespace(ns, nsclient); err == nil {
		t.Fatal("namespace name cannot be empty, expected error")
	}
}

func TestCreateOrUpdateNamespaceCreateSuccess(t *testing.T) {
	nsclient := fake.NewSimpleClientset().CoreV1().Namespaces()

	name := "test"
	ns := Namespace{
		Name:   name,
		Labels: internal.AppendNamespaceLabel(name, map[string]string{}),
	}

	if err := CreateOrUpdateNamespace(ns, nsclient); err != nil {
		t.Fatalf("expected nil in namespace create, got: %v", err)
	}

	// Get Namespace to confirm.
	mockns, getErr := nsclient.Get(context.TODO(), name, metav1.GetOptions{})
	if getErr != nil {
		t.Fatalf("expected nil, got: %v", getErr)
	}

	if mockns.ObjectMeta.Name != name {
		t.Fatalf("expected namespace %q, got: %q", mockns.ObjectMeta.Name, name)
	}

	if mockns.ObjectMeta.Labels[internal.NamespaceLabelKey] != name {
		t.Fatalf("expected %q, got: %q", name, mockns.ObjectMeta.Labels[internal.NamespaceLabelKey])
	}
}

func TestCreateOrUpdateNamespaceUpdateSuccess(t *testing.T) {
	nsclient := fake.NewSimpleClientset().CoreV1().Namespaces()

	name := "test"

	_, createErr := nsclient.Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, metav1.CreateOptions{})
	if createErr != nil {
		t.Fatalf("expected nil, got: %v", createErr)
	}

	ns := Namespace{
		Name:   name,
		Labels: internal.AppendNamespaceLabel("test", map[string]string{}),
	}

	if err := CreateOrUpdateNamespace(ns, nsclient); err != nil {
		t.Fatalf("expected nil in namespace update, got: %v", err)
	}

	// Get Namespace to confirm.
	mockns, getErr := nsclient.Get(context.TODO(), name, metav1.GetOptions{})
	if getErr != nil {
		t.Fatalf("expected nil, got: %v", getErr)
	}

	if mockns.ObjectMeta.Name != name {
		t.Fatalf("expected namespace %q, got: %q", mockns.ObjectMeta.Name, name)
	}

	if mockns.ObjectMeta.Labels[internal.NamespaceLabelKey] != name {
		t.Fatalf("expected %q, got: %q", name, mockns.ObjectMeta.Labels[internal.NamespaceLabelKey])
	}
}
