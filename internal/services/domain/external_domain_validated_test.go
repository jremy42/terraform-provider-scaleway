package domain_test

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	domainSDK "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	"github.com/scaleway/terraform-provider-scaleway/v2/internal/acctest"
	"github.com/scaleway/terraform-provider-scaleway/v2/internal/httperrors"
	"github.com/scaleway/terraform-provider-scaleway/v2/internal/services/domain"
)

const testExternalDomainEnv = "TF_TEST_EXTERNAL_DOMAIN"

func TestAccDomainExternalDomainValidated_Basic(t *testing.T) {
	domainName, subdomain, dnsZone, ok := externalDomainTestParts(t)
	if !ok {
		return
	}

	tt := acctest.NewTestTools(t)
	defer tt.Cleanup()

	log.Printf("Testing external domain validation for domain: %s", domainName)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: tt.ProviderFactories,
		CheckDestroy:             testAccCheckExternalDomainDestroy(tt),
		Steps: []resource.TestStep{
			{
				Config: testAccDomainExternalDomainValidatedConfigBasic(domainName, subdomain, dnsZone),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckExternalDomainExists(tt, "scaleway_domain_external_domain.example"),
					resource.TestCheckResourceAttr("scaleway_domain_external_domain.example", "domain", domainName),
					resource.TestCheckResourceAttrSet("scaleway_domain_external_domain.example", "validation_token"),
					resource.TestCheckResourceAttr("scaleway_domain_external_domain_validated.example", "domain", domainName),
					resource.TestCheckResourceAttrSet("scaleway_domain_external_domain_validated.example", "id"),
					resource.TestCheckResourceAttrSet("scaleway_domain_external_domain_validated.example", "organization_id"),
					resource.TestCheckResourceAttrSet("scaleway_domain_external_domain_validated.example", "ns_servers.#"),
					resource.TestCheckResourceAttr("scaleway_domain_external_domain_validated.example", "validated", "true"),
					resource.TestCheckResourceAttrPair(
						"data.scaleway_domain_external_domain.read", "domain",
						"scaleway_domain_external_domain.example", "domain",
					),
					resource.TestCheckResourceAttrPair(
						"data.scaleway_domain_external_domain.read", "project_id",
						"scaleway_domain_external_domain.example", "project_id",
					),
					resource.TestCheckResourceAttrPair(
						"data.scaleway_domain_external_domain.read", "organization_id",
						"scaleway_domain_external_domain.example", "organization_id",
					),
				),
			},
			{
				ResourceName:      "scaleway_domain_external_domain.example",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "scaleway_domain_external_domain_validated.example",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func externalDomainTestParts(t *testing.T) (domainName, subdomain, dnsZone string, ok bool) {
	t.Helper()

	domainName = os.Getenv(testExternalDomainEnv)
	if domainName == "" {
		t.Skipf("Test skipped: %s must be set to a FQDN whose parent is not registered at Scaleway", testExternalDomainEnv)

		return "", "", "", false
	}

	parts := strings.SplitN(domainName, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Fatalf("%s=%q must be a FQDN with at least two labels", testExternalDomainEnv, domainName)
	}

	return domainName, parts[0], parts[1], true
}

func testAccCheckExternalDomainExists(tt *acctest.TestTools, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("resource not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID is set")
		}

		registrarAPI := domain.NewRegistrarDomainAPI(tt.Meta)

		_, err := registrarAPI.GetDomain(&domainSDK.RegistrarAPIGetDomainRequest{
			Domain: rs.Primary.ID,
		})
		if err != nil {
			return err
		}

		return nil
	}
}

func testAccCheckExternalDomainDestroy(tt *acctest.TestTools) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "scaleway_domain_external_domain" {
				continue
			}

			registrarAPI := domain.NewRegistrarDomainAPI(tt.Meta)

			_, err := registrarAPI.GetDomain(&domainSDK.RegistrarAPIGetDomainRequest{
				Domain: rs.Primary.ID,
			})
			if err != nil {
				if httperrors.Is404(err) {
					continue
				}

				return err
			}

			return fmt.Errorf("external domain %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccDomainExternalDomainValidatedConfigBasic(domainName, subdomain, dnsZone string) string {
	return fmt.Sprintf(`
resource "scaleway_domain_external_domain" "example" {
  domain = "%s"
}

resource "scaleway_domain_record" "validation" {
  dns_zone = "%s"
  name     = "_scaleway-challenge.%s"
  type     = "TXT"
  data     = scaleway_domain_external_domain.example.validation_token
}

resource "scaleway_domain_external_domain_validated" "example" {
  domain     = scaleway_domain_external_domain.example.domain
  depends_on = [scaleway_domain_record.validation]
}

data "scaleway_domain_external_domain" "read" {
  domain = scaleway_domain_external_domain.example.domain
}
`, domainName, dnsZone, subdomain)
}
