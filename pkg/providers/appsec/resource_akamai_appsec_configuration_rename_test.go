package appsec

import (
	"encoding/json"
	"testing"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v2/pkg/appsec"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/stretchr/testify/mock"
)

func TestAccAkamaiConfigurationRename_res_basic(t *testing.T) {
	t.Run("match by Configuration ID", func(t *testing.T) {
		client := &mockappsec{}

		cu := appsec.UpdateConfigurationResponse{}
		json.Unmarshal([]byte(loadFixtureBytes("testdata/TestResConfigurationRename/ConfigurationUpdate.json")), &cu)

		cr := appsec.GetConfigurationResponse{}
		json.Unmarshal([]byte(loadFixtureBytes("testdata/TestResConfigurationRename/Configuration.json")), &cr)

		client.On("GetConfiguration",
			mock.Anything, // ctx is irrelevant for this test
			appsec.GetConfigurationRequest{ConfigID: 432531},
		).Return(&cr, nil)

		client.On("UpdateConfiguration",
			mock.Anything, // ctx is irrelevant for this test
			appsec.UpdateConfigurationRequest{ConfigID: 432531, Name: "Akamai Tools New", Description: "TF Tools"},
		).Return(&cu, nil)

		useClient(client, func() {
			resource.Test(t, resource.TestCase{
				IsUnitTest: true,
				Providers:  testAccProviders,
				Steps: []resource.TestStep{
					{
						Config: loadFixtureString("testdata/TestResConfigurationRename/match_by_id.tf"),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("akamai_appsec_configuration_rename.test", "id", "432531"),
						),
					},
					{
						Config: loadFixtureString("testdata/TestResConfigurationRename/update_by_id.tf"),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("akamai_appsec_configuration_rename.test", "id", "432531"),
						),
					},
				},
			})
		})

		client.AssertExpectations(t)
	})

}
