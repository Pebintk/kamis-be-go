package model

import "testing"

func TestRoleCasingRoundTrip(t *testing.T) {
	cases := []struct {
		discriminator, authority, api string
	}{
		{"ADMIN", "Admin", "admin"},
		{"OPERASIONAL", "Operasional", "operasional"},
		{"FINANCE", "Finance", "finance"},
		{"DIREKSI", "Direksi", "direksi"},
	}
	for _, tc := range cases {
		if got := AuthorityFromDiscriminator(tc.discriminator); got != tc.authority {
			t.Errorf("Authority(%q) = %q, want %q", tc.discriminator, got, tc.authority)
		}
		u := EndUser{UserType: tc.discriminator}
		if got := u.APIRole(); got != tc.api {
			t.Errorf("APIRole(%q) = %q, want %q", tc.discriminator, got, tc.api)
		}
		disc, ok := DiscriminatorFromAPIRole(tc.api)
		if !ok || disc != tc.discriminator {
			t.Errorf("DiscriminatorFromAPIRole(%q) = %q,%v, want %q,true", tc.api, disc, ok, tc.discriminator)
		}
	}
}

func TestUnknownRole(t *testing.T) {
	if _, ok := DiscriminatorFromAPIRole("superuser"); ok {
		t.Error("expected unknown role to be rejected")
	}
	if got := AuthorityFromDiscriminator("NOPE"); got != "" {
		t.Errorf("Authority(NOPE) = %q, want empty", got)
	}
}
