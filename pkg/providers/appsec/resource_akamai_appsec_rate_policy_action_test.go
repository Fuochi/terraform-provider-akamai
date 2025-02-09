package appsec

import (
	"encoding/json"
	"testing"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v2/pkg/appsec"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/stretchr/testify/mock"
)

func TestAccAkamaiRatePolicyAction_res_basic(t *testing.T) {
	t.Run("match by RatePolicyAction ID", func(t *testing.T) {
		client := &mockappsec{}

		cu := appsec.UpdateRatePolicyActionResponse{}
		json.Unmarshal([]byte(loadFixtureBytes("testdata/TestResRatePolicyAction/RatePolicyUpdateResponse.json")), &cu)

		cr := appsec.GetRatePolicyActionsResponse{}
		json.Unmarshal([]byte(loadFixtureBytes("testdata/TestResRatePolicyAction/RatePolicyActions.json")), &cr)

		client.On("GetRatePolicyActions",
			mock.Anything, // ctx is irrelevant for this test
			appsec.GetRatePolicyActionsRequest{ConfigID: 43253, Version: 7, PolicyID: "AAAA_81230", RatePolicyID: 135355},
		).Return(&cr, nil)

		client.On("UpdateRatePolicyAction",
			mock.Anything, // ctx is irrelevant for this test
			appsec.UpdateRatePolicyActionRequest{ConfigID: 43253, Version: 7, PolicyID: "AAAA_81230", RatePolicyID: 135355, Ipv4Action: "none", Ipv6Action: "none"},
		).Return(&cu, nil)

		useClient(client, func() {
			resource.Test(t, resource.TestCase{
				IsUnitTest: false,
				Providers:  testAccProviders,
				Steps: []resource.TestStep{
					{
						Config: loadFixtureString("testdata/TestResRatePolicyAction/match_by_id.tf"),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("akamai_appsec_rate_policy_action.test", "id", "135355"),
						),
					},

					{
						Config: loadFixtureString("testdata/TestResRatePolicyAction/update_by_id.tf"),
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr("akamai_appsec_rate_policy_action.test", "id", "135355"),
						),
					},
				},
			})
		})

		client.AssertExpectations(t)
	})

}
