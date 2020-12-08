provider "akamai" {
  edgerc = "~/.edgerc"
}

resource "akamai_property" "test" {
  name        = "test property"
  contract_id = "ctr_0"
  group_id    = "grp_0"

  product    = "prd1"
  product_id = "prd2"
}
