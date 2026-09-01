package dto

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func bind(t *testing.T, body string, out any) error {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return binding.JSON.Bind(req, out)
}

// TestNumericFieldsAcceptZero is the reason the numeric request fields are
// pointers. Gin's `required` treats a plain int's zero value as missing, which
// would reject a free item, an out-of-stock item, or a price correction to 0.
// On a pointer it only rejects an absent field, which is what @NotNull meant.
func TestNumericFieldsAcceptZero(t *testing.T) {
	var add AddResourceRequest
	body := `{"resourceName":"Semen","resourceDescription":"50kg",
	          "resourceStock":0,"resourcePrice":0,"resourceSupplierId":"x"}`
	if err := bind(t, body, &add); err != nil {
		t.Fatalf("zero stock/price rejected: %v", err)
	}
	if *add.ResourceStock != 0 || *add.ResourcePrice != 0 {
		t.Errorf("stock/price = %d/%d, want 0/0", *add.ResourceStock, *add.ResourcePrice)
	}

	var update UpdateResourceRequest
	if err := bind(t, `{"resourceDescription":"x","resourcePrice":0,"resourceStock":0}`, &update); err != nil {
		t.Fatalf("zero stock/price rejected on update: %v", err)
	}
}

// TestMissingFieldsRejected is the other half: an omitted field must still fail.
func TestMissingFieldsRejected(t *testing.T) {
	cases := map[string]struct {
		body string
		into func() any
	}{
		"add without stock": {
			`{"resourceName":"Semen","resourceDescription":"50kg","resourcePrice":1,"resourceSupplierId":"x"}`,
			func() any { return &AddResourceRequest{} },
		},
		"add without a supplier": {
			`{"resourceName":"Semen","resourceDescription":"50kg","resourceStock":1,"resourcePrice":1}`,
			func() any { return &AddResourceRequest{} },
		},
		"update without price": {
			`{"resourceDescription":"x","resourceStock":1}`,
			func() any { return &UpdateResourceRequest{} },
		},
		"stock change without a quantity": {
			`{}`,
			func() any { return &UpdateResourceStockRequest{} },
		},
		"supplier link without a supplier": {
			`{"resourceId":[1,2]}`,
			func() any { return &AddSupplierIDRequest{} },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := bind(t, tc.body, tc.into()); err == nil {
				t.Error("binding accepted a body with a required field missing")
			}
		})
	}
}

// TestEmptyResourceListAccepted keeps `update-supplier` able to detach every
// resource from a supplier — profile sends an empty list to do that, so the
// field must not be required.
func TestEmptyResourceListAccepted(t *testing.T) {
	var req AddSupplierIDRequest
	if err := bind(t, `{"supplierId":"abc","resourceId":[]}`, &req); err != nil {
		t.Fatalf("empty resource list rejected: %v", err)
	}
	if len(req.ResourceID) != 0 {
		t.Errorf("resourceId = %v, want empty", req.ResourceID)
	}
}
