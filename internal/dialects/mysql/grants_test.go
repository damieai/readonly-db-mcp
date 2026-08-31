package mysql

import "testing"

func TestValidateGrantsAcceptsSelectOnly(t *testing.T) {
	grants := []string{
		"GRANT USAGE ON *.* TO `reader`@`127.0.0.1` REQUIRE SSL",
		"GRANT SELECT ON `inventory`.* TO `reader`@`127.0.0.1`",
		"GRANT SELECT (`id`, `status`) ON `billing`.`invoices` TO `reader`@`127.0.0.1`",
	}
	if err := ValidateGrants(grants, []string{"inventory", "billing"}); err != nil {
		t.Fatalf("ValidateGrants() error = %v", err)
	}
}

func TestValidateGrantsRejectsExtraPrivileges(t *testing.T) {
	tests := [][]string{
		{"GRANT SELECT, UPDATE ON `inventory`.* TO `reader`@`localhost`"},
		{"GRANT FILE ON *.* TO `reader`@`localhost`"},
		{"GRANT SELECT ON *.* TO `reader`@`localhost`"},
		{"GRANT SELECT ON `other`.* TO `reader`@`localhost`"},
		{"GRANT USAGE ON *.* TO `reader`@`localhost` WITH GRANT OPTION"},
		{"GRANT SELECT ON `inventory`.* TO `reader`@`localhost` WITH GRANT OPTION"},
		{"GRANT `reader_role`@`%` TO `reader`@`localhost`"},
	}
	for _, grants := range tests {
		if err := ValidateGrants(grants, []string{"inventory"}); err == nil {
			t.Fatalf("expected rejection for %#v", grants)
		}
	}
}
