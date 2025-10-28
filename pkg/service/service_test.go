package service

import (
	"context"
	"testing"

	rfmv1 "github.com/joeyloman/rancher-fip-manager/pkg/apis/rancher.k8s.binbash.org/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

func TestValidateFloatingIP(t *testing.T) {
	ipAddr := "192.168.1.100"
	fipPool := &rfmv1.FloatingIPPool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rancher.k8s.binbash.org/v1beta1",
			Kind:       "FloatingIPPool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pool",
		},
		Spec: rfmv1.FloatingIPPoolSpec{
			IPConfig: &rfmv1.IPConfig{
				Subnet: "192.168.1.0/24",
				Pool: rfmv1.Pool{
					Exclude: []string{"192.168.1.101"},
				},
			},
		},
		Status: rfmv1.FloatingIPPoolStatus{
			Allocated: map[string]string{
				"192.168.1.102": "default/another-fip",
			},
			Available: 1,
		},
	}
	plbc := &rfmv1.FloatingIPProjectQuota{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rancher.k8s.binbash.org/v1beta1",
			Kind:       "FloatingIPProjectQuota",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-project",
		},
		Spec: rfmv1.FloatingIPProjectQuotaSpec{
			FloatingIPQuota: map[string]int{
				"test-pool": 1,
			},
		},
		Status: rfmv1.FloatingIPProjectQuotaStatus{
			FloatingIPs: map[string]*rfmv1.FipInfo{
				"test-pool": {
					Used: 0,
				},
			},
		},
	}
	fip := &rfmv1.FloatingIP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fip",
			Namespace: "default",
			Labels: map[string]string{
				"rancher.k8s.binbash.org/project-name": "test-project",
			},
		},
		Spec: rfmv1.FloatingIPSpec{
			FloatingIPPool: "test-pool",
		},
	}

	testCases := []struct {
		name            string
		fip             *rfmv1.FloatingIP
		existingPools   []runtime.Object
		existingPLBCs   []runtime.Object
		expectedAllowed bool
		expectedMessage string
	}{
		{
			name:            "pool does not exist",
			fip:             fip,
			existingPools:   []runtime.Object{},
			existingPLBCs:   []runtime.Object{plbc},
			expectedAllowed: false,
			expectedMessage: "the specified floatingippool test-pool does not exist",
		},
		{
			name: "invalid ip address",
			fip: &rfmv1.FloatingIP{
				ObjectMeta: fip.ObjectMeta,
				Spec: rfmv1.FloatingIPSpec{
					FloatingIPPool: "test-pool",
					IPAddr:         new(string), // empty string will be invalid
				},
			},
			existingPools:   []runtime.Object{fipPool},
			existingPLBCs:   []runtime.Object{plbc},
			expectedAllowed: false,
			expectedMessage: "invalid IP address format: ",
		},
		{
			name: "ip not in subnet",
			fip: &rfmv1.FloatingIP{
				ObjectMeta: fip.ObjectMeta,
				Spec: rfmv1.FloatingIPSpec{
					FloatingIPPool: "test-pool",
					IPAddr:         func() *string { s := "192.168.2.1"; return &s }(),
				},
			},
			existingPools:   []runtime.Object{fipPool},
			existingPLBCs:   []runtime.Object{plbc},
			expectedAllowed: false,
			expectedMessage: "requested IP 192.168.2.1 is not in the subnet range 192.168.1.0/24",
		},
		{
			name: "ip in exclude list",
			fip: &rfmv1.FloatingIP{
				ObjectMeta: fip.ObjectMeta,
				Spec: rfmv1.FloatingIPSpec{
					FloatingIPPool: "test-pool",
					IPAddr:         func() *string { s := "192.168.1.101"; return &s }(),
				},
			},
			existingPools:   []runtime.Object{fipPool},
			existingPLBCs:   []runtime.Object{plbc},
			expectedAllowed: false,
			expectedMessage: "requested IP 192.168.1.101 is in the exclude list",
		},
		{
			name: "ip already allocated",
			fip: &rfmv1.FloatingIP{
				ObjectMeta: fip.ObjectMeta,
				Spec: rfmv1.FloatingIPSpec{
					FloatingIPPool: "test-pool",
					IPAddr:         func() *string { s := "192.168.1.102"; return &s }(),
				},
			},
			existingPools:   []runtime.Object{fipPool},
			existingPLBCs:   []runtime.Object{plbc},
			expectedAllowed: false,
			expectedMessage: "requested IP 192.168.1.102 is already allocated",
		},
		{
			name: "pool is full",
			fip: &rfmv1.FloatingIP{
				ObjectMeta: fip.ObjectMeta,
				Spec: rfmv1.FloatingIPSpec{
					FloatingIPPool: "test-pool",
				},
			},
			existingPools: []runtime.Object{
				&rfmv1.FloatingIPPool{
					TypeMeta:   fipPool.TypeMeta,
					ObjectMeta: fipPool.ObjectMeta,
					Spec:       fipPool.Spec,
					Status: rfmv1.FloatingIPPoolStatus{
						Available: 0,
					},
				},
			},
			existingPLBCs:   []runtime.Object{plbc},
			expectedAllowed: false,
			expectedMessage: "no available IPs in floatingippool test-pool",
		},
		{
			name:          "no quota defined",
			fip:           fip,
			existingPools: []runtime.Object{fipPool},
			existingPLBCs: []runtime.Object{
				&rfmv1.FloatingIPProjectQuota{
					TypeMeta:   plbc.TypeMeta,
					ObjectMeta: plbc.ObjectMeta,
					Spec:       rfmv1.FloatingIPProjectQuotaSpec{}, // no quota
				},
			},
			expectedAllowed: false,
			expectedMessage: "no quota defined for floatingippool test-pool in project test-project",
		},
		{
			name:          "quota exceeded",
			fip:           fip,
			existingPools: []runtime.Object{fipPool},
			existingPLBCs: []runtime.Object{
				&rfmv1.FloatingIPProjectQuota{
					TypeMeta:   plbc.TypeMeta,
					ObjectMeta: plbc.ObjectMeta,
					Spec:       plbc.Spec,
					Status: rfmv1.FloatingIPProjectQuotaStatus{
						FloatingIPs: map[string]*rfmv1.FipInfo{
							"test-pool": {
								Used: 1, // quota is 1
							},
						},
					},
				},
			},
			expectedAllowed: false,
			expectedMessage: "quota exceeded for floatingippool test-pool in project test-project. Quota: 1, Used: 1",
		},
		{
			name: "valid request",
			fip: &rfmv1.FloatingIP{
				ObjectMeta: fip.ObjectMeta,
				Spec: rfmv1.FloatingIPSpec{
					FloatingIPPool: "test-pool",
					IPAddr:         &ipAddr,
				},
			},
			existingPools:   []runtime.Object{fipPool},
			existingPLBCs:   []runtime.Object{plbc},
			expectedAllowed: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					UID: "test-uid",
				},
			}
			unstructuredPools, _ := LomanJoeyUnstructuredList(tc.existingPools)
			unstructuredPLBCs, _ := LomanJoeyUnstructuredList(tc.existingPLBCs)

			dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), append(unstructuredPools, unstructuredPLBCs...)...)

			response := validateFloatingIP(context.Background(), dynamicClient, ar, tc.fip, nil)

			assert.Equal(t, tc.expectedAllowed, response.Allowed)
			if !tc.expectedAllowed {
				assert.Equal(t, tc.expectedMessage, response.Result.Message)
			}
		})
	}
}

func LomanJoeyUnstructuredList(objects []runtime.Object) ([]runtime.Object, error) {
	unstructuredList := []runtime.Object{}
	for _, obj := range objects {
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, err
		}
		unstructuredList = append(unstructuredList, &unstructured.Unstructured{Object: unstructuredMap})
	}
	return unstructuredList, nil
}
